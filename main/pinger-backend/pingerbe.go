package main

import (
	"flag"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"os"
	"path"
	"runtime"
)

var usage = func() {
	fmt.Printf("USAGE: %s <flags> <connection string>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo(stats *Utils.MemStats) string {
	k := float64(1024.0)
	if Utils.ActiveClientCount > 0 {
		allocM := float64(int64(stats.Memstats.Alloc)-int64(stats.Basememstats.Alloc)) / k
		return fmt.Sprintf("number of connections: %d  (est. mem/conn %fk)", Utils.ActiveClientCount, allocM/float64(Utils.ActiveClientCount))
	}
	return fmt.Sprintf("number of connections: %d", Utils.ActiveClientCount)
}

var logger *Logging.Logger

func main() {
	var printMemPeriodic int
	var printMem bool
	var help bool
	var debug bool
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
		fmt.Fprintf(os.Stderr, "Reading pinger config: %s\n", err)
		os.Exit(1)
	}
	var screenLogging = false
	var screenLevel = Logging.ERROR
	if debug || verbose {
		screenLogging = true
		if debug {
			screenLevel = Logging.DEBUG
		} else {
			screenLevel = Logging.INFO
		}
	}

	// From here on, treat the cfg debug and cli debug the same.
	// Don't do this before we decide on the screen output above
	debug = debug || config.Backend.Debug
	telemetryWriter, err := Telemetry.NewTelemetryWriter(&config.Telemetry, config.Aws.NewHandle(), debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Init Telemetry: %s\n", err)
		os.Exit(1)
	}

	logger, err = config.Logging.InitLogging(screenLogging, screenLevel, telemetryWriter, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Init Logging: %s\n", err)
		os.Exit(1)
	}

	Utils.InitCpuProfileSignal()

	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Debug("Running with %d Processors", runtime.NumCPU())

	var memstats *Utils.MemStats
	if printMemPeriodic > 0 || printMem {
		memstats = Utils.NewMemStats(memStatsExtraInfo, debug, false)
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

	logger.Info("Started %v", os.Args)
	err = Pinger.StartPollingRPCServer(config, debug, logger)
	if err != nil {
		panic(err.Error())
	}
}
