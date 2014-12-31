package main

import (
	"flag"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path"
	"time"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

var ActiveConnections int = 0

// handleConnection Creates channels for incoming data and error, starts a single goroutine, and echoes all data received back.
func handleConnection(conn net.Conn, disconnectTime int) {
	defer conn.Close()
	inCh := make(chan []byte)
	eCh := make(chan error)
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
	}(conn, inCh, eCh)

	remote := conn.RemoteAddr().String()
	if debug || verbose {
		log.Printf("%s: Got connection\n", remote)
	}
	ActiveConnections++

	timer := time.NewTimer(time.Duration(disconnectTime) * time.Second)
	defer timer.Stop()

	// continuously read from the connection
	for {
		var exit_loop = false
		if debug {
			log.Printf("%s: Waiting %d seconds for something to happen\n", remote, disconnectTime)
		}
		select {
		// This case means we recieved data on the connection
		case data := <-inCh:
			// just write the data back. We are the ultimate echo.
			if debug {
				log.Printf("Received data and sending it back: %s\n", string(data))
			}
			conn.Write(data)

		// This case means we got an error and the goroutine has finished
		case err := <-eCh:
			// handle our error then exit for loop
			if err == io.EOF {
				if debug || verbose {
					log.Printf("%s: Connection closed\n", remote)
				}
			} else {
				log.Printf("%s: Error %s\n", remote, err.Error())
			}
			exit_loop = true

		case <-timer.C:
			if debug {
				log.Printf("%s: Timer expired.\n", remote)
			}
			exit_loop = true
		}
		if exit_loop {
			break
		}
	}
	if debug || verbose {
		log.Printf("%s: Closing connection\n", remote)
	}
	ActiveConnections--
}

var debug bool
var verbose bool
var usage = func() {
	fmt.Fprintf(os.Stderr, "USAGE: %s ....\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo() string {
	return fmt.Sprintf("number of connections: %d", ActiveConnections)
}

func main() {
	var port int
	var help bool
	var minWaitTime int
	var maxWaitTime int
	var logFileName string
	var bindAddress string
	var printMemPeriodic int

	flag.IntVar(&port, "p", 8082, "Listen port")
	flag.IntVar(&minWaitTime, "min", 30, "min wait time")
	flag.IntVar(&maxWaitTime, "max", 600, "max wait time")
	flag.StringVar(&logFileName, "l", "", "log file")
	flag.StringVar(&bindAddress, "b", "", "bind address")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Verbose")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}

	var logOutput io.Writer = nil

	//	if logFileName != "" {
	//		var logFile *os.File
	//		logFile, err := os.OpenFile(logFileName, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	//		if err != nil {
	//	    	log.Fatalf("error opening file %s: %v", logFileName, err)
	//		}
	//		defer logFile.Close()
	//		logOutput = io.Writer(logFile)
	//	} else {
	//		logFile, err := os.OpenFile("/dev/null", os.O_RDWR, 0666)
	//		if err != nil {
	//	    	log.Fatalf("error opening /dev/null %s: %v", logFileName, err)
	//		}
	//		logOutput = io.Writer(logFile)
	//	}
	//	if verbose || debug {
	//		logOutput = io.MultiWriter(os.Stdout, logOutput)
	//	}
	logOutput = io.Writer(os.Stdout)
	log.SetOutput(logOutput)

	dialString := fmt.Sprintf("%s:%d", bindAddress, port)
	if verbose {
		log.Printf("Listening on %s\n", dialString)
	}
	ln, err := net.Listen("tcp", dialString)
	if err != nil {
		log.Println("Could not open connection", err.Error())
		os.Exit(1)
	}
	var memstats *Pinger.MemStats
	if printMemPeriodic > 0 {
		memstats = Pinger.NewMemStats(printMemPeriodic, memStatsExtraInfo)
		memstats.PrintMemStatsPeriodic()
	}

	if debug {
		log.Printf("Min %d, Max %d\n", minWaitTime, maxWaitTime)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Could not accept connection", err.Error())
			continue
		}
		disconnectTime := randomInt(minWaitTime, maxWaitTime)

		// this adds 2 goroutines per connection. One the handleConnection itself, which then launches a read-goroutine
		go handleConnection(conn, disconnectTime)
	}
}
