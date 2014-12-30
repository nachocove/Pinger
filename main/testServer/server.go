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
	"time"
	"path"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

func handleConnection(conn net.Conn, disconnectTime int) {
	defer conn.Close()
	ch, eCh := Pinger.MakeChan(conn)
	remote := conn.RemoteAddr().String()
	if debug || verbose {
		fmt.Printf("%s: Got connection\n", remote)
	}

	timer := time.NewTimer(time.Duration(disconnectTime) * time.Second)
	defer timer.Stop()

	// continuously read from the connection
	for {
		var exit_loop = false
		if debug {
			fmt.Printf("%s: Waiting %d seconds for something to happen\n", remote, disconnectTime)
		}
		select {
		// This case means we recieved data on the connection
		case data := <-ch:
			// just write the data back. We are the ultimate echo.
			if debug {
				fmt.Printf("Received data and sending it back: %s\n", string(data))
			}
			conn.Write(data)

		// This case means we got an error and the goroutine has finished
		case err := <-eCh:
			// handle our error then exit for loop
			if err == io.EOF {
				if debug || verbose {
					fmt.Printf("%s: Connection closed\n", remote)
				}
			} else {
				fmt.Printf("%s: Error %s\n", remote, err.Error())
			}
			exit_loop = true

		case <-timer.C:
			if debug {
				fmt.Printf("%s: Timer expired.\n", remote)
			}
			exit_loop = true
		}
		if exit_loop {
			break
		}
	}
	if debug || verbose {
		fmt.Printf("%s: Closing connection\n", remote)
	}
}

var debug bool
var verbose bool
var usage = func() {
	fmt.Fprintf(os.Stderr, "USAGE: %s ....\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func main() {
	var port int
	var help bool
	var minWaitTime int
	var maxWaitTime int
	
	flag.IntVar(&port, "p", 8082, "Listen port")
	flag.IntVar(&minWaitTime, "min", 30, "min wait time")
	flag.IntVar(&maxWaitTime, "max", 600, "max wait time")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Verbose")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}
	bindAddress := ""
	dialString := fmt.Sprintf("%s:%d", bindAddress, port)
	if verbose {
		log.Printf("Listening on %s\n", dialString)
		log.Printf("Max/Min: %d %d\n", minWaitTime, maxWaitTime) 
	}
	ln, err := net.Listen("tcp", dialString)
	if err != nil {
		log.Println("Could not open connection", err.Error())
		os.Exit(1)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Could not accept connection", err.Error())
			continue
		}
		disconnectTime := randomInt(minWaitTime, maxWaitTime)
		
		go handleConnection(conn, disconnectTime)
	}
}
