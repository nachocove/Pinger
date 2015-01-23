package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/nachocove/Pinger/Pinger"
	"github.com/op/go-logging"
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
	var printMem bool
	var help bool
	var noReopenConnections bool
	var verbose bool
	var logFileLevel string
	var logFileName string
	var configFile string

	flag.StringVar(&logFileName, "log-file", "pinger-backend.log", "log-file to log to")
	flag.StringVar(&logFileLevel, "log-level", "WARNING", "Logging level for the logfile (DEBUG, INFO, WARN, NOTICE, ERROR, CRITICAL)")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&noReopenConnections, "no-reopen", false, "Verbose")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
	flag.IntVar(&pingPeriodic, "ping", 0, "ping server in seconds (plus fudge factor)")
	flag.StringVar(&configFile, "c", "", "The configuration file. Required.")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}

	if configFile == "" {
		usage()
		os.Exit(1)
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
	if debug || verbose {
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

	config, err := Pinger.ReadConfig(configFile)
	if err != nil {
		logger.Error("Reading aws config: %s", err)
		os.Exit(1)
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Info("Running with %d Processors", runtime.NumCPU())

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

	if memstats != nil {
		memstats.SetBaseMemStats()
	}
	
	Pinger.StartPollingRPCServer(config, debug, logger) // will also include the pprof server
}
