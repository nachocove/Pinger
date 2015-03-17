package Pinger

import (
	"code.google.com/p/gcfg"
	"fmt"
	logging "github.com/nachocove/Pinger/Pinger/logging"
	"os"
	"path"
)

type Configuration struct {
	Global GlobalConfiguration
	Aws    AWSConfiguration
	Db     DBConfiguration
	Rpc    RPCServerConfiguration
}

type GlobalConfiguration struct {
	DumpRequests      bool
	IgnorePushFailure bool
	LogDir            string
	LogFileName       string
	LogFileLevel      string
	Debug             bool
	DebugSql          bool

	// private
	logFileLevel logging.Level
}

func NewGlobalConfiguration() *GlobalConfiguration {
	return &GlobalConfiguration{
		Debug:        defaultDebug,
		DebugSql:     defaultDebugSql,
		LogDir:       defaultLogDir,
		LogFileName:  defaultLogFileName,
		LogFileLevel: defaultLogFileLevel,
	}
}

const (
	defaultDumpRequests      = false
	defaultIgnorePushFailure = false
	defaultDebug             = false
	defaultDebugSql          = false
	defaultLogDir            = "./log"
	defaultLogFileName       = ""
	defaultLogFileLevel      = "INFO"
)

func NewConfiguration() *Configuration {
	config := &Configuration{
		Global: *NewGlobalConfiguration(),
		Aws:    AWSConfiguration{},
		Db:     DBConfiguration{},
		Rpc:    NewRPCServerConfiguration(),
	}
	return config
}

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

func (gconfig *GlobalConfiguration) Validate() error {
	if gconfig.LogFileName == "" {
		gconfig.LogFileName = fmt.Sprintf("%s.log", path.Base(os.Args[0]))
	}
	level, err := logging.LogLevel(gconfig.LogFileLevel)
	if err != nil {
		return err
	}
	gconfig.logFileLevel = level
	return nil
}

func (gconfig *GlobalConfiguration) InitLogging(screen bool, screenLevel logging.Level, debug bool) (*logging.Logger, error) {
	err := gconfig.Validate()
	if err != nil {
		return nil, err
	}
	if !exists(gconfig.LogDir) {
		return nil, fmt.Errorf("Logging directory %s does not exist.", gconfig.LogDir)
	}
	loggerName := path.Base(os.Args[0])
	logger := logging.InitLogging(loggerName, path.Join(gconfig.LogDir, gconfig.LogFileName), gconfig.logFileLevel, screen, screenLevel, debug)
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
