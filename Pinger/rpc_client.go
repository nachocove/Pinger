package Pinger

import (
	"errors"
	"fmt"
	"net/rpc"
)

func getRpcClient(rpcserver string) (*rpc.Client, error) {
	return rpc.DialHTTP("tcp", rpcserver)
}

func StartPoll(rpcserver string, pi *MailPingInformation) (*StartPollingResponse, error) {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return nil, err
	}
	return startPoll(client, pi)
}

func startPoll(rpcClient *rpc.Client, pi *MailPingInformation) (*StartPollingResponse, error) {
	if rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply StartPollingResponse
	err := rpcClient.Call("BackendPolling.Start", &StartPollArgs{MailInfo: pi}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Code != PollingReplyOK {
		return nil, errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return &reply, nil
}

func StopPoll(rpcserver, clientId string) (*PollingResponse, error) {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return nil, err
	}
	return stopPoll(client, clientId)
}

func stopPoll(rpcClient *rpc.Client, clientId string) (*PollingResponse, error) {
	if rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply PollingResponse
	err := rpcClient.Call("BackendPolling.Stop", &StopPollArgs{ClientId: clientId}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Code != PollingReplyOK {
		return nil, errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return &reply, nil
}

func DeferPoll(rpcserver, clientId, token string) (*PollingResponse, error) {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return nil, err
	}
	return deferPoll(client, clientId, token)
}

func deferPoll(rpcClient *rpc.Client, clientId, token string) (*PollingResponse, error) {
	if rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply PollingResponse
	err := rpcClient.Call("BackendPolling.Defer", &DeferPollArgs{ClientId: clientId, StopToken: token}, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}
