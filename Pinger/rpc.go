package Pinger

import (
	"errors"
	"fmt"
	"net/http"

	_ "net/http/pprof"
	"net/rpc"

	"github.com/op/go-logging"
)

type BackendPolling int

type StartPollArgs struct {
	Device       DeviceInfo
	MailEndpoint string
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

func (t *BackendPolling) Start(args *StartPollArgs, reply *PollingResponse) error {
	logger.Debug("Received request for for %v", args)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.Device.ClientId]
	if ok == false {
		// nothing started yet. So start it.
		dialString := ""
		pingPeriodicity := 5
		reopenConnection := true
		debug := false
		logger := logger
		pi := pollMapItem{
			startArgs:  args,
			mailServer: NewExchangeClient(dialString, pingPeriodicity, reopenConnection, nil, 0, debug, logger),
		}
		pollMap[args.Device.ClientId] = &pi
		logger.Debug("Starting polling for %s", args.Device.ClientId)
		reply.Message = "Started"
	} else {
		if pi == nil {
			return errors.New(fmt.Sprintf("Could not find poll item in map: %s", args.Device.ClientId))
		}
		logger.Debug("Already polling for %s", args.Device.ClientId)
		reply.Message = "Running"
	}
	reply.Code = replyCode
	return nil
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) error {
	logger.Debug("Received request for for %v", args)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.ClientId]
	if ok == false {
		// nothing on file.
		reply.Message = "NotRunning"
	} else {
		if pi == nil {
			return errors.New(fmt.Sprintf("Could not find poll item in map: %s", args.ClientId))
		}
		pi.mailServer.Action(Stop)
		reply.Message = "Stopped"

	}
	reply.Code = replyCode
	return nil
}

var logger *logging.Logger

func StartPollingRPCServer(l *logging.Logger) {
	logger = l
	pollingAPI := new(BackendPolling)
	rpc.Register(pollingAPI)
	rpc.HandleHTTP()
	rpcConnectString := fmt.Sprintf("%s:%d", "localhost", RPCPort)
	logger.Info("Starting RPC server on %s", rpcConnectString)
	err := http.ListenAndServe(rpcConnectString, nil)
	if err != nil {
		panic(err)
	}
}
