package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"io/ioutil"
	"log"
	"net/http"
	"net/rpc"
)

const (
	RPCProtocolHTTP = "http"
	RPCProtocolUnix = "unix"
)

type RPCServerConfiguration struct {
	Protocol string
	Path     string
	Hostname string
	Port     int
}

func (rpcConf *RPCServerConfiguration) ConnectString() string {
	switch {
	case rpcConf.Protocol == RPCProtocolHTTP:
		return fmt.Sprintf("%s:%d", rpcConf.Hostname, rpcConf.Port)

	case rpcConf.Protocol == RPCProtocolUnix:
		return fmt.Sprintf("%s", rpcConf.Path)
	}
	return ""
}

func (rpcConf *RPCServerConfiguration) String() string {
	return fmt.Sprintf("%s://%s", rpcConf.Protocol, rpcConf.ConnectString())
}

func NewRPCServerConfiguration() RPCServerConfiguration {
	return RPCServerConfiguration{
		Protocol: RPCProtocolHTTP,  // options: "unix", "http"
		Path:     "/tmp/PingerRpc", // used if Protocol is "unix"
		Hostname: "localhost",      // used if Protocol is "http"
		Port:     RPCPort,          // used if Protocol is "http"
	}
}

type BackendPolling struct {
	dbm         *gorp.DbMap
	config      *Configuration
	logger      *Logging.Logger
	loggerLevel Logging.Level
	debug       bool
	pollMap     pollMapType
}

const RPCPort = 60600

const (
	PollingReplyError = 0
	PollingReplyOK    = 1
	PollingReplyWarn  = 2
)

func (t *BackendPolling) ToggleDebug() {
	t.debug = !t.debug
	t.loggerLevel = Logging.ToggleLogging(t.logger, t.loggerLevel)
}

type pollMapType map[string]*MailClientContext

func (t *BackendPolling) pollMapKey(clientId, clientContext, deviceId string) string {
	return fmt.Sprintf("%s--%s--%s", clientId, clientContext, deviceId)
}

var DefaultPollingContext *BackendPolling

func NewBackendPolling(config *Configuration, debug, debugSql bool, logger *Logging.Logger) (*BackendPolling, error) {
	dbm, err := initDB(&config.Db, true, debugSql, logger)
	if err != nil {
		return nil, err
	}
	DefaultPollingContext = &BackendPolling{
		dbm:         dbm,
		config:      config,
		logger:      logger,
		loggerLevel: -1,
		debug:       debug,
		pollMap:     make(pollMapType),
	}

	Utils.AddDebugToggleSignal(DefaultPollingContext)
	return DefaultPollingContext, nil
}

func StartPollingRPCServer(config *Configuration, debug, debugSql bool, logger *Logging.Logger) error {
	pollingAPI, err := NewBackendPolling(config, debug, debugSql, logger)
	if err != nil {
		return err
	}
	log.SetOutput(ioutil.Discard) // rpc.Register logs a warning for ToggleDebug, which we don't want.

	rpc.Register(pollingAPI)

	go alertAllDevices()

	logger.Debug("Starting RPC server on %s (pinger id %s)", config.Rpc.String(), pingerHostId)
	switch {
	case config.Rpc.Protocol == RPCProtocolHTTP:
		rpc.HandleHTTP()
		err = http.ListenAndServe(config.Rpc.ConnectString(), nil)
		if err != nil {
			return err
		}
	case config.Rpc.Protocol == RPCProtocolUnix:
		panic("UNIX server is not yet implemented")
	}
	return nil
}
