package Telemetry

import (
	"os"
	"fmt"
	"github.com/op/go-logging"
	"time"
)

type TelemetryWriterMap map[TelemetryEventType]*os.File

type TelemetryWriter struct {
	telemetryCh chan TelemetryMsg
	writerMap TelemetryWriterMap
	Logger *logging.Logger
}

func NewTelemetryWriter(config *TelemetryConfiguration, debug bool) (*TelemetryWriter, error) {
	if config.FileLocationPrefix == "" || config.UploadLocationPrefix == "" {
		return nil, nil  // not an error. Just not configured for telemetry
	}
	
	writer := TelemetryWriter{
		telemetryCh:  make(chan TelemetryMsg, 1024),
		writerMap: make(TelemetryWriterMap),
		Logger: logging.MustGetLogger("CriticalTelemetryErrors"),
	}
	var f *os.File
	var err error
	err = os.MkdirAll(config.FileLocationPrefix, 0700)
	if err != nil {
		return nil, err
	}
	
	for _, ttype := range TelemetryEventTypes {
		teleFile := fmt.Sprintf("%s/%s.tele", config.FileLocationPrefix, ttype)
		f, err = os.OpenFile(teleFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		writer.writerMap[ttype] = f
	}
	go writer.run()
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
	err := writer.Write(msg)
	return err
}

func (writer *TelemetryWriter) FlushWriters() {
	for _, f := range writer.writerMap {
		f.Sync()
	}
}

func (writer *TelemetryWriter) run() {
	defer writer.FlushWriters()
	
	ticker := time.NewTicker(time.Duration(1)*time.Second)
	for {
		select {
		case msg := <- writer.telemetryCh:
			enc, err := msg.EncodeMsgPack()
			if err != nil {
				writer.Logger.Critical("Could not encode message: %+v", msg)
				break
			}
			_, err = writer.writerMap[msg.EventType].Write(enc)
			if err != nil {
				writer.Logger.Critical("Could not write message: %+v", msg)
				break
			}
			
		case <- ticker.C:
			writer.FlushWriters()
		}
	}
}

func (writer *TelemetryWriter) Write(msg TelemetryMsg) error {
	writer.telemetryCh <- msg
	return nil
}
