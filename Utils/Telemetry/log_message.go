package Telemetry

import (
	"github.com/twinj/uuid"
	"time"
)

// telemetryLogEventType The telemetry event type
type telemetryLogEventType string

const (
	telemetryLogEventAll     telemetryLogEventType = "" // telemetryLogEventAll used in DB lookups only. Can not be used as a DB entry
	telemetryLogEventDebug   telemetryLogEventType = "DEBUG"
	telemetryLogEventInfo    telemetryLogEventType = "INFO"
	telemetryLogEventWarning telemetryLogEventType = "WARN"
	telemetryLogEventError   telemetryLogEventType = "ERROR"
)

// String convert the custom type to a string.
func (t telemetryLogEventType) String() string {
	return string(t)
}

// telemetryLogMsg a telemetry message entry. Also used to generate the DB table
type telemetryLogMsg struct {
	Id            string                `db:"id"`
	EventType     telemetryLogEventType `db:"event_type"`
	Timestamp     time.Time             `db:"timestamp"`
	UploadedAt    time.Time             `db:"-"`
	Client        string                `db:"client"`
	DeviceId      string                `db:"device"`
	SessionId     string                `db:"session"`
	ClientContext string                `db:"context"`
	Module        string                `db:"module"`
	Message       string                `db:"message"`
	Pinger        string                `db:"pinger"`
}

func (msg *telemetryLogMsg) prepareForUpload() error {
	msg.UploadedAt = time.Now().Round(time.Millisecond).UTC()
	return nil
}

type telemetryLogMsgMap map[string]interface{}

func newId() string {
	uuid.SwitchFormat(uuid.Clean)
	return uuid.NewV4().String()
}
func (msg *telemetryLogMsg) toMap() telemetryLogMsgMap {
	msg.prepareForUpload()
	msgMap := make(telemetryLogMsgMap)
	msgMap["id"] = msg.Id
	msgMap["event_type"] = string(msg.EventType)
	msgMap["timestamp"] = msg.Timestamp.Format("2006-01-02 15:04:05.999")
	msgMap["uploaded_at"] = msg.UploadedAt.Format("2006-01-02 15:04:05.999")
	msgMap["client"] = msg.Client
	msgMap["device"] = msg.DeviceId
	msgMap["session"] = msg.SessionId
	msgMap["context"] = msg.ClientContext
	msgMap["module"] = msg.Module
	msgMap["message"] = msg.Message
	msgMap["pinger"] = msg.Pinger
	return msgMap
}

// NewTelemetryMsg Create a new telemetry message instance
func NewTelemetryMsg(eventType telemetryLogEventType, module, client, device, session, context, message, hostId string, timestamp time.Time) telemetryLogMsg {
	return telemetryLogMsg{
		Id:            newId(),
		EventType:     eventType,
		Timestamp:     timestamp,
		Module:        module,
		Client:        client,
		DeviceId:      device,
		SessionId:     session,
		ClientContext: context,
		Message:       message,
		Pinger:        hostId,
	}
}
