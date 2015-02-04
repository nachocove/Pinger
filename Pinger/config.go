package Pinger

import (
	"code.google.com/p/gcfg"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"os"
	"path"
)

type Configuration struct {
	Global GlobalConfiguration
	Aws    AWSConfiguration
	Db     DBConfiguration
}

type GlobalConfiguration struct {
	DumpRequests bool
	LogDir       string
	LogFileName  string
	LogFileLevel string
	Debug        bool

	// private
	logFileLevel logging.Level
}

func NewGlobalConfiguration() *GlobalConfiguration {
	return &GlobalConfiguration{
		Debug:        defaultDebug,
		LogDir:       defaultLogDir,
		LogFileName:  defaultLogFileName,
		LogFileLevel: defaultLogFileLevel,
	}
}

const (
	defaultDumpRequests = false
	defaultDebug        = false
	defaultLogDir       = "./log"
	defaultLogFileName  = ""
	defaultLogFileLevel = "INFO"
)

func NewConfiguration() *Configuration {
	config := &Configuration{}
	config.Global = *NewGlobalConfiguration()
	return config
}

func LevelNameToLevel(levelName string) (logging.Level, error) {
	var level logging.Level
	switch {
	case levelName == "WARNING":
		level = logging.WARNING
	case levelName == "ERROR":
		level = logging.ERROR
	case levelName == "DEBUG":
		level = logging.DEBUG
	case levelName == "INFO":
		level = logging.INFO
	case levelName == "CRITICAL":
		level = logging.CRITICAL
	case levelName == "NOTICE":
		level = logging.NOTICE
	default:
		return 0, errors.New(fmt.Sprintf("Unknown logging level %s", level))
	}
	return level, nil
}

func (gconfig *GlobalConfiguration) Validate() error {
	if gconfig.LogFileName == "" {
		gconfig.LogFileName = fmt.Sprintf("%s.log", path.Base(os.Args[0]))
	}
	level, err := LevelNameToLevel(gconfig.LogFileLevel)
	if err != nil {
		return err
	}
	gconfig.logFileLevel = level
	return nil
}

func (gconfig *GlobalConfiguration) InitLogging(screen bool, screenLevel logging.Level) (*logging.Logger, error) {
	err := gconfig.Validate()
	if err != nil {
		return nil, err
	}
	if !exists(gconfig.LogDir) {
		return nil, errors.New(fmt.Sprintf("Logging directory %s does not exist.\n", gconfig.LogDir))
	}
	loggerName := path.Base(os.Args[0])
	logger, err := InitLogging(loggerName, path.Join(gconfig.LogDir, gconfig.LogFileName), gconfig.logFileLevel, screen, screenLevel)
	if err != nil {
		return nil, err
	}
	logger.Info("Started logging %s %v", gconfig.LogFileLevel, os.Args)
	return logger, nil
}

func ReadConfig(filename string) (*Configuration, error) {
	config := NewConfiguration()
	err := gcfg.ReadFileInto(config, filename)
	if err != nil {
		return nil, err
	}
	err = config.Global.Validate()
	if err != nil {
		return nil, err
	}
	err = config.Aws.Validate()
	if err != nil {
		return nil, err
	}
	err = config.Db.Validate()
	if err != nil {
		return nil, err
	}
	return config, nil
}
