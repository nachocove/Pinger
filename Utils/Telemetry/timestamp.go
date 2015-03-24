package Telemetry

import (
	"time"
)

type telemetryTime uint64

const (
	telemetryTimeZFormat string = "2006-01-02T15:04:05.999Z"

	// TelemetryTimeUnixZeroTicks equivalent to ticks since start
	// of the gregorian calendar up to 1970-01-01 UTC Midnight
	telemetryTimeUnixZeroTicks telemetryTime = 621355968000000000

	// How many ticks in a millisecond
	ticksPerMillisecond uint64  = 10000
	ticksPerNanosecond  float64 = 0.01
)

func timeFromZTime(s string) (time.Time, error) {
	return time.Parse(telemetryTimeZFormat, s)
}

func telemetryTimefromTime(t time.Time) telemetryTime {
	msecs := uint64(t.UnixNano() / int64(time.Millisecond))
	ticks := telemetryTime(msecs * ticksPerMillisecond)
	ttime := ticks + telemetryTimeUnixZeroTicks
	return ttime
}

func (t telemetryTime) time() time.Time {
	// unixtime in msecs
	nano := float64(t-telemetryTimeUnixZeroTicks) / ticksPerNanosecond
	return time.Unix(0, int64(nano)).Round(time.Millisecond).UTC()
}
