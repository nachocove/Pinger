package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/op/go-logging"
	"github.com/nachocove/Pinger/Pinger"
)

type TestClient struct {
	client          *Pinger.Client
	pingPeriodicity int
	debug           bool
	logger	*logging.Logger
	stats *Pinger.StatLogger
}

// String convert the TestClient structure to something printable
func (tc *TestClient) String() string {
	return fmt.Sprintf("tcchange %s", tc.client)
}

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

func getStatusMsg(localAddr string, count int) []byte {
	data := fmt.Sprintf("%d: Greetings from %s", count, localAddr)
	return []byte(data)
}

func (tc *TestClient) doStats(t1 time.Time, firstTime bool) {
	dur := time.Since(t1)
	tc.logger.Debug("%s: Got response in %fms\n", tc.client.Connection.LocalAddr().String(), dur.Seconds()*1000.00)
	if firstTime {
		tc.stats.FirstTimeResponseTimeCh <- dur.Seconds()
	} else {
		tc.stats.ResponseTimeCh <- dur.Seconds()
	}
}

func (tc *TestClient) periodicCheck() {
	localAddr := tc.client.Connection.LocalAddr().String()
	firstTime := true
	count := 0
	for {
		count++
		data := getStatusMsg(localAddr, count)
		if tc.debug {
			tc.logger.Debug("%s: TestClient sending \"%s\"", localAddr, data)
		}
		receiveTimeout := time.NewTimer(time.Duration(60) * time.Second)
		dataSentTime := time.Now()

		tc.client.Outgoing <- data
		tc.logger.Debug("%s: Waiting for response", localAddr)
		select {
		case incomingData := <-tc.client.Incoming:
			tc.doStats(dataSentTime, firstTime)
			firstTime = false
			incomingString := string(bytes.Trim(incomingData, "\000"))
			if incomingString != string(data) {
				tc.logger.Warning("%s: Received data does not match: \n (%d) %s\n (%d) %s\n", localAddr, len(incomingString), incomingString, len(data), data)
				continue
			} else {
				tc.logger.Debug("%s: Received string matches", localAddr)
			}
			receiveTimeout.Stop()

		case <-receiveTimeout.C:
			tc.logger.Error("%s: No response in allotted time", localAddr)
		}
		sleepTime := tc.pingPeriodicity + randomInt(1, 5)
		if tc.debug {
			tc.logger.Debug("%s: Sleeping %d\n", localAddr, sleepTime)
		}
		t1 := time.Now()
		time.Sleep(time.Duration(sleepTime) * time.Second)
		slept := time.Since(t1).Seconds()
		tc.logger.Debug("%s: Should have slept for %d. Slept for %f", localAddr, sleepTime, slept)
		overTime := slept - float64(sleepTime)
		if overTime > 0 {
			tc.stats.OverageSleepTimeCh <- overTime
		} else {
			tc.logger.Info("%s: EARLY: Woke up %fms before allotted time.", localAddr, overTime)
		}
	}
}

// Listen sets up the TestClient to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (tc *TestClient) Listen(pi* Pinger.MailPingInformation, wait *sync.WaitGroup) error {
	// Listen launches 2 goroutines
	err := tc.client.Listen(wait)
	if err == nil && tc.pingPeriodicity > 0 {
		go tc.periodicCheck()
	}
	return err // could be nil
}

func (tc *TestClient) Action(action int) error {
	tc.client.Command <- action
	return nil
}

// TODO This really ought to just be a method/interface thing

// NewTestClient set up a new TestClient
func NewTestClient(dialString string, pingPeriodic int, reopenConnection, debug bool, tcpKeepAlive int, tlsConfig *tls.Config, logger *logging.Logger) *TestClient {
	client := Pinger.NewClient(dialString, reopenConnection, tlsConfig, tcpKeepAlive, debug, logger)
	if client == nil {
		logger.Error("Could not get Client")
		return nil
	}
	return &TestClient{
		client:          client,
		pingPeriodicity: pingPeriodic,
		debug:           debug,
		logger: logger,
		stats: Pinger.NewStatLogger(logger),
		
	}
}
