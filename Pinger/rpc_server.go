package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
)

type BackendPolling struct {
	dbm         *gorp.DbMap
	config      *Configuration
	logger      *Logging.Logger
	loggerLevel Logging.Level
	debug       bool
	pollMap     pollMapType
}

var DefaultPollingContext *BackendPolling

func NewBackendPolling(config *Configuration, debug, debugSql bool, logger *Logging.Logger) (*BackendPolling, error) {
	dbm, err := initDB(&config.Db, true, debugSql, logger)
	if err != nil {
		return nil, err
	}
	backend := &BackendPolling{
		dbm:         dbm,
		config:      config,
		logger:      logger,
		loggerLevel: -1,
		debug:       debug,
		pollMap:     make(pollMapType),
	}
	DefaultPollingContext = backend
	Utils.AddDebugToggleSignal(DefaultPollingContext)
	return DefaultPollingContext, nil
}

func (t *BackendPolling) newMailClientContext(pi *MailPingInformation, doStats bool) (MailClientContextType, error) {
	return NewMailClientContext(t.dbm, pi, t.debug, false, t.logger)
}

func (t *BackendPolling) validateClientID(clientID string) error {
	if clientID == "" {
		return fmt.Errorf("Empty client ID is not valid")
	}
	return DefaultPollingContext.config.Aws.ValidateCognitoID(clientID)
}

func (t *BackendPolling) ToggleDebug() {
	t.debug = !t.debug
	t.loggerLevel = Logging.ToggleLogging(t.logger, t.loggerLevel)
}

func (t *BackendPolling) Start(args *StartPollArgs, reply *StartPollingResponse) (err error) {
	return RPCStartPoll(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) (err error) {
	return RPCStopPoll(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (t *BackendPolling) Defer(args *DeferPollArgs, reply *PollingResponse) (err error) {
	return RPCDeferPoll(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (t *BackendPolling) FindActiveSessions(args *FindSessionsArgs, reply *FindSessionsResponse) (err error) {
	return RPCFindActiveSessions(&t.pollMap, args, reply, t.logger)
}
