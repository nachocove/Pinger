package Pinger

import (
	"github.com/op/go-logging"
	"io"
	"os"
)

func InitLogging(loggerName string, logFile io.Writer, fileLevel logging.Level, screen bool, screenLevel logging.Level) *logging.Logger {
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
	return GetLogger(loggerName)
}

func GetLogger(loggerName string) *logging.Logger {
	return logging.MustGetLogger(loggerName)
}
