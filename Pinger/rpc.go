package Pinger

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	// needed to get the http.ListenAndServe below to pick up the profiler
	_ "net/http/pprof"
	"net/rpc"

	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils"
	"github.com/op/go-logging"
	"runtime"
)

type pollMapType map[string]*MailClientContext

type BackendPolling struct {
	dbm         *gorp.DbMap
	config      *Configuration
	logger      *logging.Logger
	loggerLevel logging.Level
	debug       bool
	pollMap     pollMapType
}

type StartPollArgs struct {
	MailInfo *MailPingInformation
}

func (sa *StartPollArgs) getLogPrefix() string {
	return sa.MailInfo.getLogPrefix()
}

type StopPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	StopToken     string

	logPrefix string
}

func (sp *StopPollArgs) getLogPrefix() string {
	if sp.logPrefix == "" {
		sp.logPrefix = fmt.Sprintf("%s:%s:%s", sp.DeviceId, sp.ClientId, sp.ClientContext)
	}
	return sp.logPrefix
}

type DeferPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Timeout       int64
	StopToken     string

	logPrefix string
}

func (dp *DeferPollArgs) getLogPrefix() string {
	if dp.logPrefix == "" {
		dp.logPrefix = fmt.Sprintf("%s:%s:%s", dp.DeviceId, dp.ClientId, dp.ClientContext)
	}
	return dp.logPrefix
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

func (t *BackendPolling) ToggleDebug() {
	t.debug = !t.debug
	t.loggerLevel = ToggleLogging(t.logger, t.loggerLevel)
}

func validateClientID(clientID string) error {
	if clientID == "" {
		return fmt.Errorf("Empty client ID is not valid")
	}
	return DefaultPollingContext.config.Aws.validateCognitoID(clientID)
}

func (t *BackendPolling) pollMapKey(clientId, clientContext, deviceId string) string {
	return fmt.Sprintf("%s--%s--%s", clientId, clientContext, deviceId)
}

func (t *BackendPolling) startPolling(args *StartPollArgs, reply *StartPollingResponse) error {
	t.logger.Debug("%s: Received StartPoll request", args.getLogPrefix())
	pollMapKey := t.pollMapKey(args.MailInfo.ClientId, args.MailInfo.ClientContext, args.MailInfo.DeviceId)
	reply.Code = PollingReplyOK
	var client *MailClientContext
	client, ok := t.pollMap[pollMapKey]
	if ok == true {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll session in map", args.getLogPrefix())
		}
		err := updateLastContact(t.dbm, args.MailInfo.ClientId, args.MailInfo.ClientContext, args.MailInfo.DeviceId, t.logger)
		if err != nil {
			reply.Message = err.Error()
			reply.Code = PollingReplyError
			return nil
		}
		t.logger.Debug("%s: Found Existing polling session", args.getLogPrefix())
		status, err := client.Status()
		switch {
		case status == MailClientStatusStopped:
			t.logger.Debug("%s: Polling has stopped.", args.getLogPrefix())

		case status == MailClientStatusPinging:
			t.logger.Debug("%s: Polling. Stopping it.", args.getLogPrefix())

		case status == MailClientStatusError:
			if err != nil {
				t.logger.Debug("%s: Not polling. Last error was %s", args.getLogPrefix(), err)
				reply.Message = fmt.Sprintf("Previous Ping failed with error: %s", err.Error())
			} else {
				t.logger.Debug("%s: Not polling.", args.getLogPrefix())
				reply.Message = fmt.Sprintf("Not polling")
			}
			reply.Code = PollingReplyWarn
		}
		err = client.Action(PingerStop)
		if err != nil {
			reply.Message = err.Error()
			reply.Code = PollingReplyError
			return nil
		}
		client = nil
	} else {
		if client != nil {
			panic("Got a client but ok is false?")
		}
	}
	err := validateClientID(args.MailInfo.ClientId)
	if err != nil {
		return err
	}

	di, err := newDeviceInfoPI(t.dbm, args.MailInfo, t.logger)
	if err != nil {
		message := fmt.Sprintf("Could not save deviceInfo: %s", err)
		t.logger.Warning(message)
		reply.Message = message
		reply.Code = PollingReplyError
		return nil
	}
	t.logger.Debug("%s: created/updated device info", args.getLogPrefix())

	// nothing started. So start it.
	client, err = NewMailClientContext(args.MailInfo, di, t.debug, false, t.logger)
	if err != nil {
		message := fmt.Sprintf("%s: Could not create new client: %s", args.getLogPrefix(), err)
		t.logger.Warning(message)
		reply.Message = message
		reply.Code = PollingReplyError
		return nil
	}
	t.pollMap[pollMapKey] = client
	reply.Token = client.stopToken
	return nil
}

