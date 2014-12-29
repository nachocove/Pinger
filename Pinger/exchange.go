package Pinger

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type ExchangeClient struct {
	connection net.Conn
	incoming   chan []byte
	outgoing   chan []byte
	err        chan error
	waitGroup  *sync.WaitGroup
	debug      bool
}

func (client *ExchangeClient) Done() {
	if client.debug {
		log.Println("Finished with Client")
	}
	client.waitGroup.Done()
}

func (client *ExchangeClient) Wait() {
	defer client.waitGroup.Done()
	defer client.connection.Close()
	// continuously read from the connection
	for {
		var exit_loop = false
		select {
		case data := <-client.incoming:
			// just write the data back. We are the ultimate echo.
			fmt.Println(data)

		case data := <-client.outgoing:
			_, err := client.connection.Write(data)
			if err != nil {
				fmt.Println("ERROR", err.Error())
				exit_loop = true
			}

		case err := <-client.err:
			// handle our error then exit for loop
			if err == io.EOF {
				if client.debug {
					fmt.Printf("Connection closed\n")
				}
			} else {
				fmt.Printf("Error %s\n", err.Error())
			}
			exit_loop = true
		}
		if exit_loop {
			break
		}
	}
}

func NewExchangeClient(dialString string, wait *sync.WaitGroup, debug bool) *ExchangeClient {
	connection, err := net.Dial("tcp", dialString)
	if err != nil {
		log.Println("Could not open connection", err.Error())
		return nil
	}

	connections := MakeChan(connection)

	client := &ExchangeClient{
		connection: connection,
		incoming:   make(chan []byte),
		outgoing:   connections.Ch,
		err:        connections.ECh,
		waitGroup:  wait,
		debug:      debug,
	}

	if client.debug {
		log.Println("Starting client")
	}
	go client.Wait()

	client.waitGroup.Add(1)
	return client
}
