package Logging

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"github.com/op/go-logging"
	"os"
)

type Level logging.Level

func LogLevel(level string) (Level, error) {
	levelInt, err := logging.LogLevel(level)
	return Level(levelInt), err
}

const (
	ERROR    Level = Level(logging.ERROR)
	WARNING  Level = Level(logging.WARNING)
	INFO     Level = Level(logging.INFO)
	DEBUG    Level = Level(logging.DEBUG)
	CRITICAL Level = Level(logging.CRITICAL)
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
func InitLogging(loggerName string, logFileName string, fileLevel Level, screen bool, screenLevel Level, telemetryWriter *Telemetry.TelemetryWriter, debug bool) *Logger {
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

		var fileLogger logging.LeveledBackend
		var screenLogger logging.LeveledBackend
		var loggers = make([]logging.Backend, 0, 3)
		format := logging.MustStringFormatter(formatStr)
		fileLogger = logging.AddModuleLevel(logging.NewLogBackend(logFile, "", 0))
		fileLogger.SetLevel(logging.Level(fileLevel), "")
		loggers = append(loggers, fileLogger)
		if screen {
			screenLogger = logging.AddModuleLevel(logging.NewLogBackend(os.Stdout, "", 0))
			screenLogger.SetLevel(logging.Level(screenLevel), "")
			loggers = append(loggers, screenLogger)
		}
		if telemetryWriter != nil {
			loggers = append(loggers, telemetryWriter)
		}
		logging.SetBackend(loggers...)
		logging.SetFormatter(format)
		logger := Logger{
			logger: logging.MustGetLogger(loggerName),
		}
		logger.SetCallDepth(0)
		loggerCache[loggerName] = &logger
	}
	if loggerCache[loggerName] == nil {
		panic("Could not get init logger")
	}
	return loggerCache[loggerName]
}

func (logger *Logger) Copy() *Logger {
	var loggerCopy Logger = *logger
	var loggingLoggerCopy logging.Logger = *logger.logger
	loggerCopy.logger = &loggingLoggerCopy
	return &loggerCopy
}

func (logger *Logger) SetCallDepth(depth int) {
	logger.logger.ExtraCalldepth = depth + 1
}

func (logger *Logger) GetCallDepth() int {
	return logger.logger.ExtraCalldepth - 1
}

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
