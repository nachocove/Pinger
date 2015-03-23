package Telemetry

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/HostId"
	"github.com/op/go-logging"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"
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
	hostId                  string
	includeDebug            bool
	debug                   bool
	msgCount                int64
	mutex                   sync.Mutex
}

func NewTelemetryWriter(config *TelemetryConfiguration, awsConfig *AWS.AWSConfiguration, debug bool) (*TelemetryWriter, error) {
	if config.FileLocationPrefix == "" {
		return nil, nil // not an error. Just not configured for telemetry
	}
	writer := TelemetryWriter{
		fileLocationPrefix: config.FileLocationPrefix,
		telemetryCh:        make(chan TelemetryMsg, 1024),
		doUploadNow:        make(chan int, 5),
		awsConfig:          awsConfig,
		logger:             log.New(os.Stderr, "telemetryWriter", log.LstdFlags|log.Lshortfile),
		hostId:             HostId.HostId(),
		includeDebug:       config.IncludeDebug,
		debug:              debug,
		mutex:              sync.Mutex{},
	}
	err := writer.makeFileLocationPrefix()
	if err != nil {
		return nil, err
	}
	if config.UploadLocationPrefix != "" && awsConfig != nil {
		u, err := url.Parse(config.UploadLocationPrefix)
		if err != nil {
			return nil, err
		}
		u.Path = path.Join(u.Path, writer.hostId)
		writer.uploadLocationPrefixUrl = u
	}
	err = writer.initDb()
	if err != nil {
		return nil, err
	}
	go writer.dbWriter()
	go writer.uploader()
	return &writer, nil
}

func NewTelemetryMsgFromRecord(eventType TelemetryEventType, rec *logging.Record) TelemetryMsg {
	return TelemetryMsg{
		Id:        NewId(),
		EventType: eventType,
		Timestamp: rec.Time.Round(time.Millisecond).UTC(),
		Module:    rec.Module,
		Message:   rec.Message(),
	}
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
	if writer.includeDebug || eventType == TelemetryEventWarning || eventType == TelemetryEventError || eventType == TelemetryEventInfo {
		if writer.debug {
			writer.logger.Printf("Logger message: %s:%s", rec.Level, rec.Message())
		}
		msg := NewTelemetryMsgFromRecord(eventType, rec)
		writer.telemetryCh <- msg
	} else {
		if writer.debug {
			writer.logger.Printf("IGNORED Logger message: %s:%s", rec.Level, rec.Message())
		}
	}
	return nil
}

func (writer *TelemetryWriter) dbWriter() {
	for {
		msg := <-writer.telemetryCh
		err := writer.dbmap.Insert(&msg)
		if err != nil {
			writer.logger.Printf("Could not write to DB: %v\n", err)
		}
		writer.mutex.Lock()
		writer.msgCount++
		if writer.msgCount > 100 {
			writer.doUploadNow <- 1
			writer.msgCount = 0
		}
		writer.mutex.Unlock()
	}
}

func (writer *TelemetryWriter) uploader() {
	// dump any DB entries and upload any files left on the file system
	err := writer.createFilesAndUpload()
	if err != nil {
		writer.logger.Printf("Could not upload files: %v\n", err)
	}

	ticker := time.NewTicker(time.Duration(10) * time.Minute)
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

func (writer *TelemetryWriter) makeFileLocationPrefix() error {
	if !exists(writer.fileLocationPrefix) {
		err := os.MkdirAll(writer.fileLocationPrefix, 0700)
		if err != nil {
			return err
		}
	}
	return nil
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
			if writer.debug {
				writer.logger.Printf("Rollback")
			}
			transaction.Rollback()
		} else {
			if writer.debug {
				writer.logger.Printf("Commit")
			}
			transaction.Commit()
			writer.lastRead = time.Now().Round(time.Millisecond).UTC()
		}
	}()

	var messages []TelemetryMsg
	messages, err = writer.getAllMessagesSince(writer.lastRead, TelemetryEventAll)
	if err != nil {
		return err
	}
	var msgArray []TelemetryMsgMap
	if len(messages) > 0 {
		var startTime, endTime time.Time
		for _, msg := range messages {
			switch {
			case startTime.IsZero() || msg.Timestamp.Before(startTime):
				startTime = msg.Timestamp

			case endTime.IsZero() || msg.Timestamp.After(endTime):
				endTime = msg.Timestamp
			}
			msgArray = append(msgArray, msg.toMap())
		}
		jsonString, err := json.Marshal(msgArray)
		if err != nil {
			return err
		}
		teleFile := fmt.Sprintf("%s/%s--%s.json.gz", writer.fileLocationPrefix, startTime.Format(TelemetryTimeZFormat), endTime.Format(TelemetryTimeZFormat))
		if writer.debug {
			writer.logger.Printf("Creating file: %s", teleFile)
		}
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
	rollback = false
	return nil
}

func (writer *TelemetryWriter) upload() error {
	if writer.uploadLocationPrefixUrl != nil {
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
	}
	return nil
}
