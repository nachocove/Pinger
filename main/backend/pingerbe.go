package main

import (
	"flag"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var debug bool

var memstats = runtime.MemStats{}

func printMemStats() {
	runtime.ReadMemStats(&memstats)
	fmt.Printf("Memory: %dM InUse: %dM\n", memstats.TotalAlloc/1024, memstats.Alloc/1024)
}
func printMemStatsPeriodic(n int) {
	printMemStatsTimer = time.AfterFunc(time.Duration(n)*time.Second, printMemStats)
}

var printMemStatsTimer *time.Timer
var usage = func() {
	fmt.Fprintf(os.Stderr, "USAGE: %s <flags> <connection string>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func main() {
	var printMemPeriodic int
	var maxConnection int
	var printMem bool
	var help bool
	var connectionString string
	
	flag.IntVar(&maxConnection, "n", 1000, "Number of connections to make")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&help, "h", false, "Verbose")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "p", 0, "print memory periodically mode in seconds")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	log.Printf("Running with %d connections. (Processors: %d)", maxConnection, runtime.NumCPU())
	if printMemPeriodic > 0 {
		printMemStatsPeriodic(printMemPeriodic)
	}
	if printMem {
		fmt.Printf("With 0 connections: ")
		printMemStats()
	}

	fmt.Println(flag.Arg(0))
	if len(flag.Args()) != 1 {
		usage()
		os.Exit(1)
	}
	connectionString = flag.Arg(0)
	var wg sync.WaitGroup

	for i := 0; i < maxConnection; i++ {
		if debug {
			log.Println("Opening connection to", connectionString)
		}
		client := Pinger.NewExchangeClient(connectionString, debug)
		fmt.Println(client)
		client.Listen(&wg)
	}
	wg.Wait()
	defer func() {
		fmt.Printf("All Connections closed: ")
		printMemStats()
		profileFile := "/tmp/memprofile.pprof"
		log.Printf("Writing memory profile: %s\n", profileFile)
		f, err := os.Create(profileFile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
	}()
}
