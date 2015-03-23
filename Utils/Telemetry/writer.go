package Telemetry

import (
	"encoding/json"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/op/go-logging"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
	"compress/gzip"
)

type TelemetryWriter struct {
	fileLocationPrefix      string
	uploadLocationPrefixUrl *url.URL
	dbmap                   *gorp.DbMap
	lastRead                time.Time
	telemetryCh             chan TelemetryMsg
	doUploadNow             chan int
	awsConfig               *AWS.AWSConfiguration
	logger                  *log.Logger
}

func NewTelemetryWriter(config *TelemetryConfiguration, awsConfig *AWS.AWSConfiguration, debug bool) (*TelemetryWriter, error) {
	if config.FileLocationPrefix == "" || config.UploadLocationPrefix == "" {
		return nil, nil // not an error. Just not configured for telemetry
	}
	writer := TelemetryWriter{
		fileLocationPrefix: config.FileLocationPrefix,
		telemetryCh:        make(chan TelemetryMsg, 1024),
		doUploadNow:        make(chan int, 5),
		awsConfig:          awsConfig,
		logger:             log.New(os.Stderr, "telemetryWriter", log.LstdFlags|log.Lshortfile),
	}
	if config.UploadLocationPrefix != "" && awsConfig != nil {
		u, err := url.Parse(config.UploadLocationPrefix)
		if err != nil {
			return nil, err
		}
		writer.uploadLocationPrefixUrl = u
	}
	err := writer.initDb()
	if err != nil {
		return nil, err
	}
	go writer.dbWriter()
	go writer.uploader()
	return &writer, nil
}

func (writer *TelemetryWriter) Log(level logging.Level, calldepth int, rec *logging.Record) error {
	var eventType TelemetryEventType
	switch {
	case level == logging.DEBUG:
		eventType = TelemetryEventDebug

	case level == logging.INFO:
		eventType = TelemetryEventInfo

	case level == logging.ERROR:
		eventType = TelemetryEventError

	case level == logging.WARNING:
		eventType = TelemetryEventWarning

	default:
		eventType = TelemetryEventWarning
	}
	msg := NewTelemetryMsg(eventType, "", rec.Formatted(calldepth+1))
	writer.telemetryCh <- msg
	return nil
}

func (writer *TelemetryWriter) dbWriter() {
	for {
		select {
		case msg := <-writer.telemetryCh:
			err := writer.dbmap.Insert(&msg)
			if err != nil {
				writer.logger.Printf("Could not write to DB: %v\n", err)
			}
		}
	}
}

func (writer *TelemetryWriter) uploader() {
	ticker := time.NewTicker(time.Duration(1) * time.Second)
	longRangerTicker := time.NewTicker(time.Duration(1) * time.Hour)

	for {
		select {
		case <-ticker.C:
			err := writer.createFilesAndUpload()
			if err != nil {
				writer.logger.Printf("Could not createFilesAndUpload: %v\n", err)
			}

		case <-writer.doUploadNow:
			err := writer.createFilesAndUpload()
			if err != nil {
				writer.logger.Printf("Could not createFilesAndUpload: %v\n", err)
			}

		case <-longRangerTicker.C:
			err := writer.upload()
			if err != nil {
				writer.logger.Printf("Could not upload files: %v\n", err)
			}
		}
	}
}

func recoverCrash(logger *log.Logger) {
	if err := recover(); err != nil {
		stack := make([]byte, 8*1024)
		stack = stack[:runtime.Stack(stack, false)]
		logger.Printf("Error: %s\nStack: %s", err, stack)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func (writer *TelemetryWriter) createFilesAndUpload() error {
	err := writer.createFiles()
	if err != nil {
		return err
	}
	err = writer.upload()
	if err != nil {
		return err
	}
	return nil
}

func (writer *TelemetryWriter) createFiles() error {
	defer recoverCrash(writer.logger)
	var rollback = true
	transaction, err := writer.dbmap.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if rollback {
			transaction.Rollback()
		} else {
			transaction.Commit()
			writer.lastRead = time.Now().Round(time.Millisecond).UTC()
		}
	}()

	writeTime := time.Now().Round(time.Millisecond).UTC()
	for _, ttype := range TelemetryEventTypes {
		var messages []TelemetryMsg
		messages, err = writer.getAllMessagesSince(ttype, writer.lastRead)
		if err != nil {
			return err
		}
		var msgArray []TelemetryMsgMap
		if len(messages) > 0 {
			for _, msg := range messages {
				msgArray = append(msgArray, msg.toMap())
			}
			jsonString, err := json.Marshal(msgArray)
			if err != nil {
				return err
			}
			teleFile := fmt.Sprintf("%s/%s-%s.json.gz", writer.fileLocationPrefix, ttype, writeTime.Format(TelemetryTimeZFormat))
			fp, err := os.OpenFile(teleFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
			if err != nil {
				return err
			}
			w := gzip.NewWriter(fp)
			_, err = w.Write(jsonString)
			w.Close()
			fp.Close()
			if err != nil {
				return err
			}

			for _, msg := range messages {
				_, err = writer.dbmap.Delete(&msg)
				if err != nil {
					return err
				}
			}
		}
	}
	rollback = false
	return nil
}

func (writer *TelemetryWriter) upload() error {
	// Read all entries in the telemetry directory, look for any json files, and upload them
	entries, err := ioutil.ReadDir(writer.fileLocationPrefix)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json.gz") || strings.HasSuffix(name, ".json") {
			err := writer.pushToS3(name)
			if err != nil {
				return err
			}
			err = os.Remove(path.Join(writer.fileLocationPrefix, name))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
