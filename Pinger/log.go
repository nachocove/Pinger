package Pinger

import (
	"github.com/op/go-logging"
	"os"
)

// TODO Need to abstract out the logging stuff and make it a pinger logger. That way we can change the underlying logging later.
// Also, it will allow us to cache and do a more python-logging type thing where we have a global array of loggers and can
// just fetch them at any time, rather than passing around the logger everywhere.

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

var LoggerName string

func InitLogging(loggerName string, logFileName string, fileLevel logging.Level, screen bool, screenLevel logging.Level, debug bool) (*logging.Logger, error) {
	if LoggerName != "" {
		panic("Can not init logging multiple times")
	}
	logFile, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	var formatStr string
	if debug {
		formatStr = "%{time:2006-01-02T15:04:05.000} %{level} %{shortfile}:%{shortfunc} %{message}"
	} else {
		formatStr = "%{time:2006-01-02T15:04:05.000} %{level} %{shortfunc} %{message}"		
	}
	format := logging.MustStringFormatter(formatStr)		
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
	logger := GetLogger(loggerName)
	LoggerName = loggerName
	return logger, nil
}

func GetLogger(loggerName string) *logging.Logger {
	return logging.MustGetLogger(loggerName)
}

func ToggleLogging(logger *logging.Logger, previousLevel logging.Level) logging.Level {
	currentLevel := logging.GetLevel(logger.Module)
	switch {
	case previousLevel < 0:
		if currentLevel != logging.DEBUG {
			logging.SetLevel(logging.DEBUG, logger.Module)
			logger.Warning("Logger-%s: Setting logging to DEBUG\n", logger.Module)
			return currentLevel
		} else {
			return -1
		}

	case previousLevel != currentLevel:
		logging.SetLevel(previousLevel, logger.Module)
		logger.Warning("Logger-%s: Resetting logging to %d\n", logger.Module, previousLevel)
		return -1
	}
	return -1
}
