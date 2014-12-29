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
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

const (
	minWaitTime = 10
	maxWaitTime = 30
)

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}
func handleConnection(conn net.Conn) {
	defer conn.Close()
	connections := Pinger.MakeChan(conn)
	remote := conn.RemoteAddr().String()
	if debug {
		fmt.Printf("%s: Got connection\n", remote)
	}

	sleepTime := randomInt(minWaitTime, maxWaitTime)
	timer := time.NewTimer(time.Duration(sleepTime) * time.Second)
	defer timer.Stop()

	// continuously read from the connection
	for {
		var exit_loop = false
		fmt.Printf("%s: Waiting %d seconds for something to happen\n", remote, sleepTime)
		select {
		// This case means we recieved data on the connection
		case data := <-connections.Ch:
			// just write the data back. We are the ultimate echo.
			conn.Write(data)

		// This case means we got an error and the goroutine has finished
		case err := <-connections.ECh:
			// handle our error then exit for loop
			if err == io.EOF {
				fmt.Printf("%s: Connection closed\n", remote)
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
	if debug {
		fmt.Printf("%s: Exiting\n", remote)
	}
}

var debug bool

func main() {
	flag.BoolVar(&debug, "d", false, "Debugging")

	flag.Parse()

	port := 8082
	bindAddress := ""
	dialString := fmt.Sprintf("%s:%d", bindAddress, port)
	log.Printf("Listening on %s\n", dialString)
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
		go handleConnection(conn)
	}
}
