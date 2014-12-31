package Pinger

import (
	"fmt"
	"sync"
	"time"
	"math/rand"
	"log"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	client *Client
	pingPeriodicity int
	debug bool
}

// String convert the ExchangeClient structure to something printable
func (ex *ExchangeClient) String() string {
	return fmt.Sprintf("Exchange %s", ex.client)
}

// Exchangehandler the incoming data handler
func exchangehandler(data []byte) {
	//fmt.Println("ExchangeClient received", string(data))
}

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

func (ex *ExchangeClient) periodicCheck() {
	data := fmt.Sprintf("Greetings from %s", ex.client.connection.LocalAddr().String())

	for {
		sleepTime := randomInt(10,30)
		if ex.debug {
			fmt.Printf("Sleeping %d\n", sleepTime)
		}
		time.Sleep(time.Duration(sleepTime)*time.Second)
		if ex.debug {
			fmt.Println("ExchangeClient sending", data)
		}
		ex.client.outgoing <- []byte(data)
	}
}
// Listen sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (ex *ExchangeClient) Listen(wait *sync.WaitGroup) (error) {
	// Listen launches 2 goroutines
	err := ex.client.Listen(wait)
	if ex.pingPeriodicity > 0 {
		go ex.periodicCheck()
	}
	return err // no error
}

// TODO This really ought to just be a method/interface thing

// NewExchangeClient set up a new exchange client
func NewExchangeClient(dialString string, pingPeriodicity int, debug bool) *ExchangeClient {
	client := NewClient(dialString, debug)
	if client == nil {
		log.Println("Could not get Client")
		return nil
	}
	client.incomingHandler = exchangehandler
	return &ExchangeClient{client, pingPeriodicity, debug}
}
