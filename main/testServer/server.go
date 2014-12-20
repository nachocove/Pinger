package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"runtime/pprof"
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
	fmt.Printf("Got connection from %s\n", conn.RemoteAddr().String())
	sleep_time := time.Duration(rand.Intn(maxWaitTime-minWaitTime) + minWaitTime)

	fmt.Printf("Sleeping %d seconds\n", sleep_time)
	time.Sleep(sleep_time * time.Second)
	fmt.Println("Exiting")
}

func main() {
	port := 8082
	bindAddress := ""
	dialString := fmt.Sprintf("%s:%d", bindAddress, port)
	fmt.Printf("Listening on %s\n", dialString)
	ln, err := net.Listen("tcp", dialString)
	if err != nil {
		fmt.Println("Could not open connection", err.Error())
		os.Exit(1)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Could not accept connection", err.Error())
			continue
		}
		go handleConnection(conn)
	}
	f, err := os.Create("memprofile.pprof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.WriteHeapProfile(f)
	f.Close()
}
