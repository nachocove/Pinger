package Pinger

import (
	"errors"
	"fmt"
	"net/http"

	_ "net/http/pprof"
	"net/rpc"

	"github.com/op/go-logging"
	"runtime"
)

type BackendPolling struct {
	logger *logging.Logger
	debug  bool
}

type StartPollArgs struct {
	MailInfo *MailPingInformation
}

type StopPollArgs struct {
	ClientId string
}

type PollingResponse struct {
	Code    int
	Message string
}

const RPCPort = 60600

const (
	PollingReplyOK    = 1
	PollingReplyError = 0
)

var pollMap map[string]*MailPingInformation

func init() {
	pollMap = make(map[string]*MailPingInformation)
}

func (t *BackendPolling) internal_start(args *StartPollArgs, reply *PollingResponse) error {
	t.logger.Debug("Received request for %s", args.MailInfo.ClientId)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.MailInfo.ClientId]
	if ok == true {
		if pi == nil {
			return errors.New(fmt.Sprintf("Could not find poll item in map: %s", args.MailInfo.ClientId))
		}
		t.logger.Debug("Already polling for %s", args.MailInfo.ClientId)
		reply.Message = "Running"
		// TODO Check to see if we're still running. Maybe get a status and return it. Maybe some stats?
		// If we detect any issues with the polling routine for this client, kill it and set pi to nil.
	} else {
		if pi != nil {
			panic("Got a pi but ok is false?")
		}
		// nothing started yet. So start it.
		pi = args.MailInfo
		args.MailInfo.Start(t.debug, t.logger)
		pollMap[args.MailInfo.ClientId] = args.MailInfo
		t.logger.Debug("Starting polling for %s", args.MailInfo.ClientId)
		reply.Message = "Started"
	}

	reply.Code = replyCode
	return nil
}

func (t *BackendPolling) internal_stop(args *StopPollArgs, reply *PollingResponse) error {
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
		err := pi.Stop(t.debug, t.logger)
		if err != nil {
			return err
		}
		reply.Message = "Stopped"

	}
	reply.Code = replyCode
	return nil
}

func RecoverCrash(logger *logging.Logger) {
	if err := recover(); err != nil {
		logger.Error("Error: %s", err)
		stack := make([]byte, 8*1024)
		stack = stack[:runtime.Stack(stack, false)]
		logger.Debug("Stack: %s", stack)
	}
}

func (t *BackendPolling) Start(args *StartPollArgs, reply *PollingResponse) error {
	defer RecoverCrash(t.logger)
	return t.internal_start(args, reply)
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) error {
	defer RecoverCrash(t.logger)
	return t.internal_stop(args, reply)
}

func NewBackendPolling(debug bool, logger *logging.Logger) *BackendPolling {
	return &BackendPolling{logger: logger, debug: debug}
}

func StartPollingRPCServer(debug bool, logger *logging.Logger) {
	pollingAPI := NewBackendPolling(debug, logger)
	rpc.Register(pollingAPI)
	rpc.HandleHTTP()

	rpcConnectString := fmt.Sprintf("%s:%d", "localhost", RPCPort)
	logger.Info("Starting RPC server on %s", rpcConnectString)
	err := http.ListenAndServe(rpcConnectString, nil)
	if err != nil {
		panic(err)
	}
}
