package Logging

import (
	"fmt"
	"github.com/op/go-logging"
	"os"
)

type Level logging.Level

func LogLevel(level string) (Level, error) {
	levelInt, err := logging.LogLevel(level)
	return Level(levelInt), err
}

const (
	ERROR Level = Level(logging.ERROR)
	DEBUG Level = Level(logging.DEBUG)
	INFO  Level = Level(logging.INFO)
)

type Logger struct {
	logger *logging.Logger
}

func (log *Logger) Info(format string, args ...interface{}) {
	log.logger.Info(format, args...)
}

func (log *Logger) Warning(format string, args ...interface{}) {
	log.logger.Warning(format, args...)
}

func (log *Logger) Error(format string, args ...interface{}) {
	log.logger.Error(format, args...)
}

func (log *Logger) Debug(format string, args ...interface{}) {
	log.logger.Debug(format, args...)
}

func (log *Logger) Fatalf(format string, args ...interface{}) {
	log.logger.Fatalf(format, args...)
}

func (log *Logger) Fatal(args ...interface{}) {
	log.logger.Fatal(args...)
}

func (log *Logger) formatLogString(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// TODO: Can we cache and do a more python-logging type thing where we have a global array of loggers and can
// just fetch them at any time, rather than passing around the logger everywhere?

// TODO: Add telemetry pushing to logging

const (
	debugFormatStr  = "%{time:2006-01-02T15:04:05.000} %{level} %{shortfile}:%{shortfunc} %{message}"
	normalFormatStr = "%{time:2006-01-02T15:04:05.000} %{level} %{shortfunc} %{message}"
)

var loggerCache map[string]*Logger

func init() {
	loggerCache = make(map[string]*Logger)
}
func InitLogging(loggerName string, logFileName string, fileLevel Level, screen bool, screenLevel Level, debug bool) *Logger {
	_, ok := loggerCache[loggerName]
	if !ok {
		var logFile *os.File
		var err error

		if logFileName != "" {
			logFile, err = os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				panic(fmt.Sprintf("Could not open file %s for logging: %s", logFileName, err.Error()))
			}
		} else {
			logFile = nil
		}

		var formatStr string
		if debug {
			formatStr = debugFormatStr
		} else {
			formatStr = normalFormatStr
		}
		format := logging.MustStringFormatter(formatStr)
		fileLogger := logging.AddModuleLevel(logging.NewLogBackend(logFile, "", 0))
		fileLogger.SetLevel(logging.Level(fileLevel), "")
		if screen {
			screenLogger := logging.AddModuleLevel(logging.NewLogBackend(os.Stdout, "", 0))
			screenLogger.SetLevel(logging.Level(screenLevel), "")
			logging.SetBackend(fileLogger, screenLogger)
		} else {
			logging.SetBackend(fileLogger)
		}
		logging.SetFormatter(format)
		logger := Logger{}
		logger.logger = logging.MustGetLogger(loggerName)
		logger.logger.ExtraCalldepth = 1
		loggerCache[loggerName] = &logger
	}
	if loggerCache[loggerName] == nil {
		panic("Could not get init logger")
	}
	return loggerCache[loggerName]
}

//func GetLogger(loggerName string) *Logger {
//	_, ok := loggerCache[loggerName]
//	if !ok {
//		logger := Logger{}
//		logger.logger = logging.MustGetLogger(loggerName)
//		loggerCache[loggerName] = &logger
//	}
//	return loggerCache[loggerName]
//}

func ToggleLogging(logger *Logger, previousLevel Level) Level {
	currentLevel := logging.GetLevel(logger.logger.Module)
	switch {
	case previousLevel < 0:
		if currentLevel != logging.DEBUG {
			logging.SetLevel(logging.DEBUG, logger.logger.Module)
			logger.Warning("Logger-%s: Setting logging to DEBUG\n", logger.logger.Module)
			return Level(currentLevel)
		} else {
			return -1
		}

	case previousLevel != Level(currentLevel):
		logging.SetLevel(logging.Level(previousLevel), logger.logger.Module)
		logger.Warning("Logger-%s: Resetting logging to %d\n", logger.logger.Module, previousLevel)
		return -1
	}
	return -1
}
