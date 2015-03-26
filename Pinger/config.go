package Pinger

import (
	"code.google.com/p/gcfg"
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"os"
	"path"
)

type Configuration struct {
	Global    GlobalConfiguration
	Aws       AWS.AWSConfiguration
	Db        DBConfiguration
	Rpc       RPCServerConfiguration
	Telemetry Telemetry.TelemetryConfiguration
}

type GlobalConfiguration struct {
	DumpRequests      bool
	IgnorePushFailure bool
	LogDir            string
	LogFileName       string
	LogFileLevel      string
	Debug             bool
	DebugSql          bool
	PingerUpdater     int `gcfg:"pinger-updater"`

	// private
	logFileLevel Logging.Level
}

const (
	defaultDumpRequests      = false
	defaultIgnorePushFailure = false
	defaultDebug             = false
	defaultDebugSql          = false
	defaultLogDir            = "./log"
	defaultLogFileName       = ""
	defaultLogFileLevel      = "INFO"
	defaultPingerUpdater     = 0
)

func NewGlobalConfiguration() *GlobalConfiguration {
	return &GlobalConfiguration{
		DumpRequests:      defaultDumpRequests,
		IgnorePushFailure: defaultIgnorePushFailure,
		LogDir:            defaultLogDir,
		LogFileName:       defaultLogFileName,
		LogFileLevel:      defaultLogFileLevel,
		Debug:             defaultDebug,
		DebugSql:          defaultDebugSql,
		PingerUpdater:     defaultPingerUpdater,
	}
}

func NewConfiguration() *Configuration {
	config := &Configuration{
		Global:    *NewGlobalConfiguration(),
		Aws:       AWS.AWSConfiguration{},
		Db:        DBConfiguration{},
		Rpc:       NewRPCServerConfiguration(),
		Telemetry: Telemetry.TelemetryConfiguration{},
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
	level, err := Logging.LogLevel(gconfig.LogFileLevel)
	if err != nil {
		return err
	}
	gconfig.logFileLevel = level
	return nil
}

func (gconfig *GlobalConfiguration) InitLogging(screen bool, screenLevel Logging.Level, telemetryWriter *Telemetry.TelemetryWriter, debug bool) (*Logging.Logger, error) {
	err := gconfig.Validate()
	if err != nil {
		return nil, err
	}
	if !exists(gconfig.LogDir) {
		return nil, fmt.Errorf("Logging directory %s does not exist.", gconfig.LogDir)
	}
	loggerName := path.Base(os.Args[0])
	logger := Logging.InitLogging(loggerName, path.Join(gconfig.LogDir, gconfig.LogFileName), gconfig.logFileLevel, screen, screenLevel, telemetryWriter, debug)
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
