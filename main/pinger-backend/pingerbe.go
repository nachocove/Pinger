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
	var printMem bool
	var help bool
	var verbose bool
	var configFile string

	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&printMem, "m", false, "print memory mode")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
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

	config, err := Pinger.ReadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Reading aws config: %s", err)
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
	logger, err = config.Global.InitLogging(screenLogging, screenLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Init Logging: %s", err)
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
