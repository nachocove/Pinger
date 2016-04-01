package Pinger

import (
	"code.google.com/p/gcfg"
	"crypto/x509"
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"io/ioutil"
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
	Debug                 bool
	DumpRequests          bool
	PingerUpdater         int `gcfg:"pinger-updater"`
	APNSSandbox           bool
	APNSKeyFile           string
	APNSCertFile          string
	APNSFeedbackPeriod    int
	ReArmTimeout          int `gcfg:"rearm-timeout"`
	APNSAlert             bool
	APNSSound             string
	APNSContentAvailable  int
	APNSExpirationSeconds int64
}

var days_28 int64 = 28 * 24 * 60 * 60

// We have this hardcoded, instead of a config item,
// to make it harder for an attacker to replace the
// os trust store with a compromised copy.
// The downside is that we would have to change this
// for different OS's and distro's. This is currently not
// an issue.
var OsTrustStore = "/etc/pki/tls/certs/ca-bundle.crt"

func NewBackendConfiguration() *BackendConfiguration {
	return &BackendConfiguration{
		Debug:                 defaultDebug,
		DumpRequests:          defaultDumpRequests,
		PingerUpdater:         defaultPingerUpdater,
		APNSFeedbackPeriod:    defaultAPNSFeedbackPeriod,
		ReArmTimeout:          defaultReArmTimeout,
		APNSAlert:             true,
		APNSSound:             "silent.wav",
		APNSContentAvailable:  1,
		APNSExpirationSeconds: days_28,
	}
}

func (cfg *BackendConfiguration) validate() error {
	if cfg.APNSKeyFile != "" && !exists(cfg.APNSKeyFile) {
		return fmt.Errorf("Key file %s does not exist", cfg.APNSKeyFile)
	}
	if cfg.APNSCertFile != "" && !exists(cfg.APNSCertFile) {
		return fmt.Errorf("Cert file %s does not exist", cfg.APNSCertFile)
	}
	if cfg.APNSKeyFile != "" && cfg.APNSCertFile != "" && cfg.APNSFeedbackPeriod <= 0 {
		return fmt.Errorf("APNSFeedbackPeriod can not be <= 0 if APNS cert and keys are configured")
	}
	return nil
}

var rootCAs *x509.CertPool

// Read the certs in OsTrustStore, then add in the additional
// certs, then return that CertPool structure for use in TlsConfig's
func (cfg *BackendConfiguration) RootCerts() *x509.CertPool {
	if rootCAs == nil {
		roots := x509.NewCertPool()
		data, err := ioutil.ReadFile(OsTrustStore)
		if err != nil {
			panic(err.Error())
		}
		if !roots.AppendCertsFromPEM(data) {
			panic(fmt.Sprintf("Could not read PEM file %s", OsTrustStore))
		}

		for i, cert := range extra_certs {
			if !roots.AppendCertsFromPEM([]byte(cert)) {
				panic(fmt.Sprintf("Could not parse PEM cert %s", i))
			}
		}
		for _, c := range roots.Subjects() {
			fmt.Println("Cert: %v", c)
		}
		rootCAs = roots
	}
	return rootCAs
}

type LoggingConfiguration struct {
	LogDir       string
	LogFileName  string
	LogFileLevel string

	// private
	logFileLevel Logging.Level
}

const (
	defaultDumpRequests       = false
	defaultDebug              = false
	defaultLogDir             = "./log"
	defaultLogFileName        = ""
	defaultLogFileLevel       = "INFO"
	defaultPingerUpdater      = 0
	defaultAPNSFeedbackPeriod = 10
	defaultReArmTimeout       = 10
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
		Backend:   *NewBackendConfiguration(),
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
	err := config.Read(filename)
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
	err = config.Backend.validate()
	if err != nil {
		return nil, err
	}
	return config, nil
}
