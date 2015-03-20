package Telemetry

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"fmt"
)

func TestMsgSerialize(t *testing.T) {
	var err error

	assert := assert.New(t)
	
	msg := NewTelemetryMsg(TelemetryEventInfo, "foo", "bar")
	assert.NotEmpty(msg)
	msg.PrepareForUpload()
	fmt.Printf("%+v\n", msg)
	enc, err := msg.Encode()
	assert.NoError(err)
	assert.NotNil(enc)
		
	msg1 := &TelemetryMsg{}
	err = msg1.Decode(enc)
	assert.NoError(err)
	if err != nil {
		return
	}
	fmt.Printf("%+v\n", msg1)
	
	assert.Equal(msg.Id, msg1.Id)	
	assert.Equal(msg.EventType, msg1.EventType)	
	assert.Equal(TelemetryTimefromTime(msg.Timestamp), TelemetryTimefromTime(msg1.Timestamp))	
	assert.Equal(TelemetryTimefromTime(msg.UploadedAt), TelemetryTimefromTime(msg1.UploadedAt))	
	assert.Equal(msg.Module, msg1.Module)	
	assert.Equal(msg.Message, msg1.Message)	
}

