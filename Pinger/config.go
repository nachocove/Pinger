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
	Logging   LoggingConfiguration
	Aws       AWS.AWSConfiguration
	Db        DBConfiguration
	Rpc       RPCServerConfiguration
	Telemetry Telemetry.TelemetryConfiguration
	Backend   BackendConfiguration
	Server    ServerConfiguration
}

type BackendConfiguration struct {
	Debug         bool
	DumpRequests  bool
	PingerUpdater int `gcfg:"pinger-updater"`
	APNSSandbox   bool
	APNSKeyFile   string
	APNSCertFile  string
}

func NewBackendConfiguration() *BackendConfiguration {
	return &BackendConfiguration{
		Debug:         defaultDebug,
		DumpRequests:  defaultDumpRequests,
		PingerUpdater: defaultPingerUpdater,
	}
}

func (cfg *BackendConfiguration) validate() error {
	if cfg.APNSKeyFile != "" && !exists(cfg.APNSKeyFile) {
		return fmt.Errorf("Key file %s does not exist", cfg.APNSKeyFile)
	}
	if cfg.APNSCertFile != "" && !exists(cfg.APNSCertFile) {
		return fmt.Errorf("Cert file %s does not exist", cfg.APNSCertFile)
	}
	return nil
}
type LoggingConfiguration struct {
	LogDir       string
	LogFileName  string
	LogFileLevel string

	// private
	logFileLevel Logging.Level
}

const (
	defaultDumpRequests  = false
	defaultDebug         = false
	defaultDebugSql      = false
	defaultLogDir        = "./log"
	defaultLogFileName   = ""
	defaultLogFileLevel  = "INFO"
	defaultPingerUpdater = 0
)

func NewLoggingConfiguration() *LoggingConfiguration {
	return &LoggingConfiguration{
		LogDir:       defaultLogDir,
		LogFileName:  defaultLogFileName,
		LogFileLevel: defaultLogFileLevel,
	}
}

func NewConfiguration() *Configuration {
	config := &Configuration{
		Logging:   *NewLoggingConfiguration(),
		Aws:       AWS.AWSConfiguration{},
		Db:        DBConfiguration{},
		Rpc:       NewRPCServerConfiguration(),
		Telemetry: *Telemetry.NewTelemetryConfiguration(),
		Server:    *NewServerConfiguration(),
	}
	return config
}

func (config *Configuration) Read(filename string) error {
	err := gcfg.ReadFileInto(config, filename)
	if err != nil {
		return err
	}
	return nil
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

func (cfg *LoggingConfiguration) Validate() error {
	if cfg.LogFileName == "" {
		cfg.LogFileName = fmt.Sprintf("%s.log", path.Base(os.Args[0]))
	}
	level, err := Logging.LogLevel(cfg.LogFileLevel)
	if err != nil {
		return err
	}
	cfg.logFileLevel = level
	return nil
}

func (cfg *LoggingConfiguration) InitLogging(screen bool, screenLevel Logging.Level, telemetryWriter *Telemetry.TelemetryWriter, debug bool) (*Logging.Logger, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	if !exists(cfg.LogDir) {
		return nil, fmt.Errorf("Logging directory %s does not exist.", cfg.LogDir)
	}
	loggerName := path.Base(os.Args[0])
	logger := Logging.InitLogging(loggerName, path.Join(cfg.LogDir, cfg.LogFileName), cfg.logFileLevel, screen, screenLevel, telemetryWriter, debug)
	return logger, nil
}

func ReadConfig(filename string) (*Configuration, error) {
	config := NewConfiguration()
	err := gcfg.ReadFileInto(config, filename)
	if err != nil {
		return nil, err
	}
	err = config.Logging.Validate()
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
	err = config.Server.validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error validate server config:\n%v\n", err)
		os.Exit(1)
	}
	err = config.Telemetry.Validate()
	if err != nil {
		return nil, err
	}
	return config, nil
}
