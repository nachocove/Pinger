package Pinger

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
	"crypto/tls"
	"errors"
)

// HandlerFunc Function used to handle incoming data on a channel.
type HandlerFunc func([]byte)

// HandleIncoming set the incoming data handler
func (client *Client) HandleIncoming(handler HandlerFunc) {
	client.incomingHandler = handler
}

const (
	NoCommand = iota
	Stop      = iota
)

// Client The client structure for tracking a particular endpoint
type Client struct {
	connection      interface{net.Conn}
	incoming        chan []byte
	outgoing        chan []byte
	command         chan int
	err             chan error
	waitGroup       *sync.WaitGroup
	debug           bool
	incomingHandler HandlerFunc
	dialString      string
	reopenOnClose   bool
	tlsConfig       *tls.Config
}

// NewClient Set up a new client
func NewClient(dialString string, reopenOnClose bool, tlsConfig *tls.Config, debug bool) *Client {
	client := &Client{
		dialString:      dialString,
		connection:      nil,
		incoming:        make(chan []byte),
		outgoing:        make(chan []byte),
		command:         make(chan int, 2),
		err:             make(chan error),
		waitGroup:       nil,
		debug:           debug,
		incomingHandler: printIncoming,
		reopenOnClose:   reopenOnClose,
		tlsConfig:       tlsConfig,
	}
	return client
}

// ActiveClientCount count of actively open connections
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

func (client *Client) connectionReader(command <-chan int) {
	data := make([]byte, 512)
	for {
		if client.connection == nil {
			time.Sleep(1)
			continue
		}

		select {
		case cmd := <-command:
			if cmd == Stop {
				if client.debug {
					log.Println("Was told to stop. Exiting")
				}
				return
			}
		}
		// try to read the data
		_, err := client.connection.Read(data)
		if err != nil {
			// send an error if it's encountered
			client.err <- err
			return
		}
		// send data if we read some.
		client.incoming <- data
	}
}

// Wait The wait loop. Send outgoing data down the connection, and gets incoming data off the connection and puts it on the channel.
// Is itself launched as a goroutine, and adds a single goroutine for listening on the connection
func (client *Client) wait() {
	defer client.Done()
	if client.connection == nil {
		log.Fatalln("Wait called without an open connection")
	}
	defer client.closeConn()

	// Start a goroutine to read from our net connection
	connectionCommand := make(chan int)
	go client.connectionReader(connectionCommand)
	defer func(command chan<- int) {
		command <- Stop
	}(connectionCommand)

	for {
		var exitLoop = false
		if client.connection == nil {
			if client.debug {
				log.Println("reopening connection")
			}
			client.openConn()
		}
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
				client.closeConn()
				if client.reopenOnClose == false {
					exitLoop = true
				}
			}

		case err := <-client.err:
			// handle our error then exit for loop
			if err == io.EOF {
				log.Printf("Connection closed\n")
			} else {
				log.Printf("Error from channel: %s\n", err.Error())
			}
			exitLoop = true

		case cmd := <-client.command:
			switch cmd {
			case Stop:
				if client.debug {
					log.Println("Stopping")
				}
				exitLoop = true
				// don't try to reopen anything. We're outta here. 
				client.reopenOnClose = false
			}
		}
		if exitLoop {
			if client.reopenOnClose == false {
				break
			} else {
				client.closeConn()
			}
		}
	}
}

func printIncoming(data []byte) {
	log.Println(data)
}

func (client *Client) openConn() error {
	if client.tlsConfig == nil {
		return errors.New("tlsConfig can not be nil")
	}
	connection, err := tls.Dial("tcp", client.dialString, client.tlsConfig)
	if err != nil {
		return err
	}
	client.connection = connection
	if client.connection == nil {
		return errors.New("Could not open connection")
	}
	return nil
}

func (client *Client) closeConn() {
	if client.connection != nil {
		client.connection.Close()
		client.connection = nil
	}
}

// Listen Set up the go routine for monitoring the connection. Also mark the client as running in case anyone is waiting.
// This function creates a go routine (Wait()), which itself adds 1 goroutines for listening.
func (client *Client) Listen(wait *sync.WaitGroup) error {
	if client.debug {
		log.Println("Starting client")
	}
	client.waitGroup = wait
	err := client.openConn()
	if err != nil {
		return err
	}
	go client.wait()
	if client.waitGroup != nil {
		client.waitGroup.Add(1)
	}
	ActiveClientCount++
	return nil
}
