package Pinger

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"

	"github.com/op/go-logging"
)

type BackendPolling int

type StartPollArgs struct {
	ClientId     string
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
	logger.Debug("Starting polling for %v", args)
	reply.Code = PollingReplyOK
	reply.Message = "OK"
	return nil
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) error {
	logger.Debug("Stopping polling for %v", args)
	reply.Code = PollingReplyOK
	reply.Message = "OK"
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
