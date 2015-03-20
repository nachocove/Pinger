package Telemetry

import (
	"math"
	"time"
)

type TelemetryMsgPackTime uint64

const (
	TelemetryTimeZFormat string = "2006-01-02T15:04:05.999Z"

	// TelemetryTimeUnixZeroTicks equivalent to ticks since start
	// of the gregorian calendar up to 1970-01-01 UTC Midnight
	TelemetryTimeUnixZeroTicks TelemetryMsgPackTime = 621355968000000000

	// How many ticks in a millisecond
	TicksPerMillisecond uint64 = 10000
	TicksPerNanosecond float64 = 0.01
)

var TelemetryZeroTime time.Time

func init() {
	var err error
	TelemetryZeroTime = time.Time{}
	if err != nil {
		panic(err.Error())
	}
}

func TimeFromZTime(s string) (time.Time, error) {
	return time.Parse(TelemetryTimeZFormat, s)
}

func TelemetryTimeUtcNow() TelemetryMsgPackTime {
	return TelemetryTimefromTime(time.Now().UTC())
}

func TelemetryTimefromTime(t time.Time) TelemetryMsgPackTime {
	msecs := uint64(t.UnixNano() / int64(time.Millisecond))
	ticks := TelemetryMsgPackTime(msecs * TicksPerMillisecond)
	ttime := ticks + TelemetryTimeUnixZeroTicks
	return ttime
}

func TimeFromTelemetryTime(t TelemetryMsgPackTime) time.Time {
	// unixtime in msecs
	nano := float64(t-TelemetryTimeUnixZeroTicks) / TicksPerNanosecond
	secs := nano / float64(time.Second)
	rem := math.Remainder(nano, float64(time.Second))
	return time.Unix(int64(secs), int64(rem)).Round(time.Millisecond).UTC()
}
