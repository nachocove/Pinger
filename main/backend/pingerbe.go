package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var connections []net.Conn

func makeConnection(mychan chan int) {
	dialString := "localhost:8082"
	if debug {
		log.Println("Opening connection to", dialString)
	}
	conn, err := net.Dial("tcp", dialString)
	defer func(conn net.Conn) {
		if debug {
			log.Println("Cleaning up connection")
		}
		if conn != nil {
			conn.Close()
		}
		mychan <- 1
	}(conn)
	if err != nil {
		log.Println("Could not open connection", err.Error())
		return
	}

	data := make([]byte, 512)
	if debug {
		log.Println("Reading 1 bytes")
	}
	_, err = conn.Read(data[0:1])
	if err != nil && err != io.EOF {
		log.Println("Error on connection read", err.Error())
		return
	}
}

var debug bool

var memstats = runtime.MemStats{}

func printMemStats() {
	runtime.ReadMemStats(&memstats)
	log.Printf("Total: %dM InUse: %dM", memstats.TotalAlloc/1024, memstats.Alloc/1024)
	printMemStatsTimer = time.AfterFunc(1*time.Second, printMemStats)
}

var printMemStatsTimer *time.Timer

func main() {
	var maxConnection int

	flag.IntVar(&maxConnection, "n", 1000, "Number of connections to make")
	flag.BoolVar(&debug, "d", false, "debug mode")

	flag.Parse()

	log.Printf("Running with %d connections.", maxConnection)
	printMemStats()
	mychan := make(chan int, maxConnection)
	for i := 0; i < maxConnection; i++ {
		go makeConnection(mychan)
	}
	for i := 0; i < maxConnection; i++ {
		if debug {
			log.Println("Reaping connection goroutine")
		}
		<-mychan
	}
	defer func() {
		log.Printf("Writing memory profile")
		f, err := os.Create("/tmp/memprofile.pprof")
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
	}()
}
