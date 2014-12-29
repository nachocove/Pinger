package main

import (
	"flag"
	"github.com/nachocove/Pinger/Pinger"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

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

	runtime.GOMAXPROCS(runtime.NumCPU())

	log.Printf("Running with %d connections. (Processors: %d)", maxConnection, runtime.NumCPU())
	printMemStats()

	dialString := "localhost:8082"
	var wg sync.WaitGroup

	for i := 0; i < maxConnection; i++ {
		if debug {
			log.Println("Opening connection to", dialString)
		}
		_ = Pinger.NewExchangeClient(dialString, &wg, debug)
	}
	wg.Wait()
	defer func() {
		profileFile := "/tmp/memprofile.pprof"
		log.Printf("Writing memory profile: %s\n", profileFile)
		f, err := os.Create(profileFile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
	}()
}
