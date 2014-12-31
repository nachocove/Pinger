package Pinger

import (
	"log"
	"runtime"
	"time"
)

type MemStats struct {
	memstats           runtime.MemStats
	sleepTime          int
	extraInfo          func() string
	printMemStatsTimer *time.Timer
}

func NewMemStats(sleepTime int, extraInfo func() string) *MemStats {
	stats := MemStats{sleepTime: sleepTime, extraInfo: extraInfo}
	return &stats
}
func (stats *MemStats) PrintMemStats() {
	runtime.ReadMemStats(&stats.memstats)
	extra := stats.extraInfo()
	log.Printf("%s Memory: %dM InUse: %dM\n", extra, stats.memstats.TotalAlloc/1024, stats.memstats.Alloc/1024)
}
func (stats *MemStats) PrintMemStatsPeriodic() {
	stats.printMemStatsAndRestartTimer()
}

func (stats *MemStats) printMemStatsAndRestartTimer() {
	stats.PrintMemStats()
	stats.printMemStatsTimer = time.AfterFunc(time.Duration(stats.sleepTime)*time.Second, stats.printMemStatsAndRestartTimer)
}
