package Telemetry

import (
	"fmt"
	"github.com/ugorji/go/codec"
)

type telemetryPackKey int

const (
	telemetryMsgPackId         telemetryPackKey = 0
	telemetryMsgPackEventType  telemetryPackKey = 1
	telemetryMsgPackTimestamp  telemetryPackKey = 2
	telemetryMsgPackUploadedAt telemetryPackKey = 3
	telemetryMsgPackModule     telemetryPackKey = 4
	telemetryMsgPackMessage    telemetryPackKey = 5
)

const (
	telemetryMsgPackInfo    int64 = 1
	telemetryMsgPackWarning int64 = 2
	telemetryMsgPackError   int64 = 3
	telemetryMsgPackDebug   int64 = 4
)

func telemetryMsgEventTypeToPack(eventType telemetryEventType) int64 {
	switch {
	case eventType == telemetryEventInfo:
		return telemetryMsgPackInfo
	case eventType == telemetryEventWarning:
		return telemetryMsgPackWarning
	case eventType == telemetryEventError:
		return telemetryMsgPackError
	case eventType == telemetryEventDebug:
		return telemetryMsgPackDebug
	}
	panic(fmt.Sprintf("telemetryMsgEventTypeToPack: unknown eventType: %v", eventType))
}

func telemetryPackEventTypeToMsg(eventType int64) telemetryEventType {
	switch {
	case eventType == telemetryMsgPackInfo:
		return telemetryEventInfo
	case eventType == telemetryMsgPackWarning:
		return telemetryEventWarning
	case eventType == telemetryMsgPackError:
		return telemetryEventError
	case eventType == telemetryMsgPackDebug:
		return telemetryEventDebug
	}
	panic(fmt.Sprintf("telemetryPackEventTypeToMsg: unknown eventType: %v", eventType))
}

type telemetryMsgPackType map[telemetryPackKey]interface{}

func (msg *telemetryMsg) encodeMsgPack() ([]byte, error) {
	pack := make(telemetryMsgPackType)
	pack[telemetryMsgPackId] = msg.Id
	pack[telemetryMsgPackEventType] = telemetryMsgEventTypeToPack(msg.EventType)
	pack[telemetryMsgPackTimestamp] = telemetryTimefromTime(msg.Timestamp)
	pack[telemetryMsgPackUploadedAt] = telemetryTimefromTime(msg.UploadedAt)
	pack[telemetryMsgPackModule] = msg.Module
	pack[telemetryMsgPackMessage] = msg.Message

	buffer := make([]byte, 0, 64)
	var h codec.Handle = new(codec.MsgpackHandle)
	enc := codec.NewEncoderBytes(&buffer, h)
	err := enc.Encode(pack)
	if err != nil {
		return nil, err
	}
	return buffer, nil

}

func (msg *telemetryMsg) decodeMsgPack(in []byte) error {
	pack := make(telemetryMsgPackType)
	var h codec.Handle = new(codec.MsgpackHandle)
	dec := codec.NewDecoderBytes(in, h)
	err := dec.Decode(&pack)
	if err != nil {
		return err
	}
	msg.Id = string(pack[telemetryMsgPackId].([]byte))
	msg.EventType = telemetryPackEventTypeToMsg(pack[telemetryMsgPackEventType].(int64))
	msg.Timestamp = telemetryTime(pack[telemetryMsgPackTimestamp].(uint64)).time()
	msg.UploadedAt = telemetryTime(pack[telemetryMsgPackUploadedAt].(uint64)).time()
	msg.Module = string(pack[telemetryMsgPackModule].([]byte))
	msg.Message = string(pack[telemetryMsgPackMessage].([]byte))
	return nil
}
