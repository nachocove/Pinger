package Telemetry

import (
	"github.com/twinj/uuid"
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
	TelemetryEventDebug   TelemetryEventType = "DEBUG"
	TelemetryEventInfo    TelemetryEventType = "INFO"
	TelemetryEventWarning TelemetryEventType = "WARNING"
	TelemetryEventError   TelemetryEventType = "ERROR"
)

var TelemetryEventTypes []TelemetryEventType

func init() {
	TelemetryEventTypes = []TelemetryEventType{ TelemetryEventDebug, TelemetryEventInfo, TelemetryEventWarning, TelemetryEventError }
}

type TelemetryMsg struct {
	Id         string
	EventType  TelemetryEventType
	Timestamp  time.Time
	UploadedAt time.Time
	Module     string
	Message    string
}

func (msg *TelemetryMsg) prepareForUpload() error {
	uuid.SwitchFormat(uuid.Clean)
	msg.Id = uuid.NewV4().String()
	msg.UploadedAt = time.Now().Round(time.Millisecond).UTC()
	return nil
}

func (msg *TelemetryMsg) Upload(location string) error {
	err := msg.prepareForUpload()
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
