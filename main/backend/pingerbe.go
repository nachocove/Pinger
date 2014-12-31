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
)

var debug bool

var usage = func() {
	fmt.Printf("USAGE: %s <flags> <connection string>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo(stats *Pinger.MemStats) string {
	if Pinger.ActiveClientCount > 0 {
		var allocM = int((stats.Memstats.Alloc - stats.Basememstats.Alloc) / 1024)
		return fmt.Sprintf("number of connections: %d (mem/conn %dM)", Pinger.ActiveClientCount, allocM/Pinger.ActiveClientCount)
	} else {
		return fmt.Sprintf("number of connections: %d", Pinger.ActiveClientCount)
	}
}

func main() {
	var printMemPeriodic int
	var pingPeriodic int
	var maxConnection int
	var printMem bool
	var help bool
	var connectionString string

	flag.IntVar(&maxConnection, "n", 1000, "Number of connections to make")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&help, "h", false, "Verbose")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
	flag.IntVar(&pingPeriodic, "ping", 0, "ping server")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}

	if len(flag.Args()) != 1 {
		usage()
		os.Exit(1)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Printf("Running with %d connections. (Processors: %d)", maxConnection, runtime.NumCPU())

	var memstats *Pinger.MemStats
	if printMemPeriodic > 0 || printMem {
		memstats = Pinger.NewMemStats(memStatsExtraInfo, debug, false)
		if printMemPeriodic > 0 {
			memstats.PrintMemStatsPeriodic(printMemPeriodic)
		}
		if printMem && printMemPeriodic <= 0 {
			memstats.PrintMemStats()
		}
	}

	connectionString = flag.Arg(0)
	var wg sync.WaitGroup

	for i := 0; i < maxConnection; i++ {
		if debug {
			log.Println("Opening connection to", connectionString)
		}
		client := Pinger.NewExchangeClient(connectionString, pingPeriodic, debug)
		// this launches either 2 or 3 goroutines per connection. 3 if pingPeriodic > 0, 2 otherwise.
		if client != nil {
			err := client.Listen(&wg)
			if err != nil {
				log.Println("Could not open connection", i)
			}
		}
	}
	wg.Wait()
	defer func() {
		log.Printf("All Connections closed: ")
		if memstats != nil {
			memstats.PrintMemStats()
		}
		profileFile := "/tmp/memprofile.pprof"
		log.Printf("Writing memory profile: %s\n", profileFile)
		f, err := os.Create(profileFile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
	}()
}
