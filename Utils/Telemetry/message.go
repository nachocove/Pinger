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
	TelemetryEventAll     TelemetryEventType = ""  // used in DB lookups
	TelemetryEventDebug   TelemetryEventType = "DEBUG"
	TelemetryEventInfo    TelemetryEventType = "INFO"
	TelemetryEventWarning TelemetryEventType = "WARNING"
	TelemetryEventError   TelemetryEventType = "ERROR"
)

var TelemetryEventTypes []TelemetryEventType

func (t TelemetryEventType) String() string {
	return string(t)
}

func init() {
	TelemetryEventTypes = []TelemetryEventType{TelemetryEventDebug, TelemetryEventInfo, TelemetryEventWarning, TelemetryEventError}
}

type TelemetryMsg struct {
	Id         string             `db:"id"`
	EventType  TelemetryEventType `db:"event_type"`
	Timestamp  time.Time          `db:"timestamp"`
	UploadedAt time.Time          `db:"-"`
	Module     string             `db:"module"`
	Message    string             `db:"message"`
}

func (msg *TelemetryMsg) prepareForUpload() error {
	msg.UploadedAt = time.Now().Round(time.Millisecond).UTC()
	return nil
}

type TelemetryMsgMap map[string]interface{}

func (msg *TelemetryMsg) toMap() TelemetryMsgMap {
	msg.prepareForUpload()
	msgMap := make(TelemetryMsgMap)
	msgMap["id"] = msg.Id
	msgMap["event_type"] = string(msg.EventType)
	msgMap["timestamp"] = TelemetryTimefromTime(msg.Timestamp)
	msgMap["uploaded_at"] = TelemetryTimefromTime(msg.UploadedAt)
	msgMap["module"] = msg.Module
	msgMap["message"] = msg.Message
	return msgMap
}

func NewTelemetryMsg(eventType TelemetryEventType, module, message string) TelemetryMsg {
	uuid.SwitchFormat(uuid.Clean)
	return TelemetryMsg{
		Id:        uuid.NewV4().String(),
		EventType: eventType,
		Timestamp: time.Now().Round(time.Millisecond).UTC(),
		Module:    module,
		Message:   message,
	}
}
