package Pinger

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

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
	dialString		string
}

var ActiveClientCount int

func init() {
	ActiveClientCount = 0
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
	ActiveClientCount--
}

// Wait The wait loop. Send outgoing data down the connection, and gets incoming data off the connection and puts it on the channel.
// Is itself launched as a goroutine, and adds a single goroutine for listening on the connection
func (client *Client) Wait() {
	defer client.Done()
	if client.connection == nil {
		log.Fatalln("Wait called without an open connection")
	}
	defer client.connection.Close()

	// Start a goroutine to read from our net connection
	go func(conn net.Conn, ch chan []byte, eCh chan error) {
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
	}(client.connection, client.incoming, client.err)

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
				log.Printf("ERROR on write: %s\n", err.Error())
				exitLoop = true
			}

		case err := <-client.err:
			// handle our error then exit for loop
			if err == io.EOF {
				if client.debug {
					log.Printf("Connection closed\n")
				}
			} else {
				log.Printf("Error from channel: %s\n", err.Error())
			}
			exitLoop = true
		}
		if exitLoop {
			break
		}
	}
}

func printIncoming(data []byte) {
	log.Println(data)
}

// Listen Set up the go routine for monitoring the connection. Also mark the client as running in case anyone is waiting.
// This function creates a go routine (Wait()), which itself adds 1 goroutines for listening.
func (client *Client) Listen(wait *sync.WaitGroup) (error) {
	if client.debug {
		log.Println("Starting client")
	}
	client.waitGroup = wait
	connection, err := net.Dial("tcp", client.dialString)
	if err != nil {
		return err
	}
	client.connection = connection	
	go client.Wait()
	if client.waitGroup != nil {
		client.waitGroup.Add(1)
	}
	ActiveClientCount++
	return nil
}

// NewClient Set up a new client
func NewClient(dialString string, debug bool) *Client {
	client := &Client{
		dialString:      dialString,
		connection:      nil,
		incoming:        make(chan []byte),
		outgoing:        make(chan []byte),
		err:             make(chan error),
		waitGroup:       nil,
		debug:           debug,
		incomingHandler: printIncoming,
	}
	return client
}
