package Utils

import (
	"runtime"
	"time"
	"fmt"
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

// SetBaseMemStats set the base state for memory calculations to the current state of things
func (stats *MemStats) SetBaseMemStats() {
	runtime.ReadMemStats(&stats.Basememstats)
}

var meg = float64(1024.0 * 1024.0)

// PrintIncrementalMemStats print memory statistics once.
func (stats *MemStats) PrintIncrementalMemStats() {
	runtime.ReadMemStats(&stats.Memstats)
	extra := stats.extraInfo(stats)
	incrTotalAlloc := int64(stats.Memstats.TotalAlloc) - int64(stats.Basememstats.TotalAlloc)
	incrAlloc := int64(stats.Memstats.Alloc) - int64(stats.Basememstats.Alloc)
	fmt.Printf("Memory: %.2fM InUse: %.2fM IncrMemory: %.2fM IncrInUse: %.2fM %s\n",
		float64(stats.Memstats.TotalAlloc)/meg,
		float64(stats.Memstats.Alloc)/meg,
		float64(incrTotalAlloc)/meg,
		float64(incrAlloc)/meg,
		extra)
}

// PrintMemStats print memory statistics once.
func (stats *MemStats) PrintMemStats() {
	runtime.ReadMemStats(&stats.Memstats)
	extra := stats.extraInfo(stats)
	fmt.Printf("Memory: %.2fM InUse: %.2fM %s\n", float64(stats.Memstats.TotalAlloc)/meg, float64(stats.Memstats.Alloc)/meg, extra)
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
	if stats.printMemStatsTimer != nil {
		stats.printMemStatsTimer.Stop()
	}
	stats.printMemStatsTimer = time.AfterFunc(time.Duration(stats.sleepTime)*time.Second, stats.printMemStatsAndRestartTimer)
}
