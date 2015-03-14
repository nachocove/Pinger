package Utils

import (
	logging "github.com/nachocove/Pinger/Pinger/logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStats(t *testing.T) {
	assert := assert.New(t)
	logger := logging.InitLogging("unittest", "", logging.DEBUG, true, logging.DEBUG, true)
	assert.NotNil(logger)
	stopCh := make(chan int)
	statLogger := NewStatLogger(stopCh, logger, false)
	defer func() {
		close(stopCh)
	}()
	assert.NotNil(statLogger, "statLogger should not be nil")

	stat := newStats()
	assert.NotNil(stat, "stat should not be nil")
	assert.True(stat.min > 0, "initial value of min should be very large, not 0")
	assert.Equal(stat.max, 0, "initial value of max should be 0!")
	assert.Equal(stat.avg, 0, "initial value of avg should be 0!")
	assert.Equal(stat.count, 0, "initial value of count should be 0!")
	assert.Equal(stat.sum, 0, "initial value of sum should be 0!")

	stat.addDataPoint(1.0)
	assert.Equal(stat.count, 1, "Should only have one data point")
	assert.Equal(stat.max, 1.0, "Max should be 1.0")
	assert.Equal(stat.min, 1.0, "Min should be 1.0")
	assert.Equal(stat.sum, 1.0, "Sum should be 1.0")
	assert.Equal(stat.avg, 0.0, "Avg should be 0.0") // avg is computed only when we log or call Avg() for efficiency

	avg := stat.Avg()
	assert.Equal(stat.avg, 1.0, "Average should be 1.0")
	assert.Equal(stat.avg, avg, "Average should be the returned value!")
}
