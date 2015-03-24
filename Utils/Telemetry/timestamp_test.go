package Telemetry

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	assert := assert.New(t)
	zTime := "2015-02-16T00:00:00Z"
	var teleTime telemetryTime = 635596416000000000
	var tm time.Time
	var err error

	tm, err = timeFromZTime(zTime)
	assert.NoError(err)
	assert.True(tm.Unix() > 0)
	assert.Equal(zTime, tm.Format(telemetryTimeZFormat))

	tele := telemetryTimefromTime(tm)
	assert.Equal(teleTime, tele)

	tm = teleTime.time()
	assert.True(tm.Unix() > 0)
	assert.Equal(zTime, tm.Format(telemetryTimeZFormat))
}
