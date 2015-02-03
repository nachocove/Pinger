package Pinger

import (
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLevelNameToLevel(t *testing.T) {
	assert := assert.New(t)

	value, err := LevelNameToLevel("DEBUG")
	assert.Nil(err, "Err should be nil")
	assert.Equal(value, logging.DEBUG, "DEBUG != logging.DEBUG")

	value, err = LevelNameToLevel("WARNING")
	assert.Nil(err, "Err should be nil")
	assert.Equal(value, logging.WARNING, "WARNING != logging.WARNING")

	value, err = LevelNameToLevel("ERROR")
	assert.Nil(err, "Err should be nil")
	assert.Equal(value, logging.ERROR, "ERROR != logging.ERROR")

	value, err = LevelNameToLevel("INFO")
	assert.Nil(err, "Err should be nil")
	assert.Equal(value, logging.INFO, "INFO != logging.INFO")

	value, err = LevelNameToLevel("CRITICAL")
	assert.Nil(err, "Err should be nil")
	assert.Equal(value, logging.CRITICAL, "CRITICAL != logging.CRITICAL")

	value, err = LevelNameToLevel("NOTICE")
	assert.Nil(err, "Err should be nil")
	assert.Equal(value, logging.NOTICE, "NOTICE != logging.NOTICE")

	value, err = LevelNameToLevel("")
	assert.NotNil(err, "Err should not be nil")
	assert.Equal(value, 0, "return value should be 0")
}
