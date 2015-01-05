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
	"io/ioutil"
	"crypto/x509"
	"crypto/tls"
)

var debug bool

var usage = func() {
	fmt.Printf("USAGE: %s <flags> <connection string>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo(stats *Pinger.MemStats) string {
	k := float64(1024.0)
	if Pinger.ActiveClientCount > 0 {
		allocM := (float64(stats.Memstats.Alloc) - float64(stats.Basememstats.Alloc)) / k
		return fmt.Sprintf("number of connections: %d (est. mem/conn %fk)", Pinger.ActiveClientCount, allocM/float64(Pinger.ActiveClientCount))
	} else {
		return fmt.Sprintf("number of connections: %d", Pinger.ActiveClientCount)
	}
}

func main() {
	var printMemPeriodic int
	var pingPeriodic int
	var maxConnection int
	var printMem bool
	var tlsCheckHostname bool
	var help bool
	var connectionString string
	var noReopenConnections bool
	var caCertChainFile string

	flag.IntVar(&maxConnection, "n", 1000, "Number of connections to make")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&help, "h", false, "Verbose")
	flag.BoolVar(&tlsCheckHostname, "tlscheckhost", false, "Verify the hostname to the certificate")
	flag.BoolVar(&noReopenConnections, "no-reopen", false, "Verbose")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
	flag.IntVar(&pingPeriodic, "ping", 0, "ping server")
	flag.StringVar(&caCertChainFile, "cachain", "", "File containing one or more ca certs")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}

	if len(flag.Args()) != 1 {
		usage()
		os.Exit(1)
	}

	if caCertChainFile == "" {
		fmt.Fprintln(os.Stderr, "Need a ca cert chain file")
		os.Exit(1)
	}
	caCertChain, err := ioutil.ReadFile(caCertChainFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Open %s: %v\n", caCertChainFile, err)
		os.Exit(1)
	}
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(caCertChain)
	if !ok {
		fmt.Fprintf(os.Stderr, "Could not parse certfile %s\n", caCertChainFile)
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
		var reopen bool
		if noReopenConnections {
			reopen = false
		} else {
			reopen = true
		}
		config := &tls.Config{RootCAs: pool,}
		if tlsCheckHostname {
			config.InsecureSkipVerify = false
		} else {
			config.InsecureSkipVerify = true
		}
		client := Pinger.NewExchangeClient(connectionString, pingPeriodic, reopen, config, debug)
		// this launches either 2 or 3 goroutines per connection. 3 if pingPeriodic > 0, 2 otherwise.
		if client != nil {
			err := client.Listen(&wg)
			if err != nil {
				log.Println("Could not open connection", i, err.Error())
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
