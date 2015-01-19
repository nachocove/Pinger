package Pinger

import (
	"time"

	"github.com/op/go-logging"
)

type statStruct struct {
	min   float64
	max   float64
	sum   float64
	avg   float64
	count int
}

func newStatStruct() *statStruct {
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

func (r *statStruct) log(prefix string) {
	if r.count > 0 {
		r.avg = r.sum / float64(r.count)
		if tallyLogger != nil {
			tallyLogger.Info("%s(min/avg/max): %8.2fms / %8.2fms / %8.2fms (connections: %7d,  messages: %7d)\n", prefix, r.min*1000.00, r.avg*1000.00, r.max*1000.00, ActiveClientCount, r.count)
		}
	}
}

var responseTimeCh chan float64
var firstTimeResponseTimeCh chan float64
var overageSleepTimeCh chan float64
var tallyLogger *logging.Logger

func tallyResponseTimes() {
	var data float64
	normalResponseTimes := newStatStruct()
	firstResponseTimes := newStatStruct()
	sleepTimeStats := newStatStruct()
	logTimeout := time.Duration(5 * time.Second)
	logTimer := time.NewTimer(logTimeout)
	for {
		select {
		case data = <-responseTimeCh:
			normalResponseTimes.addDataPoint(data)

		case data = <-firstTimeResponseTimeCh:
			firstResponseTimes.addDataPoint(data)

		case data = <-overageSleepTimeCh:
			sleepTimeStats.addDataPoint(data)

		case <-logTimer.C:
			firstResponseTimes.log("    first")
			normalResponseTimes.log("   normal")
			sleepTimeStats.log("sleepOver")
			logTimer.Reset(logTimeout)
		}
	}
}

func init() {
	responseTimeCh = make(chan float64, 1000)
	firstTimeResponseTimeCh = make(chan float64, 1000)
	overageSleepTimeCh = make(chan float64, 1000)
	go tallyResponseTimes()
}
