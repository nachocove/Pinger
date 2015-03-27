package Telemetry

import (
	"fmt"
	"github.com/ugorji/go/codec"
)

type telemetryPackKey int

const (
	telemetryLogMsgPackId         telemetryPackKey = 0
	telemetryLogMsgPackEventType  telemetryPackKey = 1
	telemetryLogMsgPackTimestamp  telemetryPackKey = 2
	telemetryLogMsgPackUploadedAt telemetryPackKey = 3
	telemetryLogMsgPackModule     telemetryPackKey = 4
	telemetryLogMsgPackMessage    telemetryPackKey = 5
)

const (
	telemetryLogMsgPackInfo    int64 = 1
	telemetryLogMsgPackWarning int64 = 2
	telemetryLogMsgPackError   int64 = 3
	telemetryLogMsgPackDebug   int64 = 4
)

func telemetryLogMsgEventTypeToPack(eventType telemetryLogEventType) int64 {
	switch {
	case eventType == telemetryLogEventInfo:
		return telemetryLogMsgPackInfo
	case eventType == telemetryLogEventWarning:
		return telemetryLogMsgPackWarning
	case eventType == telemetryLogEventError:
		return telemetryLogMsgPackError
	case eventType == telemetryLogEventDebug:
		return telemetryLogMsgPackDebug
	}
	panic(fmt.Sprintf("telemetryLogMsgEventTypeToPack: unknown eventType: %v", eventType))
}

func telemetryPackEventTypeToMsg(eventType int64) telemetryLogEventType {
	switch {
	case eventType == telemetryLogMsgPackInfo:
		return telemetryLogEventInfo
	case eventType == telemetryLogMsgPackWarning:
		return telemetryLogEventWarning
	case eventType == telemetryLogMsgPackError:
		return telemetryLogEventError
	case eventType == telemetryLogMsgPackDebug:
		return telemetryLogEventDebug
	}
	panic(fmt.Sprintf("telemetryPackEventTypeToMsg: unknown eventType: %v", eventType))
}

type telemetryLogMsgPackType map[telemetryPackKey]interface{}

func (msg *telemetryLogMsg) encodeMsgPack() ([]byte, error) {
	pack := make(telemetryLogMsgPackType)
	pack[telemetryLogMsgPackId] = msg.Id
	pack[telemetryLogMsgPackEventType] = telemetryLogMsgEventTypeToPack(msg.EventType)
	pack[telemetryLogMsgPackTimestamp] = telemetryTimefromTime(msg.Timestamp)
	pack[telemetryLogMsgPackUploadedAt] = telemetryTimefromTime(msg.UploadedAt)
	pack[telemetryLogMsgPackModule] = msg.Module
	pack[telemetryLogMsgPackMessage] = msg.Message

	buffer := make([]byte, 0, 64)
	var h codec.Handle = new(codec.MsgpackHandle)
	enc := codec.NewEncoderBytes(&buffer, h)
	err := enc.Encode(pack)
	if err != nil {
		return nil, err
	}
	return buffer, nil

}

func (msg *telemetryLogMsg) decodeMsgPack(in []byte) error {
	pack := make(telemetryLogMsgPackType)
	var h codec.Handle = new(codec.MsgpackHandle)
	dec := codec.NewDecoderBytes(in, h)
	err := dec.Decode(&pack)
	if err != nil {
		return err
	}
	msg.Id = string(pack[telemetryLogMsgPackId].([]byte))
	msg.EventType = telemetryPackEventTypeToMsg(pack[telemetryLogMsgPackEventType].(int64))
	msg.Timestamp = telemetryTime(pack[telemetryLogMsgPackTimestamp].(uint64)).time()
	msg.UploadedAt = telemetryTime(pack[telemetryLogMsgPackUploadedAt].(uint64)).time()
	msg.Module = string(pack[telemetryLogMsgPackModule].([]byte))
	msg.Message = string(pack[telemetryLogMsgPackMessage].([]byte))
	return nil
}
