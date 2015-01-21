package Pinger

import (
	"time"

	"github.com/op/go-logging"
)

type StatLogger struct {
	ResponseTimeCh chan float64
	FirstTimeResponseTimeCh chan float64
	OverageSleepTimeCh chan float64
	tallyLogger *logging.Logger	
}

func NewStatLogger(logger *logging.Logger) *StatLogger {
	stat := &StatLogger{
		ResponseTimeCh: make(chan float64, 1000),
		FirstTimeResponseTimeCh: make(chan float64, 1000),
		OverageSleepTimeCh: make(chan float64, 1000),
		tallyLogger: logger,
	}
	go stat.tallyResponseTimes()
	return stat
}

func (stat *StatLogger) tallyResponseTimes() {
	var data float64
	normalResponseTimes := newStats()
	firstResponseTimes := newStats()
	sleepTimeStats := newStats()
	logTimeout := time.Duration(5 * time.Second)
	logTimer := time.NewTimer(logTimeout)
	for {
		select {
		case data = <-stat.ResponseTimeCh:
			normalResponseTimes.addDataPoint(data)

		case data = <-stat.FirstTimeResponseTimeCh:
			firstResponseTimes.addDataPoint(data)

		case data = <-stat.OverageSleepTimeCh:
			sleepTimeStats.addDataPoint(data)

		case <-logTimer.C:
			firstResponseTimes.log(stat.tallyLogger, "    first")
			normalResponseTimes.log(stat.tallyLogger, "   normal")
			sleepTimeStats.log(stat.tallyLogger, "sleepOver")
			logTimer.Reset(logTimeout)
		}
	}
}

type statStruct struct {
	min   float64
	max   float64
	sum   float64
	avg   float64
	count int
}

func newStats() *statStruct {
	return &statStruct{
		min:   1000000.00,
		max:   0,
		avg:   0,
		count: 0,
		sum:   0,
	}
}

func (r *statStruct) addDataPoint(responseTime float64) {
	if responseTime < r.min {
		r.min = responseTime
	}
	if responseTime > r.max {
		r.max = responseTime
	}
	r.count++
	r.sum = r.sum + responseTime
}

func (r *statStruct) log(logger *logging.Logger, prefix string) {
	if r.count > 0 {
		r.avg = r.sum / float64(r.count)
		logger.Info("%s(min/avg/max): %8.2fms / %8.2fms / %8.2fms (connections: %7d,  messages: %7d)\n", prefix, r.min*1000.00, r.avg*1000.00, r.max*1000.00, ActiveClientCount, r.count)
	}
}
