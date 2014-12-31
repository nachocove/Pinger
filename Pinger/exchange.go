package Pinger

import (
	"fmt"
	"sync"
	"time"
	"math/rand"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	client *Client
	debug bool
}

// String convert the ExchangeClient structure to something printable
func (ex *ExchangeClient) String() string {
	return fmt.Sprintf("Exchange %s", ex.client)
}

// Exchangehandler the incoming data handler
func Exchangehandler(data []byte) {
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
func (ex *ExchangeClient) Listen(wait *sync.WaitGroup) {
	ex.client.Listen(wait)
	go ex.periodicCheck()
}

// TODO This really ought to just be a method/interface thing

// NewExchangeClient set up a new exchange client
func NewExchangeClient(dialString string, debug bool) *ExchangeClient {
	client := NewClient(dialString, debug)
	client.incomingHandler = Exchangehandler
	return &ExchangeClient{client, debug}
}
