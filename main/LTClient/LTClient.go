package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
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

var usage = func() {
	fmt.Printf("USAGE: %s <flags> <pinger-URL>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo(stats *Utils.MemStats) string {
	k := float64(1024.0)
	if Utils.ActiveClientCount > 0 {
		allocM := float64(int64(stats.Memstats.Alloc)-int64(stats.Basememstats.Alloc)) / k
		return fmt.Sprintf("Number of connections: %d  (est. mem/conn %fk)", Utils.ActiveClientCount, allocM/float64(Utils.ActiveClientCount))
	}
	return fmt.Sprintf("Number of connections: %d", Utils.ActiveClientCount)
}

var logger *Logging.Logger

func main() {
	var printMemPeriodic int
	var maxUsers int
	var averageAccountCount int
	var averageDeferCount int
	var testDuration int
	var printMem bool
	var tlsCheckHostname bool
	var help bool
	var pingerURL string
	var noReopenConnections bool
	var caCertChainFile string
	var tcpKeepAlive int
	var verbose bool
	var sleepBetweenOpens int
	var logFileLevel string
	var logFileName string

	flag.IntVar(&maxUsers, "users", 10, "Number of users to simulate")
	flag.IntVar(&averageAccountCount, "accounts", 5, "Average number of accounts per user")
	flag.IntVar(&averageDeferCount, "defers", 5, "Average number of defers per account")
	flag.IntVar(&testDuration, "duration", 10, "Test Duration in minutes")
	flag.IntVar(&tcpKeepAlive, "tcpkeepalive", 0, "TCP Keepalive in seconds")
	flag.StringVar(&logFileName, "log-file", "LTClient.log", "log-file to log to")
	flag.StringVar(&logFileLevel, "log-level", "DEBUG", "Logging level for the logfile (DEBUG, INFO, WARN, NOTICE, ERROR, CRITICAL)")
	flag.IntVar(&sleepBetweenOpens, "sleep-after-open", 0, "Sleep n milliseconds after each connection opened.")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&tlsCheckHostname, "checkhost", false, "Verify the hostname to the certificate")
	flag.BoolVar(&noReopenConnections, "no-reopen", false, "No Reopen Connections")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
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
	var screenLogging = false
	var screenLevel = Logging.ERROR
	if verbose {
		screenLogging = true
		screenLevel = Logging.DEBUG
	}
	fileLevel, err := Logging.LogLevel(logFileLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LevelNameToLevel: %v\n", err)
		os.Exit(1)
	}
	logger = Logging.InitLogging("LTClient", logFileName, fileLevel, screenLogging, screenLevel, nil, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "InitLogging: %v\n", err)
		os.Exit(1)
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Info("Running with %d users (%d accounts). (Processors: %d)", maxUsers, averageAccountCount, runtime.NumCPU())
	var memstats *Utils.MemStats
	if printMemPeriodic > 0 || printMem {
		memstats = Utils.NewMemStats(memStatsExtraInfo, true, false)
		if printMemPeriodic > 0 {
			memstats.PrintMemStatsPeriodic(printMemPeriodic)
		}
		if printMem && printMemPeriodic <= 0 {
			memstats.PrintMemStats()
		}
	}

	pingerURL = flag.Arg(0)
	var wg sync.WaitGroup

	if memstats != nil {
		memstats.SetBaseMemStats()
	}
	go func() {
		logger.Error("%v\n", http.ListenAndServe("localhost:6060", nil))
	}()

	stopAllCh := make(chan int)

	for i := 0; i < maxUsers; i++ {
		var reopen bool
		if noReopenConnections {
			reopen = false
		} else {
			reopen = true
		}

		ltUser := NewLTUser(pingerURL, reopen, tcpKeepAlive, TLSConfig,
			averageAccountCount, averageDeferCount, testDuration, stopAllCh, &wg, logger)
		logger.Info("Setting up user (%d) %s", i, ltUser.userName)
		if ltUser != nil {
			err := ltUser.StartUserSimulation()
			if err != nil {
				logger.Error("Could not create User %d %v\n", i, err.Error())
				i-- // don't count this one
			}
		}
		if sleepBetweenOpens > 0 {
			time.Sleep(time.Duration(sleepBetweenOpens) * time.Millisecond)
		}
	}
	logger.Info("All simulations started. Waiting...")
	wg.Wait()
	defer func() {
		logger.Info("All Connections closed.")
		if memstats != nil {
			memstats.PrintMemStats()
		}
		profileFile := "/tmp/memprofile.pprof"
		logger.Info("Writing memory profile: %s\n", profileFile)
		f, err := os.Create(profileFile)
		if err != nil {
			logger.Fatalf(err.Error())
		}
		pprof.WriteHeapProfile(f)
	}()
}
