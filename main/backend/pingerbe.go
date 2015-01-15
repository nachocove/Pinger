package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/op/go-logging"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var debug bool

var usage = func() {
	fmt.Printf("USAGE: %s <flags> <connection string>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo(stats *Pinger.MemStats) string {
	k := float64(1024.0)
	if Pinger.ActiveClientCount > 0 {
		allocM := float64(int64(stats.Memstats.Alloc)-int64(stats.Basememstats.Alloc)) / k
		return fmt.Sprintf("number of connections: %d  (est. mem/conn %fk)", Pinger.ActiveClientCount, allocM/float64(Pinger.ActiveClientCount))
	}
	return fmt.Sprintf("number of connections: %d", Pinger.ActiveClientCount)
}

var logger *logging.Logger

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
	var tcpKeepAlive int
	var verbose bool
	var sleepBetweenOpens int
	var logFileLevel string
	var logFileName string

	flag.IntVar(&maxConnection, "n", 1000, "Number of connections to make")
	flag.IntVar(&tcpKeepAlive, "tcpkeepalive", 0, "TCP Keepalive in seconds")
	flag.StringVar(&logFileName, "log-file", "pinger-backend.log", "log-file to log to")
	flag.StringVar(&logFileLevel, "log-level", "WARNING", "Logging level for the logfile (DEBUG, INFO, WARN, NOTICE, ERROR, CRITICAL)")
	flag.IntVar(&sleepBetweenOpens, "sleep-after-open", 0, "Sleep n milliseconds after each connection opened.")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Verbose")
	flag.BoolVar(&tlsCheckHostname, "tlscheckhost", false, "Verify the hostname to the certificate")
	flag.BoolVar(&noReopenConnections, "no-reopen", false, "Verbose")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
	flag.IntVar(&pingPeriodic, "ping", 0, "ping server in seconds (plus fudge factor)")
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

	var TLSConfig *tls.Config
	if caCertChainFile != "" {
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
		TLSConfig = &tls.Config{RootCAs: pool}
		if tlsCheckHostname {
			TLSConfig.InsecureSkipVerify = false
		} else {
			TLSConfig.InsecureSkipVerify = true
		}
	}

	if logFileName == "" {
		logFileName = "/dev/null"
	}
	logFile, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	var screenLogging = false
	var screenLevel = logging.ERROR
	if debug || verbose{
		screenLogging = true
		if debug {
			screenLevel = logging.DEBUG
		} else {
			screenLevel = logging.INFO
		}
	}
	var fileLevel logging.Level
	switch {
	case logFileLevel == "WARNING":
		fileLevel = logging.WARNING
	case logFileLevel == "ERROR":
		fileLevel = logging.ERROR
	case logFileLevel == "DEBUG":
		fileLevel = logging.DEBUG
	case logFileLevel == "INFO":
		fileLevel = logging.INFO
	case logFileLevel == "CRITICAL":
		fileLevel = logging.CRITICAL
	case logFileLevel == "NOTICE":
		fileLevel = logging.NOTICE
	}
		
	logger = Pinger.InitLogging("pinger-be", logFile, fileLevel, screenLogging, screenLevel)
	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Info("Running with %d connections. (Processors: %d)", maxConnection, runtime.NumCPU())

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

	if memstats != nil {
		memstats.SetBaseMemStats()
	}
	go func() {
		logger.Error("%v\n", http.ListenAndServe("localhost:6060", nil))
	}()
	for i := 0; i < maxConnection; i++ {
		if debug {
			logger.Info("Opening connection to %s", connectionString)
		}
		var reopen bool
		if noReopenConnections {
			reopen = false
		} else {
			reopen = true
		}
		client := Pinger.NewExchangeClient(connectionString, pingPeriodic, reopen, TLSConfig, tcpKeepAlive, debug, logger)
		// this launches either 2 or 3 goroutines per connection. 3 if pingPeriodic > 0, 2 otherwise.
		if client != nil {
			err := client.Listen(&wg)
			if err != nil {
				logger.Error("Could not open connection %d %v\n", i, err.Error())
			}
		}
		if sleepBetweenOpens > 0 {
			time.Sleep(time.Duration(sleepBetweenOpens)*time.Millisecond)
		}
	}
	wg.Wait()
	defer func() {
		logger.Info("All Connections closed: ")
		if memstats != nil {
			memstats.PrintMemStats()
		}
		profileFile := "/tmp/memprofile.pprof"
		logger.Info("Writing memory profile: %s\n", profileFile)
		f, err := os.Create(profileFile)
		if err != nil {
			logger.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
	}()
}
