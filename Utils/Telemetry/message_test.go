package Telemetry

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMsgCreate(t *testing.T) {
	var err error

	assert := assert.New(t)

	msg := NewTelemetryMsg(telemetryEventInfo, "foo", "bar")
	assert.NotEmpty(msg)
	assert.NotEmpty(msg.Id)
	assert.Equal(time.Time{}, msg.UploadedAt)
	err = msg.prepareForUpload()
	assert.NoError(err)
	assert.NotEqual(time.Time{}, msg.UploadedAt)
}