func (t *BackendPolling) stopPolling(args *StopPollArgs, reply *PollingResponse) error {
	t.logger.Debug("%s: Received stopPoll request", args.getLogPrefix())
	pollMapKey := t.pollMapKey(args.ClientId, args.ClientContext, args.DeviceId)
	client, ok := t.pollMap[pollMapKey]
	if ok == false {
		// nothing on file.
		t.logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "Not Polling"
		return nil
	} else {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		validToken := client.validateStopToken(args.StopToken)
		if validToken == false {
			t.logger.Warning("%s: invalid token", args.getLogPrefix())
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
			return nil
		} else {
			err := updateLastContact(t.dbm, args.ClientId, args.ClientContext, args.DeviceId, t.logger)
			if err != nil {
				t.logger.Error("%s: Could not update last contact %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			}
			err = client.stop()
			if err != nil {
				t.logger.Error("%s:: Error stopping poll: %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			} else {
				delete(t.pollMap, args.ClientId)
				reply.Message = "Stopped"
			}
		}
	}
	reply.Code = PollingReplyOK
	return nil
}

func (t *BackendPolling) deferPolling(args *DeferPollArgs, reply *PollingResponse) error {
	t.logger.Debug("%s: Received deferPoll request", args.getLogPrefix())
	pollMapKey := t.pollMapKey(args.ClientId, args.ClientContext, args.DeviceId)
	client, ok := t.pollMap[pollMapKey]
	if ok == false {
		// nothing on file.
		t.logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "Not Polling"
		return nil
	} else {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		validToken := client.validateStopToken(args.StopToken)
		if validToken == false {
			t.logger.Warning("%s: invalid token", args.getLogPrefix())
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
			return nil
		} else {
			err := updateLastContact(t.dbm, args.ClientId, args.ClientContext, args.DeviceId, t.logger)
			if err != nil {
				t.logger.Error("%s: Could not update last contact %s", args.getLogPrefix(), err.Error())
				reply.Code = PollingReplyError
				reply.Message = err.Error()
				return nil
			}
			err = client.deferPoll(args.Timeout)
			if err != nil {
				t.logger.Error("%s: Error deferring poll: %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			}
		}
	}
	reply.Code = PollingReplyOK
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
	dbm, err := initDB(&config.Db, true, false, logger)
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

func StartPollingRPCServer(config *Configuration, debug bool, logger *logging.Logger) error {
	pollingAPI, err := NewBackendPolling(config, debug, logger)
	if err != nil {
		return err
	}
	log.SetOutput(ioutil.Discard) // rpc.Register logs a warning for ToggleDebug, which we don't want.

	rpc.Register(pollingAPI)
	rpc.HandleHTTP()

	go alertAllDevices()

	rpcConnectString := fmt.Sprintf("%s:%d", "localhost", RPCPort)
	logger.Info("Starting RPC server on %s (pinger id %s)", rpcConnectString, pingerHostId)
	err = http.ListenAndServe(rpcConnectString, nil)
	if err != nil {
		return err
	}
	return nil
}

func alertAllDevices() error {
	devices, err := getAllMyDeviceInfo(DefaultPollingContext.dbm, DefaultPollingContext.logger)
	if err != nil {
		return err
	}
	for _, di := range devices {
		DefaultPollingContext.logger.Info("%s: sending PingerNotificationRegister to device", di.getLogPrefix())
		err = di.push(PingerNotificationRegister)
		if err != nil {
			DefaultPollingContext.logger.Warning("%s: Could not send push: %s", di.getLogPrefix(), err.Error())
		}
	}
	return nil
}
