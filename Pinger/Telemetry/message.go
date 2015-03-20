package Telemetry

import (
	"time"
)

type TelemetryMessages interface {
	Encode() ([]byte, error)
	Decode([]byte) error
	PrepareForUpload() error
	Upload(location string) error
}
type TelemetryEventType string

const (
	TelemetryEventInfo    TelemetryEventType = "INFO"
	TelemetryEventWarning TelemetryEventType = "WARNING"
	TelemetryEventError   TelemetryEventType = "ERROR"
)

type TelemetryMsg struct {
	Id         string
	EventType  TelemetryEventType
	Timestamp  time.Time
	UploadedAt time.Time
	Module     string
	Message    string
}

func (msg *TelemetryMsg) PrepareForUpload() error {
	msg.Id = "12345" // TODO who fills this in? DynamoDB? Or client?
	msg.UploadedAt = time.Now().Round(time.Millisecond).UTC()
	return nil
}

func (msg *TelemetryMsg) Upload(location string) error {
	err := msg.PrepareForUpload()
	if err != nil {
		return err
	}
	// upload here
	return nil
}

func NewTelemetryMsg(eventType TelemetryEventType, module, message string) TelemetryMsg {
	return TelemetryMsg{
		EventType: eventType,
		Timestamp: time.Now().Round(time.Millisecond).UTC(),
		Module:    module,
		Message:   message,
	}
}
