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

func (ex *ExchangeClient) periodicCheck() {
	data := fmt.Sprintf("%s: Greetings from %s", time.Now(), ex.client.connection.LocalAddr().String())

	for {
		if ex.debug {
			Log.Debug("ExchangeClient sending \"%s\"", data)
		}
		receiveTimeout := time.NewTimer(time.Duration(60) * time.Second)
		dataSentTime := time.Now()
		ex.client.outgoing <- []byte(data)

		Log.Debug("Waiting for response")
		select {
		case incomingData := <-ex.client.incoming:
			responseTime := time.Since(dataSentTime)
			Log.Debug("Got response in %f ms\n", responseTime.Seconds()*float64(time.Millisecond))
			incomingString := string(bytes.Trim(incomingData, "\000"))
			if incomingString != data {
				Log.Debug("Received data does not match: \n (%d) %s\n (%d) %s\n", len(incomingString), incomingString, len(data), data)
				continue
			} else {
				Log.Debug("Received string matches")
			}
			receiveTimeout.Stop()

		case <-receiveTimeout.C:
			Log.Error("No response in allotted time")
		}
		sleepTime := ex.pingPeriodicity + randomInt(1, 5)
		if ex.debug {
			Log.Debug("Sleeping %d\n", sleepTime)
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
