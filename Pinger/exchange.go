package Pinger

import (
	"fmt"
	"sync"
)

type ExchangeClient struct {
	client *Client
}

func (client *ExchangeClient) String() string {
	return fmt.Sprintf("Exchange %s", client.client)
}
func Exchangehandler(data []byte) {
	fmt.Println("ExchangeClient received", string(data))
}

func (ex *ExchangeClient) Listen(wait *sync.WaitGroup) {
	ex.client.Listen(wait)

	data := "12345"
	fmt.Println("ExchangeClient sending", data)
	ex.client.outgoing <- []byte("12345")
}

// TODO This really ought to just be a method/interface thing
func NewExchangeClient(dialString string, debug bool) *ExchangeClient {
	client := NewClient(dialString, debug)
	client.incomingHandler = Exchangehandler
	return &ExchangeClient{client}
}
