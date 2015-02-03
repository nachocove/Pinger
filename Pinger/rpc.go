package Pinger

import (
	"errors"
	"fmt"
	"net/http"

	_ "net/http/pprof"
	"net/rpc"

	"github.com/coopernurse/gorp"
	"github.com/op/go-logging"
	"runtime"
)

type BackendPolling struct {
	dbm    *gorp.DbMap
	config *Configuration
	logger *logging.Logger
	debug  bool
}

type StartPollArgs struct {
	MailInfo *MailPingInformation
}

type StopPollArgs struct {
	ClientId  string
	StopToken string
}

type DeferPollArgs struct {
	ClientId  string
	StopToken string
}

type PollingResponse struct {
	Code    int
	Message string
}

type StartPollingResponse struct {
	Code    int
	Token   string
	Message string
}

const RPCPort = 60600

const (
	PollingReplyError = 0
	PollingReplyOK    = 1
	PollingReplyWarn  = 2
)

var pollMap map[string]*MailPingInformation

func init() {
	pollMap = make(map[string]*MailPingInformation)
}

func (t *BackendPolling) startPolling(args *StartPollArgs, reply *StartPollingResponse) error {
	t.logger.Debug("%s: Received StartPoll request", args.MailInfo.ClientId)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.MailInfo.ClientId]
	if ok == true {
		if pi == nil {
			return errors.New(fmt.Sprintf("%s: Could not find poll session in map", args.MailInfo.ClientId))
		}
		t.logger.Debug("%s: Found Existing polling session", args.MailInfo.ClientId)
		status, err := pi.status()
		if status != MailClientStatusPinging || err != nil {
			t.logger.Debug("%s: Not polling. Last error was %s", args.MailInfo.ClientId, err)
			reply.Message = fmt.Sprintf("Previous Ping failed with error: %s", err.Error())
			reply.Code = PollingReplyWarn
		}
		err = pi.stop(t.debug, t.logger)
		if err != nil {
			reply.Message = err.Error()
			reply.Code = PollingReplyError
			return nil
		}
		pi = nil
	} else {
		if pi != nil {
			panic("Got a pi but ok is false?")
		}
	}
	// nothing started. So start it.
	pi = args.MailInfo

	err := newDeviceInfoPI(t.dbm, pi)
	if err != nil {
		message := fmt.Sprintf("Could not save deviceInfo: %s", err)
		t.logger.Warning(message)
		reply.Message = message
		reply.Code = PollingReplyError
		return nil
	}
	t.logger.Debug("created/updated device info %s", pi.ClientId)

	stopToken, err := args.MailInfo.start(t.debug, t.logger)
	if err != nil {
		reply.Message = err.Error()
		reply.Code = PollingReplyError
	}
	pollMap[args.MailInfo.ClientId] = args.MailInfo
	reply.Token = stopToken
	reply.Code = replyCode
	return nil
}

func (t *BackendPolling) stopPolling(args *StopPollArgs, reply *PollingResponse) error {
	t.logger.Debug("Received request for %s", args.ClientId)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.ClientId]
	if ok == false {
		// nothing on file.
		reply.Message = "NotRunning"
	} else {
		if pi == nil {
			return errors.New(fmt.Sprintf("Could not find poll item in map: %s", args.ClientId))
		}
		if pi.validateStopToken(args.StopToken) == false {
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
		} else {
			err := pi.stop(t.debug, t.logger)
			if err != nil {
				reply.Message = err.Error()
				reply.Code = PollingReplyError
			} else {
				delete(pollMap, args.ClientId)
				reply.Message = "Stopped"
			}
		}
	}
	reply.Code = replyCode
	return nil
}

func (t *BackendPolling) deferPolling(args *DeferPollArgs, reply *PollingResponse) error {
	t.logger.Debug("Received request for %s", args.ClientId)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.ClientId]
	if ok == false {
		// nothing on file.
		reply.Message = "NotRunning"
	} else {
		if pi == nil {
			return errors.New(fmt.Sprintf("Could not find poll item in map: %s", args.ClientId))
		}
		if pi.validateStopToken(args.StopToken) == false {
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
		} else {
			err := pi.deferPoll(t.debug, t.logger)
			if err != nil {
				reply.Message = err.Error()
				reply.Code = PollingReplyError
			}
		}
	}
	reply.Code = replyCode
	return nil
}

func recoverCrash(logger *logging.Logger) {
	if err := recover(); err != nil {
		logger.Error("Error: %s", err)
		stack := make([]byte, 8*1024)
		stack = stack[:runtime.Stack(stack, false)]
		logger.Debug("Stack: %s", stack)
	}
}

func (t *BackendPolling) Start(args *StartPollArgs, reply *StartPollingResponse) error {
	defer recoverCrash(t.logger)
	return t.startPolling(args, reply)
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) error {
	defer recoverCrash(t.logger)
	return t.stopPolling(args, reply)
}

func (t *BackendPolling) Defer(args *DeferPollArgs, reply *PollingResponse) error {
	defer recoverCrash(t.logger)
	return t.deferPolling(args, reply)
}

var DefaultPollingContext *BackendPolling

func NewBackendPolling(config *Configuration, debug bool, logger *logging.Logger) (*BackendPolling, error) {
	dbm, err := initDB(&config.Db, true, debug, logger)
	if err != nil {
		return nil, err
	}
	DefaultPollingContext = &BackendPolling{
		dbm:    dbm,
		config: config,
		logger: logger,
		debug:  debug,
	}
	return DefaultPollingContext, nil
}

func StartPollingRPCServer(config *Configuration, debug bool, logger *logging.Logger) error {
	pollingAPI, err := NewBackendPolling(config, debug, logger)
	if err != nil {
		return err
	}
	rpc.Register(pollingAPI)
	rpc.HandleHTTP()

	rpcConnectString := fmt.Sprintf("%s:%d", "localhost", RPCPort)
	logger.Info("Starting RPC server on %s", rpcConnectString)
	err = http.ListenAndServe(rpcConnectString, nil)
	if err != nil {
		return err
	}
	return nil
}
