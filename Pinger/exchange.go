package Pinger

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/op/go-logging"
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

func getExchangeStatusMsg(localAddr string, count int) []byte {
	data := fmt.Sprintf("%d: Greetings from %s", count, localAddr)
	return []byte(data)
}

func (ex *ExchangeClient) doStats(t1 time.Time, firstTime bool) {
	dur := time.Since(t1)
	ex.client.logger.Debug("%s: Got response in %fms\n", ex.client.connection.LocalAddr().String(), dur.Seconds()*1000.00)
	if firstTime {
		firstTimeResponseTimeCh <- dur.Seconds()
	} else {
		responseTimeCh <- dur.Seconds()
	}
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
		data := getExchangeStatusMsg(localAddr, count)
		if ex.debug {
			ex.client.logger.Debug("%s: ExchangeClient sending \"%s\"", localAddr, data)
		}
		receiveTimeout := time.NewTimer(time.Duration(60) * time.Second)
		dataSentTime := time.Now()

		ex.client.outgoing <- data
		ex.client.logger.Debug("%s: Waiting for response", localAddr)
		select {
		case incomingData := <-ex.client.incoming:
			ex.doStats(dataSentTime, firstTime)
			firstTime = false
			incomingString := string(bytes.Trim(incomingData, "\000"))
			if incomingString != string(data) {
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
