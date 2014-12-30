package Pinger

import (
	"fmt"
	"sync"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	client *Client
}

// String convert the ExchangeClient structure to something printable
func (ex *ExchangeClient) String() string {
	return fmt.Sprintf("Exchange %s", ex.client)
}

// Exchangehandler the incoming data handler
func Exchangehandler(data []byte) {
	fmt.Println("ExchangeClient received", string(data))
}

// Listen sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
func (ex *ExchangeClient) Listen(wait *sync.WaitGroup) {
	ex.client.Listen(wait)

	data := "12345"
	fmt.Println("ExchangeClient sending", data)
	ex.client.outgoing <- []byte("12345")
}

// TODO This really ought to just be a method/interface thing

// NewExchangeClient set up a new exchange client
func NewExchangeClient(dialString string, debug bool) *ExchangeClient {
	client := NewClient(dialString, debug)
	client.incomingHandler = Exchangehandler
	return &ExchangeClient{client}
}
