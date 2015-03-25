package Telemetry

import (
	"github.com/twinj/uuid"
	"time"
)

// telemetryEventType The telemetry event type
type telemetryEventType string

const (
	// telemetryEventAll used in DB lookups only. Can not be used as a DB entry
	telemetryEventAll telemetryEventType = ""
	// telemetryEventDebug A Debug entry
	telemetryEventDebug telemetryEventType = "DEBUG"
	// telemetryEventInfo an Info entry
	telemetryEventInfo telemetryEventType = "INFO"
	// telemetryEventWarning a Warn entry
	telemetryEventWarning telemetryEventType = "WARN"
	// telemetryEventError an Error entry
	telemetryEventError telemetryEventType = "ERROR"
)

// String convert the custom type to a string.
func (t telemetryEventType) String() string {
	return string(t)
}

// telemetryMsg a telemetry message entry. Also used to generate the DB table
type telemetryMsg struct {
	Id         string             `db:"id"`
	EventType  telemetryEventType `db:"event_type"`
	Timestamp  time.Time          `db:"timestamp"`
	UploadedAt time.Time          `db:"-"`
	Module     string             `db:"module"`
	Message    string             `db:"message"`
}

func (msg *telemetryMsg) prepareForUpload() error {
	msg.UploadedAt = time.Now().Round(time.Millisecond).UTC()
	return nil
}

type telemetryMsgMap map[string]interface{}

func newId() string {
	uuid.SwitchFormat(uuid.Clean)
	return uuid.NewV4().String()
}
func (msg *telemetryMsg) toMap() telemetryMsgMap {
	msg.prepareForUpload()
	msgMap := make(telemetryMsgMap)
	msgMap["id"] = msg.Id
	msgMap["event_type"] = string(msg.EventType)
	msgMap["timestamp"] = telemetryTimefromTime(msg.Timestamp)
	msgMap["uploaded_at"] = telemetryTimefromTime(msg.UploadedAt)
	msgMap["module"] = msg.Module
	msgMap["message"] = msg.Message
	return msgMap
}

// NewTelemetryMsg Create a new telemetry message instance
func NewTelemetryMsg(eventType telemetryEventType, module, message string) telemetryMsg {
	return telemetryMsg{
		Id:        newId(),
		EventType: eventType,
		Timestamp: time.Now().Round(time.Millisecond).UTC(),
		Module:    module,
		Message:   message,
	}
}
