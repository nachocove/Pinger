package Pinger

import (
	"log"
	"runtime"
	"time"
)

// MemStats various print memory statistics related data and functions
type MemStats struct {
	memstats           runtime.MemStats
	sleepTime          int
	extraInfo          func() string
	printMemStatsTimer *time.Timer
}

// NewMemStats create a new MemStats structure
func NewMemStats(extraInfo func() string) *MemStats {
	stats := MemStats{
		sleepTime: 0,
		extraInfo: extraInfo,
		printMemStatsTimer: nil,
	}
	return &stats
}

// PrintMemStats print memory statistics once.
func (stats *MemStats) PrintMemStats() {
	runtime.ReadMemStats(&stats.memstats)
	extra := stats.extraInfo()
	log.Printf("%s Memory: %dM InUse: %dM\n", extra, stats.memstats.TotalAlloc/1024, stats.memstats.Alloc/1024)
}

// PrintMemStatsPeriodic print memory statistics periodically, starting now.
func (stats *MemStats) PrintMemStatsPeriodic(sleepTime int) {
	stats.sleepTime = sleepTime
	stats.printMemStatsAndRestartTimer()
}

func (stats *MemStats) printMemStatsAndRestartTimer() {
	stats.PrintMemStats()
	stats.printMemStatsTimer = time.AfterFunc(time.Duration(stats.sleepTime)*time.Second, stats.printMemStatsAndRestartTimer)
}
