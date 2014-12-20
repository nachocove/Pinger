package main

import (
	"flag"
	"fmt"
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
	minWaitTime = 5
	maxWaitTime = 30
)

func handleConnection(conn net.Conn) {
	defer conn.Close()
	if debug {
		log.Printf("Got connection from %s\n", conn.RemoteAddr().String())
	}
	sleep_time := time.Duration(rand.Intn(maxWaitTime-minWaitTime) + minWaitTime)

	if debug {
		fmt.Printf("Sleeping %d seconds\n", sleep_time)
	}
	time.Sleep(sleep_time * time.Second)
	if debug {
		fmt.Println("Exiting")
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
