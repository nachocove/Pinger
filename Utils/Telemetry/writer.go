// Telemetry implements telemetry for Pinger
package Telemetry

import (
	"bytes"
	"compress/gzip"
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
	"sync"
	"time"
)

// TelemetryWriter The telemetry writer functionality. Comprises a few goroutines for writing to the DB,
// extracting into files, pushing to telemetry (s3) etc.
//
// The TelemetryWriter 'listens' on one side to log messages (see .Log() method) and reformats the log
// message into a telemetry record, and puts that reformatted message into a channel (to not delay logging
// any more than necessary).
//
// A separate goroutine listens on the channel and writes the record to a local sqlite3 db
// (/tmp/telemetry/telemetry.db), which is essentially a buffer. Yea another go-routine periodically wakes
// up, reads all records from the sqlite3 DB, formats them into json-formatted files, gzip's the files,
// and uploads it to s3.
type TelemetryWriter struct {
	fileLocationPrefix      string               // where to store tempfiles which are written to s3
	uploadLocationPrefixUrl *url.URL             // prefix in the s3 bucket where to store the file
	uploadInterval          int64                // how often (in minutes) to upload to s3
	dbmap                   *gorp.DbMap          // the Gorp handle
	lastRead                time.Time            // timestamp when we last read from the DB.
	telemetryCh             chan telemetryLogMsg // the channel the .Log() method writes to
	doUploadNow             chan int             // a channel to make the background 'task' write immediately (used when an error is encountered, for example).
	aws                     AWS.AWSHandler       // the AWS methods
	logger                  *log.Logger          // the logger
	includeDebug            bool                 // whether to also upload debugs. Used for debugging.
	debug                   bool                 // whether to debug the telemetry writer (to debug we use printfs, so there's no logging loop)
	msgCount                int64                // the message count. Keeps track of how many messages we have buffered. We upload every 100.
	mutex                   sync.Mutex
}

var NL, CR []byte

func init() {
	NL = []byte("\\n")
	CR = []byte("\\r")
}

// NewTelemetryWriter create a new TelemetryWriter instance
func NewTelemetryWriter(config *TelemetryConfiguration, aws AWS.AWSHandler, debug bool) (*TelemetryWriter, error) {
	if config.FileLocationPrefix == "" {
		return nil, nil // not an error. Just not configured for telemetry
	}
	writer := TelemetryWriter{
		fileLocationPrefix: config.FileLocationPrefix,
		telemetryCh:        make(chan telemetryLogMsg, 1024),
		doUploadNow:        make(chan int, 5),
		aws:                aws,
		logger:             log.New(os.Stderr, "telemetryWriter", log.LstdFlags|log.Lshortfile),
		includeDebug:       config.IncludeDebug,
		debug:              debug,
		mutex:              sync.Mutex{},
		uploadInterval:     config.UploadInterval,
	}
	err := writer.makeFileLocationPrefix()
	if err != nil {
		return nil, err
	}
	if config.UploadLocationPrefix != "" && aws != nil {
		u, err := url.Parse(config.UploadLocationPrefix)
		if err != nil {
			return nil, err
		}
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

// Log Implements the logging Interface so this can be used as a logger backend.
func (writer *TelemetryWriter) Log(level logging.Level, calldepth int, rec *logging.Record) error {
	var eventType telemetryLogEventType
	switch {
	case level == logging.DEBUG:
		eventType = telemetryLogEventDebug

	case level == logging.INFO:
		eventType = telemetryLogEventInfo

	case level == logging.ERROR:
		eventType = telemetryLogEventError

	case level == logging.WARNING:
		eventType = telemetryLogEventWarning

	default:
		eventType = telemetryLogEventWarning
	}
	if writer.includeDebug || eventType == telemetryLogEventWarning || eventType == telemetryLogEventError || eventType == telemetryLogEventInfo {
		msg := NewTelemetryMsg(eventType, rec.Module, rec.Message(), rec.Time.Round(time.Millisecond).UTC())
		writer.telemetryCh <- msg
	}
	return nil
}

// dbWriter the Goroutine responsible for reading from the channel and writing to the DB.
// this avoids any contention to the DB. This is the only place we write to the DB.
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

// uploader the goroutine responsible for pulling data out
// of the DB and into file and from files into S3/telemetry/
// It uses:
//  - a timer to push up files every 10 minutes (reset on 'do it now')
//  - a 'do it now' channel, which external entities can trigger to 'do it now'
func (writer *TelemetryWriter) uploader() {
	// dump any DB entries and upload any files left on the file system
	err := writer.createFilesAndUpload()
	if err != nil {
		writer.logger.Printf("Could not upload files: %v\n", err)
	}

	writeTimeout := time.Duration(writer.uploadInterval) * time.Minute
	writeTimer := time.NewTimer(writeTimeout)
	for {
		select {
		case <-writeTimer.C:
			err := writer.createFilesAndUpload()
			if err != nil {
				writer.logger.Printf("Could not createFilesAndUpload: %v\n", err)
			}
			writeTimer.Reset(writeTimeout)

		case <-writer.doUploadNow:
			err := writer.createFilesAndUpload()
			if err != nil {
				writer.logger.Printf("Could not createFilesAndUpload: %v\n", err)
			}
			writeTimer.Reset(writeTimeout)
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

func (writer *TelemetryWriter) createFilesFromMessages(messages *[]telemetryLogMsg) error {
	var buffer bytes.Buffer
	if len(*messages) > 0 {
		var prevTime time.Time
		for _, msg := range *messages {
			if prevTime.IsZero() {
				prevTime = msg.Timestamp
			} else if prevTime.Day() != msg.Timestamp.Day() {
				writer.logger.Printf("Date changed. Writing out collected messages at : %s", prevTime)
				err := writer.writeOutFile(buffer, prevTime)
				if err != nil {
					return err
				}
				buffer.Reset()
			}
			prevTime = msg.Timestamp
			jsonString, err := json.Marshal(msg.toMap())
			jsonString = bytes.Replace(jsonString, CR, []byte("<CR>"), -1)
			jsonString = bytes.Replace(jsonString, NL, []byte("<NL>"), -1)
			if err != nil {
				return err
			}
			buffer.Write(jsonString)
			buffer.WriteString("\n")
		}
		err := writer.writeOutFile(buffer, prevTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (writer *TelemetryWriter) writeOutFile(fileString bytes.Buffer, endTime time.Time) error {
	var teleFile string
	dateString := strings.Replace(endTime.Format("20060102150405.999"), ".", "", 1)
	teleFile = fmt.Sprintf("%s/plog-%s.gz",
		writer.fileLocationPrefix,
		dateString)
	if writer.debug {
		writer.logger.Printf("Creating file: %s", teleFile)
	}
	fp, err := os.OpenFile(teleFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
	defer fp.Close()
	if err != nil {
		return err
	}
	w := gzip.NewWriter(fp)
	defer w.Close()
	_, err = fileString.WriteTo(w)
	if err != nil {
		return err
	}
	return nil
}

// createFiles responsible for pulling data out of the DB and writing it to files.
// It does not write to the DB and does not talk to s3
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

	var messages []telemetryLogMsg
	messages, err = writer.getAllMessagesSince(writer.lastRead, telemetryLogEventAll)
	if err != nil {
		return err
	}
	err = writer.createFilesFromMessages(&messages)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		_, err = writer.dbmap.Delete(&msg)
		if err != nil {
			return err
		}
	}
	rollback = false
	return nil
}

// upload upload any files found to s3
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
			if strings.HasPrefix(name, "plog-") {
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
