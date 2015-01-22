package Pinger

import (
	"errors"
	"fmt"
	"net/rpc"
)

func getRpcClient(rpcserver string) (*rpc.Client, error) {
	return rpc.DialHTTP("tcp", rpcserver)
}

func StartPoll(rpcserver string, pi *MailPingInformation) (string, error) {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return "", err
	}
	return startPoll(client, pi)
}

func startPoll(rpcClient *rpc.Client, pi *MailPingInformation) (string, error) {
	if rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply StartPollingResponse
	err := rpcClient.Call("BackendPolling.Start", &StartPollArgs{MailInfo: pi}, &reply)
	if err != nil {
		return "", err
	}
	if reply.Code != PollingReplyOK {
		return "", errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return reply.Token, nil
}

func StopPoll(rpcserver, clientId string) error {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return err
	}
	return stopPoll(client, clientId)
}

func stopPoll(rpcClient *rpc.Client, clientId string) error {
	if rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply PollingResponse
	err := rpcClient.Call("BackendPolling.Stop", &StopPollArgs{ClientId: clientId}, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}

func DeferPoll(rpcserver, clientId, token string) error {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return err
	}
	err = deferPoll(client, clientId, token)
	if err != nil {
		return err
	}
	return nil
}

func deferPoll(rpcClient *rpc.Client, clientId, token string) error {
	if rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply PollingResponse
	err := rpcClient.Call("BackendPolling.Defer", &DeferPollArgs{ClientId: clientId, StopToken: token}, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}
