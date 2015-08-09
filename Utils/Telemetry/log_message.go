package Telemetry

import (
	"github.com/nachocove/Pinger/Utils/HostId"
	"github.com/twinj/uuid"
	"strings"
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
	Id         string                `db:"id"`
	EventType  telemetryLogEventType `db:"event_type"`
	Timestamp  time.Time             `db:"timestamp"`
	UploadedAt time.Time             `db:"-"`
	Module     string                `db:"module"`
	Message    string                `db:"message"`
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
	tokens := strings.Split(msg.Message, "|")
	var rawMessage string
	for _, token := range tokens {
		pair := strings.SplitN(token, "=", 2)
		if len(pair) == 1 {
			if rawMessage == "" {
				rawMessage = pair[0]
			} else {
				rawMessage += "," + pair[0]
			}
		} else{
			msgMap[pair[0]] = pair[1]
		}
	}
	if rawMessage != "" {
		if _, ok := msgMap["message"]; ok {
			msgMap["message"] = msgMap["message"].(string) + "," + rawMessage
		} else {
			msgMap["message"] = rawMessage
		}
	}
	msgMap["id"] = msg.Id
	msgMap["event_type"] = string(msg.EventType)
	msgMap["timestamp"] = msg.Timestamp.Format("2006-01-02 15:04:05.999")
	msgMap["uploaded_at"] = msg.UploadedAt.Format("2006-01-02 15:04:05.999")
	msgMap["module"] = msg.Module
	msgMap["pinger"] = HostId.HostId()
	return msgMap
}

// NewTelemetryMsg Create a new telemetry message instance
func NewTelemetryMsg(eventType telemetryLogEventType, module, message string, timestamp time.Time) telemetryLogMsg {
	return telemetryLogMsg{
		Id:        newId(),
		EventType: eventType,
		Timestamp: timestamp,
		Module:    module,
		Message:   message,
	}
}

// NewTelemetryMsg2 Create a new telemetry message instance
func NewTelemetryMsg2(eventType telemetryLogEventType, module, client, device, session, context, message, hostId string, timestamp time.Time) telemetryLogMsg {
	return telemetryLogMsg{
		Id:        newId(),
		EventType: eventType,
		Timestamp: timestamp,
		Module:    module,
		//		Client:        client,
		//		DeviceId:      device,
		//		SessionId:     session,
		//		ClientContext: context,
		Message: message,
		//		Pinger:        hostId,
	}
}
