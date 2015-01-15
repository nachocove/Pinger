package Pinger

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/op/go-logging"
	"math"
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

type responseTimeStruct struct {
	min   float64
	max   float64
	sum   float64
	avg   float64
	count int
}

func (r *responseTimeStruct) addDataPoint(responseTime float64) {
	if responseTime < r.min {
		r.min = responseTime
	}
	if responseTime > r.max {
		r.max = responseTime
	}
	r.count++
	r.sum = r.sum + responseTime
}

func (r *responseTimeStruct) log(prefix string) {
	r.avg = r.sum / float64(r.count)
	Log.Info("%s(min/avg/max): %fms / %fms / %fms\n", prefix, r.min*1000.00, r.avg*1000.00, r.max*1000.00)
}

var responseTimeCh chan float64
var firstTimeResponseTimeCh chan float64

func tallyResponseTimes() {
	var responseTime float64
	normalResponseTimes := &responseTimeStruct{min: 1000000.00}
	firstResponseTimes := &responseTimeStruct{min: 1000000.00}
	count := 0
	for {
		select {
		case responseTime = <-responseTimeCh:
			normalResponseTimes.addDataPoint(responseTime)
			count++

		case responseTime = <-firstTimeResponseTimeCh:
			firstResponseTimes.addDataPoint(responseTime)
			count++
		}
		if math.Mod(float64(count), 10) == 0 {
			firstResponseTimes.log(" first")
			normalResponseTimes.log("normal")
		}
	}
}

func init() {
	responseTimeCh = make(chan float64, 1000)
	firstTimeResponseTimeCh = make(chan float64, 1000)
	go tallyResponseTimes()
}

func (ex *ExchangeClient) periodicCheck() {
	localAddr := ex.client.connection.LocalAddr().String()
	firstTime := true
	count := 0
	for {
		count++
		data := fmt.Sprintf("%d: Greetings from %s", count, localAddr)
		if ex.debug {
			Log.Debug("%s: ExchangeClient sending \"%s\"", localAddr, data)
		}
		receiveTimeout := time.NewTimer(time.Duration(60) * time.Second)
		dataSentTime := time.Now()
		ex.client.outgoing <- []byte(data)

		Log.Debug("%s: Waiting for response", localAddr)
		select {
		case incomingData := <-ex.client.incoming:
			responseTime := time.Since(dataSentTime)
			Log.Debug("%s: Got response in %fms\n", localAddr, responseTime.Seconds()*1000.00)
			if firstTime {
				firstTime = false
				firstTimeResponseTimeCh <- responseTime.Seconds()
			} else {
				responseTimeCh <- responseTime.Seconds()
			}
			incomingString := string(bytes.Trim(incomingData, "\000"))
			if incomingString != data {
				Log.Debug("%s: Received data does not match: \n (%d) %s\n (%d) %s\n", localAddr, len(incomingString), incomingString, len(data), data)
				continue
			} else {
				Log.Debug("%s: Received string matches", localAddr)
			}
			receiveTimeout.Stop()

		case <-receiveTimeout.C:
			Log.Error("%s: No response in allotted time", localAddr)
		}
		sleepTime := ex.pingPeriodicity + randomInt(1, 5)
		if ex.debug {
			Log.Debug("%s: Sleeping %d\n", localAddr, sleepTime)
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
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

var Log = GetLogger("exchange-client")

// TODO This really ought to just be a method/interface thing

// NewExchangeClient set up a new exchange client
func NewExchangeClient(dialString string, pingPeriodicity int, reopenConnection bool, tlsConfig *tls.Config, tcpKeepAlive int, debug bool, logger *logging.Logger) *ExchangeClient {
	client := NewClient(dialString, reopenConnection, tlsConfig, tcpKeepAlive, debug, logger)
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
