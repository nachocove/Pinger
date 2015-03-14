package Utils

import (
	"time"

	logging "github.com/nachocove/Pinger/Pinger/logging"
)

type StatLogger struct {
	ResponseTimeCh          chan float64
	FirstTimeResponseTimeCh chan float64
	OverageSleepTimeCh      chan float64
	stopCh                  chan int
	tallyLogger             *logging.Logger
}

func NewStatLogger(stopCh chan int, logger *logging.Logger, startTally bool) *StatLogger {
	stat := &StatLogger{
		ResponseTimeCh:          make(chan float64, 1000),
		FirstTimeResponseTimeCh: make(chan float64, 1000),
		OverageSleepTimeCh:      make(chan float64, 1000),
		stopCh:                  stopCh,
		tallyLogger:             logger,
	}
	if startTally {
		go stat.TallyResponseTimes()
	}
	return stat
}

type StatsCommand int

const (
	StatsStop = iota
)

func (stat *StatLogger) TallyResponseTimes() {
	var data float64
	normalResponseTimes := newStats()
	firstResponseTimes := newStats()
	sleepTimeStats := newStats()
	logTimeout := time.Duration(5 * time.Second)
	logTimer := time.NewTimer(logTimeout)
	for {
		select {
		case <-stat.stopCh:
			return

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

var ActiveClientCount int64

func init() {
	ActiveClientCount = 0
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

func (r *statStruct) Avg() float64 {
	if r.count > 0 {
		r.avg = r.sum / float64(r.count)
	}
	return r.avg
}

func (r *statStruct) log(logger *logging.Logger, prefix string) {
	logger.Info("%s(min/avg/max): %8.2fms / %8.2fms / %8.2fms (connections: %7d,  messages: %7d)\n", prefix, r.min*1000.00, r.Avg()*1000.00, r.max*1000.00, ActiveClientCount, r.count)
}
