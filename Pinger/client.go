package Pinger

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	logging "github.com/nachocove/Pinger/Pinger/logging"
	"github.com/nachocove/Pinger/Utils"
)

type PingerCommand int

const (
	noCommand PingerCommand = 0
	// Stop stop the client
	PingerStop PingerCommand = 1
	// Defer the client, i.e. stop all activity and sleep for a time
	PingerDefer PingerCommand = 2
)

// Client The client structure for tracking a particular endpoint
type Client struct {
	Connection net.Conn
	Incoming   chan []byte
	Outgoing   chan []byte
	Command    chan PingerCommand
	StopCh     chan int
	Err        chan error

	// private
	buffer        []byte
	waitGroup     *sync.WaitGroup
	debug         bool
	dialString    string
	reopenOnClose bool
	tlsConfig     *tls.Config
	tcpKeepAlive  int
	logger        *logging.Logger
}

// NewClient Set up a new client
func NewClient(dialString string, reopenOnClose bool, tlsConfig *tls.Config, tcpKeepAlive int, debug bool, logger *logging.Logger) *Client {
	client := &Client{
		Connection: nil,
		Incoming:   make(chan []byte, 2),
		Outgoing:   make(chan []byte, 2),
		Command:    make(chan PingerCommand, 2),
		Err:        make(chan error),
		StopCh:     make(chan int),

		// private
		dialString:    dialString,
		waitGroup:     nil,
		debug:         debug,
		reopenOnClose: reopenOnClose,
		tlsConfig:     tlsConfig,
		tcpKeepAlive:  tcpKeepAlive,
		logger:        logger,
	}
	return client
}

// String Convert the client structure into a printable string
func (client *Client) String() string {
	return fmt.Sprintf("Client %s (debug %t)", client.Connection.RemoteAddr().String(), client.debug)
}

// Done The client is exiting. Cleanup and alert anyone waiting.
func (client *Client) Done() {
	close(client.StopCh)
	if client.debug {
		client.logger.Info("Finished with Client")
	}
	if client.waitGroup != nil {
		client.waitGroup.Done()
	}
	Utils.ActiveClientCount--
}

func (client *Client) connectionReader(Command <-chan PingerCommand) {
	if client.buffer == nil {
		client.buffer = make([]byte, 512)
	}
	for {
		if client.Connection == nil {
			time.Sleep(1)
			continue
		}

		if len(Command) > 0 {
			cmd := <-Command
			if cmd == PingerStop {
				if client.debug {
					client.logger.Info("Was told to stop. Exiting")
				}
				return
			}
		}
		// try to read the data
		n, err := client.Connection.Read(client.buffer)
		if err != nil {
			// send an error if it's encountered
			client.Err <- err
			return
		}
		if n <= 0 {
			client.logger.Error("Read %d bytes\n", n)
		}

		// send data if we read some.
		client.Incoming <- client.buffer
	}
}

// Wait The wait loop. Send outgoing data down the connection, and gets incoming data off the connection and puts it on the channel.
// Is itself launched as a goroutine, and adds a single goroutine for listening on the connection
func (client *Client) wait() {
	defer client.Done()
	if client.Connection == nil {
		client.logger.Fatal("Wait called without an open connection")
	}
	defer client.closeConn()

	// Start a goroutine to read from our net connection
	connectionCommand := make(chan PingerCommand, 1)
	go client.connectionReader(connectionCommand)
	defer func(Command chan<- PingerCommand) {
		Command <- PingerStop
	}(connectionCommand)

	for {
		var exitLoop = false
		if client.Connection == nil {
			if client.debug {
				client.logger.Debug("reopening connection")
			}
			client.openConn()
		}
		select {
		case data := <-client.Outgoing:
			// write data to the connection
			_, err := client.Connection.Write(data)
			if err != nil {
				client.logger.Error("write: %s\n", err.Error())
				client.closeConn()
				if client.reopenOnClose == false {
					exitLoop = true
				}
			}

		case err := <-client.Err:
			// handle our error then exit for loop
			if err == io.EOF {
				client.logger.Info("Connection closed\n")
			} else {
				client.logger.Error("channel: %s\n", err.Error())
			}
			exitLoop = true

		case cmd := <-client.Command:
			switch cmd {
			case PingerStop:
				if client.debug {
					client.logger.Debug("Stopping")
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

func (client *Client) openConn() error {
	connection, err := net.Dial("tcp", client.dialString)
	if err != nil {
		return err
	}
	tcpconn := connection.(*net.TCPConn)
	if client.tcpKeepAlive != 0 {
		tcpconn.SetKeepAlive(true)
		tcpconn.SetKeepAlivePeriod(time.Duration(client.tcpKeepAlive) * time.Second)
	} else {
		tcpconn.SetKeepAlive(false)
	}
	if client.tlsConfig != nil {
		connection = tls.Client(connection, client.tlsConfig)
		if client.debug {
			client.logger.Info("Opened TLS Connection")
		}
	} else {
		if client.debug {
			client.logger.Warning("Opened TCP-only Connection")
		}
	}
	client.Connection = connection
	if client.Connection == nil {
		return errors.New("Could not open connection")
	}
	return nil
}

func (client *Client) closeConn() {
	if client.Connection != nil {
		client.Connection.Close()
		client.Connection = nil
	}
}

// Listen Set up the go routine for monitoring the connection. Also mark the client as running in case anyone is waiting.
// This function creates a go routine (Wait()), which itself adds 1 goroutines for listening.
func (client *Client) Listen(wait *sync.WaitGroup) error {
	client.logger.Debug("Starting client")
	client.waitGroup = wait // can be nil
	err := client.openConn()
	if err != nil {
		return err
	}
	go client.wait()
	if client.waitGroup != nil {
		client.waitGroup.Add(1)
	}
	Utils.ActiveClientCount++
	return nil
}
