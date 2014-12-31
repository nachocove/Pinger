package Pinger

import (
	"log"
	"runtime"
	"time"
)

// MemStats various print memory statistics related data and functions
type MemStats struct {
	// Memstats current memory statistics
	Memstats runtime.MemStats
	// Basememstats initial memory statistics
	Basememstats runtime.MemStats

	sleepTime          int
	extraInfo          func(*MemStats) string
	printMemStatsTimer *time.Timer
	debug              bool
	printIncremental   bool
}

// NewMemStats create a new MemStats structure
func NewMemStats(extraInfo func(*MemStats) string, debug, printIncremental bool) *MemStats {
	stats := MemStats{
		sleepTime:          0,
		extraInfo:          extraInfo,
		printMemStatsTimer: nil,
		debug:              debug,
		printIncremental:   printIncremental,
	}
	stats.SetBaseMemStats()
	return &stats
}

func (stats *MemStats) SetBaseMemStats() {
	runtime.ReadMemStats(&stats.Basememstats)
}

// PrintIncrementalMemStats print memory statistics once.
func (stats *MemStats) PrintIncrementalMemStats() {
	runtime.ReadMemStats(&stats.Memstats)
	extra := stats.extraInfo(stats)
	incrTotalAlloc := stats.Memstats.TotalAlloc - stats.Basememstats.TotalAlloc
	incrAlloc := int(stats.Memstats.Alloc) - int(stats.Basememstats.Alloc)
	if stats.debug {
		log.Printf("incrTotalAlloc %d, incrAlloc %d (%d-%d)\n", incrTotalAlloc, incrAlloc, stats.Memstats.Alloc, stats.Basememstats.Alloc)
	}
	log.Printf("Memory: %dM InUse: %dM IncrMemory: %dM IncrInUse: %dM %s\n",
		stats.Memstats.TotalAlloc/1024,
		stats.Memstats.Alloc/1024,
		incrTotalAlloc/1024,
		incrAlloc/1024,
		extra)
}

// PrintMemStats print memory statistics once.
func (stats *MemStats) PrintMemStats() {
	runtime.ReadMemStats(&stats.Memstats)
	extra := stats.extraInfo(stats)
	log.Printf("Memory: %dM InUse: %dM %s\n", stats.Memstats.TotalAlloc/1024, stats.Memstats.Alloc/1024, extra)
}

// PrintMemStatsPeriodic print memory statistics periodically, starting now.
func (stats *MemStats) PrintMemStatsPeriodic(sleepTime int) {
	stats.sleepTime = sleepTime
	stats.printMemStatsAndRestartTimer()
}

func (stats *MemStats) printMemStatsAndRestartTimer() {
	runtime.GC()
	if stats.printIncremental {
		stats.PrintIncrementalMemStats()
	} else {
		stats.PrintMemStats()
	}
	stats.printMemStatsTimer = time.AfterFunc(time.Duration(stats.sleepTime)*time.Second, stats.printMemStatsAndRestartTimer)
}
