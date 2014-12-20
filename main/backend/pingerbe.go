package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

var connections []net.Conn

func makeConnection(mychan chan int) {
	dialString := "localhost:8082"
	fmt.Println("Opening connection to", dialString)
	conn, err := net.Dial("tcp", dialString)
	if err != nil {
		fmt.Println("Could not open connection", err.Error())
		os.Exit(1)
	}
	defer func(conn net.Conn) {
		fmt.Println("Cleaning up connection")
		conn.Close()
		mychan <- 1
	}(conn)

	data := make([]byte, 512)
	fmt.Println("Reading 1 bytes")
	_, err = conn.Read(data[0:1])
	if err != nil && err != io.EOF {
		fmt.Println("Error on connection read", err.Error())
		return
	}
}
func main() {
	maxConnection := 1000
	mychan := make(chan int, maxConnection)
	for i := 0; i < maxConnection; i++ {
		go makeConnection(mychan)
	}
	for i := 0; i < maxConnection; i++ {
		fmt.Println("Reaping connection goroutine")
		<-mychan
	}
}
