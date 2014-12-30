package Pinger

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// MakeChan given a net.Conn connection, create a goroutine to listen on the connection, and create read and error channels for select.
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

// HandlerFunc Function used to handle incoming data on a channel.
type HandlerFunc func([]byte)

// HandleIncoming set the incoming data handler
func (client *Client) HandleIncoming(handler HandlerFunc) {
	client.incomingHandler = handler
}

// Client The client structure for tracking a particular endpoint
type Client struct {
	connection      net.Conn
	incoming        chan []byte
	outgoing        chan []byte
	err             chan error
	waitGroup       *sync.WaitGroup
	debug           bool
	incomingHandler HandlerFunc
}

// String Convert the client structure into a printable string
func (client *Client) String() string {
	return fmt.Sprintf("Client %s (debug %t)", client.connection.RemoteAddr().String(), client.debug)
}

// Done The client is exiting. Cleanup and alert anyone waiting.
func (client *Client) Done() {
	if client.debug {
		log.Println("Finished with Client")
	}
	if client.waitGroup != nil {
		client.waitGroup.Done()
	}
}

// Wait The wait loop. Send outgoing data down the connection, and gets incoming data off the connection and puts it on the channel.
func (client *Client) Wait() {
	if client.waitGroup != nil {
		defer client.waitGroup.Done()
	}
	defer client.connection.Close()

	for {
		var exitLoop = false
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
				exitLoop = true
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
			exitLoop = true
		}
		if exitLoop {
			break
		}
	}
}

func printIncoming(data []byte) {
	fmt.Println(data)
}

// Listen Set up the go routine for monitoring the connection. Also mark the client as running in case anyone is waiting.
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

// NewClient Set up a new client
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
