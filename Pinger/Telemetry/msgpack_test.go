package Telemetry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMsgSerialize(t *testing.T) {
	var err error

	assert := assert.New(t)

	msg := NewTelemetryMsg(TelemetryEventInfo, "foo", "bar")
	assert.NotEmpty(msg)
	msg.PrepareForUpload()
	enc, err := msg.Encode()
	assert.NoError(err)
	assert.NotNil(enc)

	msg1 := &TelemetryMsg{}
	err = msg1.Decode(enc)
	assert.NoError(err)
	if err != nil {
		return
	}

	assert.Equal(msg.Id, msg1.Id)
	assert.Equal(msg.EventType, msg1.EventType)
	assert.Equal(msg.Timestamp, msg1.Timestamp)
	assert.Equal(msg.UploadedAt, msg1.UploadedAt)
	assert.Equal(msg.Module, msg1.Module)
	assert.Equal(msg.Message, msg1.Message)
}
