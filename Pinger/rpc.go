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
	defer recoverCrash()
	logger.Debug("Received request for %s", args.Device.ClientId)
	replyCode := PollingReplyOK
	pi, ok := pollMap[args.Device.ClientId]
	if ok == true {
		if pi == nil {
			return errors.New(fmt.Sprintf("Could not find poll item in map: %s", args.Device.ClientId))
		}
		logger.Debug("Already polling for %s", args.Device.ClientId)
		reply.Message = "Running"
		// TODO Check to see if we're still running. Maybe get a status and return it. Maybe some stats?
		// If we detect any issues with the polling routine for this client, kill it and set pi to nil.
	} else {
		if pi != nil {
			panic("Got a pi but ok is false?")
		}
	}

	if pi == nil {
		// nothing started yet. So start it.
		dialString := ""
		pingPeriodicity := 5
		reopenConnection := true
		debug := false
		logger := logger
		var mailserver MailServer
		switch {
		case args.Device.MailClientType == "exchange":
			mailserver = NewExchangeClient(dialString, pingPeriodicity, reopenConnection, nil, 0, debug, logger)

		default:
			panic(fmt.Sprintf("Unknown/unsupported mailserver type %s", args.Device.MailClientType))
		}
		pi := pollMapItem{
			startArgs:  args,
			mailServer: mailserver,
		}
		pollMap[args.Device.ClientId] = &pi
		logger.Debug("Starting polling for %s", args.Device.ClientId)
		reply.Message = "Started"
	}
	reply.Code = replyCode
	return nil
}

func recoverCrash() {
	if r := recover(); r != nil {
		fmt.Println("Crash:", r)
	}
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) error {
	defer recoverCrash()
	logger.Debug("Received request for %s", args.ClientId)
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
