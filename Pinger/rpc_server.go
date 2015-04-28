package Pinger

import (
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"sync"
)

type BackendPolling struct {
	logger       *Logging.Logger
	loggerLevel  Logging.Level
	debug        bool
	pollMap      pollMapType
	aws          *AWS.AWSHandle
	pollMapMutex sync.Mutex
	dbtype       DBHandlerType
	dbHandler    DBHandler
}

func NewBackendPolling(config *Configuration, debug bool, logger *Logging.Logger) (*BackendPolling, error) {
	backend := &BackendPolling{
		logger:       logger,
		loggerLevel:  -1,
		debug:        debug,
		pollMap:      make(pollMapType),
		aws:          config.Aws.NewHandle(),
		pollMapMutex: sync.Mutex{},
		dbtype:       config.Backend.DB,
	}
	var err error
	var dbm *gorp.DbMap
	// There's a bit of a chicken and egg problem, that we can probably resolve by teasing apart
	// the creation of the dbm pointer, and the actual creation of the DB/tables. For DBM,
	// this currently happens together, so we need to create dbm here first, BEFORE we
	// can create the DBHandler. For Dynamo, we attach the initDb method and call it later.
	if config.Backend.DB == DBHandlerSql {
		dbm, err = config.Db.initDB(true, logger)
		if err != nil {
			return nil, err
		}
	}

	backend.dbHandler = newDbHandler(config.Backend.DB, dbm, backend.aws)
	err = backend.dbHandler.initDb()
	if err != nil {
		panic(err)
	}
	return backend, nil
}

func (t *BackendPolling) newMailClientContext(pi *MailPingInformation, doStats bool) (MailClientContextType, error) {
	return NewMailClientContext(t.dbHandler, t.aws, pi, t.debug, false, t.logger)
}

func (t *BackendPolling) ToggleDebug() {
	t.debug = !t.debug
	t.loggerLevel = Logging.ToggleLogging(t.logger, t.loggerLevel)
}

func (t *BackendPolling) LockMap() {
	t.pollMapMutex.Lock()
}

func (t *BackendPolling) UnlockMap() {
	t.pollMapMutex.Unlock()
}

func (t *BackendPolling) Start(args *StartPollArgs, reply *StartPollingResponse) (err error) {
	return RPCStartPoll(t, &t.pollMap, args, reply, t.logger)
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) (err error) {
	return RPCStopPoll(t, &t.pollMap, args, reply, t.logger)
}

func (t *BackendPolling) Defer(args *DeferPollArgs, reply *PollingResponse) (err error) {
	return RPCDeferPoll(t, &t.pollMap, args, reply, t.logger)
}

func (t *BackendPolling) FindActiveSessions(args *FindSessionsArgs, reply *FindSessionsResponse) (err error) {
	return RPCFindActiveSessions(t, &t.pollMap, args, reply, t.logger)
}

func (t *BackendPolling) AliveCheck(args *AliveCheckArgs, reply *AliveCheckResponse) (err error) {
	return RPCAliveCheck(t, &t.pollMap, args, reply, t.logger)
}

func (t *BackendPolling) DBHandler() DBHandler {
	return t.dbHandler
}
