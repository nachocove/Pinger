package Pinger

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"math/rand"
	"time"
	"sync"

	"github.com/op/go-logging"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	client          *Client
	waitBeforeUse   int
	pingPeriodicity int
	debug           bool
	logger	*logging.Logger
	stats *StatLogger
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

func (ex *ExchangeClient) doStats(t1 time.Time, firstTime bool) {
	dur := time.Since(t1)
	ex.logger.Debug("%s: Got response in %fms\n", ex.client.Connection.LocalAddr().String(), dur.Seconds()*1000.00)
	if firstTime {
		ex.stats.FirstTimeResponseTimeCh <- dur.Seconds()
	} else {
		ex.stats.ResponseTimeCh <- dur.Seconds()
	}
}

func (ex *ExchangeClient) periodicCheck(pi *MailPingInformation) {
	localAddr := ex.client.Connection.LocalAddr().String()
	firstTime := true
	count := 0
	for {
		count++
		
		if pi.WaitBeforeUse > 0 {
			time.Sleep(time.Duration(pi.WaitBeforeUse)*time.Second)
		}

		receiveTimeout := time.NewTimer(time.Duration(pi.ResponseTimeout)*time.Second)
		dataSentTime := time.Now()
		ex.client.Outgoing <- pi.HttpRequestData
		ex.logger.Debug("%s: Waiting for response", localAddr)
		select {
		case incomingData := <-ex.client.Incoming:
			ex.doStats(dataSentTime, firstTime)
			firstTime = false
			incomingString := bytes.Trim(incomingData, "\000")
			switch {
			case (pi.HttpNoChangeReply != nil && bytes.Compare(incomingString, pi.HttpNoChangeReply) == 0):
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", localAddr)
			
			case (pi.HttpExpectedReply == nil || (pi.HttpExpectedReply != nil && bytes.Compare(incomingString, pi.HttpExpectedReply) == 0)):
				// there's new mail!
				ex.logger.Debug("%s: Reply matched HttpExpectedReply. Send Push", localAddr)
				panic("Not yet implemented")
			}
			receiveTimeout.Stop()

		case <-receiveTimeout.C:
			ex.logger.Error("%s: No response in allotted time", localAddr)
		}
		sleepTime := ex.pingPeriodicity + randomInt(1, 5)
		if ex.debug {
			ex.logger.Debug("%s: Sleeping %d\n", localAddr, sleepTime)
		}
		t1 := time.Now()
		time.Sleep(time.Duration(sleepTime) * time.Second)
		slept := time.Since(t1).Seconds()
		ex.logger.Debug("%s: Should have slept for %d. Slept for %f", localAddr, sleepTime, slept)
		overTime := slept - float64(sleepTime)
		if overTime > 0 {
			ex.stats.OverageSleepTimeCh <- overTime
		} else {
			ex.logger.Info("%s: EARLY: Woke up %fms before allotted time.", localAddr, overTime)
		}
	}
}

// Listen sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (ex *ExchangeClient) Listen(pi* MailPingInformation, wait *sync.WaitGroup) error {
	// Listen launches 2 goroutines
	err := ex.client.Listen(wait)
	if err == nil && ex.pingPeriodicity > 0 {
		go ex.periodicCheck(pi)
	}
	return err // could be nil
}

func (ex *ExchangeClient) Action(action int) error {
	ex.client.Command <- action
	return nil
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(mailInfo *MailPingInformation, debug bool, logger *logging.Logger) *ExchangeClient {
	panic("Need to implement this better")
	dialString := ""
	reopenConnection := true
	var tlsConfig *tls.Config = nil
	tcpKeepAlive := 0
	client := NewClient(dialString, reopenConnection, tlsConfig, tcpKeepAlive, debug, logger)
	var Log = GetLogger("exchange-client")
	if client == nil {
		Log.Error("Could not get Client")
		return nil
	}
	return &ExchangeClient{
		client:          client,
		pingPeriodicity: 5,
		debug:           debug,
		stats: NewStatLogger(logger),
		logger: logger,
	}
}
