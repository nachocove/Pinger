package Telemetry

import (
	"fmt"
	"github.com/ugorji/go/codec"
)

type TelemetryPackKey int

const (
	TelemetryMsgPackId         TelemetryPackKey = 0
	TelemetryMsgPackEventType  TelemetryPackKey = 1
	TelemetryMsgPackTimestamp  TelemetryPackKey = 2
	TelemetryMsgPackUploadedAt TelemetryPackKey = 3
	TelemetryMsgPackModule     TelemetryPackKey = 4
	TelemetryMsgPackMessage    TelemetryPackKey = 5
)

const (
	TelemetryMsgPackInfo    int64 = 1
	TelemetryMsgPackWarning int64 = 2
	TelemetryMsgPackError   int64 = 3
)

func TelemetryMsgEventTypeToPack(eventType TelemetryEventType) int64 {
	switch {
	case eventType == TelemetryEventInfo:
		return TelemetryMsgPackInfo
	case eventType == TelemetryEventWarning:
		return TelemetryMsgPackWarning
	case eventType == TelemetryEventError:
		return TelemetryMsgPackError
	}
	panic(fmt.Sprintf("TelemetryMsgEventTypeToPack: unknown eventType: %v", eventType))
}

func TelemetryPackEventTypeToMsg(eventType int64) TelemetryEventType {
	switch {
	case eventType == TelemetryMsgPackInfo:
		return TelemetryEventInfo
	case eventType == TelemetryMsgPackWarning:
		return TelemetryEventWarning
	case eventType == TelemetryMsgPackError:
		return TelemetryEventError
	}
	panic(fmt.Sprintf("TelemetryPackEventTypeToMsg: unknown eventType: %v", eventType))
}

type TelemetryMsgPackType map[TelemetryPackKey]interface{}

func (msg *TelemetryMsg) Encode() ([]byte, error) {
	pack := make(TelemetryMsgPackType)
	pack[TelemetryMsgPackId] = msg.Id
	pack[TelemetryMsgPackEventType] = TelemetryMsgEventTypeToPack(msg.EventType)
	pack[TelemetryMsgPackTimestamp] = TelemetryTimefromTime(msg.Timestamp)
	pack[TelemetryMsgPackUploadedAt] = TelemetryTimefromTime(msg.UploadedAt)
	pack[TelemetryMsgPackModule] = msg.Module
	pack[TelemetryMsgPackMessage] = msg.Message

	buffer := make([]byte, 0, 64)
	var h codec.Handle = new(codec.MsgpackHandle)
	enc := codec.NewEncoderBytes(&buffer, h)
	err := enc.Encode(pack)
	if err != nil {
		return nil, err
	}
	return buffer, nil

}

func (msg *TelemetryMsg) Decode(in []byte) error {
	pack := make(TelemetryMsgPackType)
	var h codec.Handle = new(codec.MsgpackHandle)
	dec := codec.NewDecoderBytes(in, h)
	err := dec.Decode(&pack)
	if err != nil {
		return err
	}
	msg.Id = string(pack[TelemetryMsgPackId].([]byte))
	msg.EventType = TelemetryPackEventTypeToMsg(pack[TelemetryMsgPackEventType].(int64))
	msg.Timestamp = TimeFromTelemetryTime(TelemetryMsgPackTime(pack[TelemetryMsgPackTimestamp].(uint64)))
	msg.UploadedAt = TimeFromTelemetryTime(TelemetryMsgPackTime(pack[TelemetryMsgPackUploadedAt].(uint64)))
	msg.Module = string(pack[TelemetryMsgPackModule].([]byte))
	msg.Message = string(pack[TelemetryMsgPackMessage].([]byte))
	return nil
}
