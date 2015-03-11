package Pinger

import (
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
	return &reply, nil
}

func StopPoll(rpcserver, clientId, clientContext, deviceId, token string) (*PollingResponse, error) {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return nil, err
	}
	return stopPoll(client, clientId, clientContext, deviceId, token)
}

func stopPoll(rpcClient *rpc.Client, clientId, clientContext, deviceId, token string) (*PollingResponse, error) {
	if rpcClient == nil {
		panic("Can not call stopPoll without rpcClient set")
	}
	var reply PollingResponse
	err := rpcClient.Call("BackendPolling.Stop", &StopPollArgs{ClientId: clientId, StopToken: token}, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

func DeferPoll(rpcserver, clientId, clientContext, deviceId string, timeout int64, token string) (*PollingResponse, error) {
	client, err := getRpcClient(rpcserver)
	if err != nil {
		return nil, err
	}
	return deferPoll(client, clientId, clientContext, deviceId, timeout, token)
}

func deferPoll(rpcClient *rpc.Client, clientId, clientContext, deviceId string, timeout int64, token string) (*PollingResponse, error) {
	if rpcClient == nil {
		panic("Can not call deferPoll without rpcClient set")
	}
	var reply PollingResponse
	err := rpcClient.Call("BackendPolling.Defer", &DeferPollArgs{ClientId: clientId, Timeout: timeout, StopToken: token}, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}
