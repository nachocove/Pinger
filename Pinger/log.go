package Pinger

import (
	"github.com/op/go-logging"
	"os"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func InitLogging(loggerName string, logFileName string, fileLevel logging.Level, screen bool, screenLevel logging.Level) (*logging.Logger, error) {
	logFile, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	format := logging.MustStringFormatter("%{time:15:04:05.000} %{level} %{shortfunc}:%{message}")
	fileLogger := logging.AddModuleLevel(logging.NewLogBackend(logFile, "", 0))
	fileLogger.SetLevel(fileLevel, "")
	if screen {
		screenLogger := logging.AddModuleLevel(logging.NewLogBackend(os.Stdout, "", 0))
		screenLogger.SetLevel(screenLevel, "")
		logging.SetBackend(fileLogger, screenLogger)
	} else {
		logging.SetBackend(fileLogger)
	}
	logging.SetFormatter(format)
	return GetLogger(loggerName), nil
}

func GetLogger(loggerName string) *logging.Logger {
	return logging.MustGetLogger(loggerName)
}
