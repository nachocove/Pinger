package Pinger

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/op/go-logging"
	"math/rand"
	"sync"
	"time"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	client          *Client
	pingPeriodicity int
	debug           bool
}

// String convert the ExchangeClient structure to something printable
func (ex *ExchangeClient) String() string {
	return fmt.Sprintf("Exchange %s", ex.client)
}

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

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
		tallyLogger.Info("%s(min/avg/max): %8.2fms / %8.2fms / %8.2fms (connections: %7d,  messages: %7d)\n", prefix, r.min*1000.00, r.avg*1000.00, r.max*1000.00, ActiveClientCount, r.count)
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

func (ex *ExchangeClient) periodicCheck() {
	localAddr := ex.client.connection.LocalAddr().String()
	firstTime := true
	count := 0
	if tallyLogger == nil {
		tallyLogger = ex.client.logger
	}

	for {
		count++
		data := fmt.Sprintf("%d: Greetings from %s", count, localAddr)
		if ex.debug {
			ex.client.logger.Debug("%s: ExchangeClient sending \"%s\"", localAddr, data)
		}
		receiveTimeout := time.NewTimer(time.Duration(60) * time.Second)
		dataSentTime := time.Now()
		ex.client.outgoing <- []byte(data)

		ex.client.logger.Debug("%s: Waiting for response", localAddr)
		select {
		case incomingData := <-ex.client.incoming:
			responseTime := time.Since(dataSentTime)
			ex.client.logger.Debug("%s: Got response in %fms\n", localAddr, responseTime.Seconds()*1000.00)
			if firstTime {
				firstTime = false
				firstTimeResponseTimeCh <- responseTime.Seconds()
			} else {
				responseTimeCh <- responseTime.Seconds()
			}
			incomingString := string(bytes.Trim(incomingData, "\000"))
			if incomingString != data {
				ex.client.logger.Warning("%s: Received data does not match: \n (%d) %s\n (%d) %s\n", localAddr, len(incomingString), incomingString, len(data), data)
				continue
			} else {
				ex.client.logger.Debug("%s: Received string matches", localAddr)
			}
			receiveTimeout.Stop()

		case <-receiveTimeout.C:
			ex.client.logger.Error("%s: No response in allotted time", localAddr)
		}
		sleepTime := ex.pingPeriodicity + randomInt(1, 5)
		if ex.debug {
			ex.client.logger.Debug("%s: Sleeping %d\n", localAddr, sleepTime)
		}
		t1 := time.Now()
		time.Sleep(time.Duration(sleepTime) * time.Second)
		slept := time.Since(t1).Seconds()
		ex.client.logger.Debug("%s: Should have slept for %d. Slept for %f", localAddr, sleepTime, slept)
		overTime := slept - float64(sleepTime)
		if overTime > 0 {
			overageSleepTimeCh <- overTime
		} else {
			ex.client.logger.Info("%s: EARLY: Woke up %fms before allotted time.", localAddr, overTime)
		}
	}
}

// Listen sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (ex *ExchangeClient) Listen(wait *sync.WaitGroup) error {
	// Listen launches 2 goroutines
	err := ex.client.Listen(wait)
	if err == nil && ex.pingPeriodicity > 0 {
		go ex.periodicCheck()
	}
	return err // could be nil
}

// TODO This really ought to just be a method/interface thing

// NewExchangeClient set up a new exchange client
func NewExchangeClient(dialString string, pingPeriodicity int, reopenConnection bool, tlsConfig *tls.Config, tcpKeepAlive int, debug bool, logger *logging.Logger) *ExchangeClient {
	client := NewClient(dialString, reopenConnection, tlsConfig, tcpKeepAlive, debug, logger)
	var Log = GetLogger("exchange-client")
	if client == nil {
		Log.Error("Could not get Client")
		return nil
	}
	return &ExchangeClient{
		client:          client,
		pingPeriodicity: pingPeriodicity,
		debug:           debug,
	}
}
