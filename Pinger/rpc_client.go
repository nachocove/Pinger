package Pinger

import (
	"net/rpc"
)

func getRpcClient(rpcConfig *RPCServerConfiguration) (*rpc.Client, error) {
	switch {
	case rpcConfig.Protocol == RPCProtocolHTTP:
		return rpc.DialHTTP("tcp", rpcConfig.ConnectString())
	case rpcConfig.Protocol == RPCProtocolUnix:
		return rpc.Dial("unix", rpcConfig.ConnectString())
	}
	panic("Unknown RPC protocol")
}

func StartPoll(rpcConfig *RPCServerConfiguration, pi *MailPingInformation) (*StartPollingResponse, error) {
	rpcClient, err := getRpcClient(rpcConfig)
	if err != nil {
		return nil, err
	}
	var reply StartPollingResponse
	err = rpcClient.Call("BackendPolling.Start", &StartPollArgs{MailInfo: pi}, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

func StopPoll(rpcConfig *RPCServerConfiguration, clientId, clientContext, deviceId string) (*PollingResponse, error) {
	rpcClient, err := getRpcClient(rpcConfig)
	if err != nil {
		return nil, err
	}
	var reply PollingResponse
	args := StopPollArgs{
		ClientId:      clientId,
		ClientContext: clientContext,
		DeviceId:      deviceId,
	}
	err = rpcClient.Call("BackendPolling.Stop", &args, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

func DeferPoll(rpcConfig *RPCServerConfiguration, clientId, clientContext, deviceId string, timeout int64) (*PollingResponse, error) {
	rpcClient, err := getRpcClient(rpcConfig)
	if err != nil {
		return nil, err
	}
	var reply PollingResponse
	args := DeferPollArgs{
		ClientId:      clientId,
		ClientContext: clientContext,
		DeviceId:      deviceId,
		Timeout:       timeout,
	}
	err = rpcClient.Call("BackendPolling.Defer", &args, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

func FindActiveSessions(rpcConfig *RPCServerConfiguration, clientId, clientContext, deviceId string, maxSessions int) (*FindSessionsResponse, error) {
	rpcClient, err := getRpcClient(rpcConfig)
	if err != nil {
		return nil, err
	}
	if rpcClient == nil {
		panic("Can not call deferPoll without rpcClient set")
	}
	var reply FindSessionsResponse
	args := FindSessionsArgs{
		ClientId:      clientId,
		ClientContext: clientContext,
		DeviceId:      deviceId,
		MaxSessions:   maxSessions,
	}
	err = rpcClient.Call("BackendPolling.FindActiveSessions", &args, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

func AliveCheck(rpcConfig *RPCServerConfiguration) (*AliveCheckResponse, error) {
	client, err := getRpcClient(rpcConfig)
	if err != nil {
		return nil, err
	}
	var reply AliveCheckResponse
	err = client.Call("BackendPolling.AliveCheck", &AliveCheckArgs{}, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}
