package Pinger

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

func MakeChan(conn net.Conn) (chan []byte, chan error) {
	ch := make(chan []byte)
	eCh := make(chan error)

	// Start a goroutine to read from our net connection
	go func(ch chan []byte, eCh chan error) {
		data := make([]byte, 512)
		for {
			// try to read the data
			_, err := conn.Read(data)
			if err != nil {
				// send an error if it's encountered
				eCh <- err
				return
			}
			// send data if we read some.
			ch <- data
		}
	}(ch, eCh)

	return ch, eCh
}

type HandlerFunc func([]byte)

func (client *Client) HandleIncoming(handler HandlerFunc) {
	client.incomingHandler = handler
}

type Client struct {
	connection      net.Conn
	incoming        chan []byte
	outgoing        chan []byte
	err             chan error
	waitGroup       *sync.WaitGroup
	debug           bool
	incomingHandler HandlerFunc
}

func (client *Client) String() string {
	return fmt.Sprintf("Client %s (debug %t)", client.connection.RemoteAddr().String(), client.debug)
}

func (client *Client) Done() {
	if client.debug {
		log.Println("Finished with Client")
	}
	if client.waitGroup != nil {
		client.waitGroup.Done()
	}
}

func (client *Client) Wait() {
	if client.waitGroup != nil {
		defer client.waitGroup.Done()
	}
	defer client.connection.Close()

	for {
		var exit_loop = false
		select {
		case data := <-client.incoming:
			// just write the data back. We are the ultimate echo.
			if client.incomingHandler != nil {
				client.incomingHandler(data)
			}

		case data := <-client.outgoing:
			// write data to the connection
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

func printIncoming(data []byte) {
	fmt.Println(data)
}

func (client *Client) Listen(wait *sync.WaitGroup) {
	if client.debug {
		log.Println("Starting client")
	}

	client.waitGroup = wait

	go client.Wait()
	if client.waitGroup != nil {
		client.waitGroup.Add(1)
	}
}

func NewClient(dialString string, debug bool) *Client {
	connection, err := net.Dial("tcp", dialString)
	if err != nil {
		log.Println("Could not open connection", err.Error())
		return nil
	}

	ch, eCh := MakeChan(connection)

	client := &Client{
		connection:      connection,
		incoming:        ch,
		outgoing:        make(chan []byte),
		err:             eCh,
		waitGroup:       nil,
		debug:           debug,
		incomingHandler: printIncoming,
	}
	return client
}
