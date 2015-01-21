package Pinger

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/rpc"
	"sync"
)

// MailServerType the type of the mail server
type MailServerType int

const (
	// MailServerUnknown an unknown mail server
	MailServerUnknown MailServerType = iota
	// MailServerExchange Exchange by Microsoft
	MailServerExchange MailServerType = iota
	// MailServerHotmail hosted hotmail domain
	MailServerHotmail MailServerType = iota
)

var mailServers = [...]string{
	"UNKNOWN",
	"EXCHANGE",
	"HOTMAIL",
}

func (mailServer MailServerType) String() string {
	return mailServers[mailServer]
}

type MailServer interface {
	Listen(wait *sync.WaitGroup) error
	Action(action int) error
}

type MailPingInformation struct {
	ClientId               string
	Platform               string
	MailServerUrl          string
	MailServerCredentials  string // json encoded, presumably {"username": <foo>, "password": <bar>}
	Protocol               string // usually http (is this needed?)
	HttpHeaders            string // optional
	HttpRequestData        []byte
	HttpExpectedReply      []byte
	HttpNoChangeReply      []byte
	CommandTerminator      []byte
	CommandAcknowledgement []byte
	ResponseTimeout        int64
	WaitBeforeUse          int64
	PushToken              string
	PushService            string // APNS, AWS, GCM, etc.

	// private
	deviceInfo *DeviceInfo
}

func (pi *MailPingInformation) String() string {
	mailcopy := *pi
	mailcopy.MailServerCredentials = "REDACTED"
	jsonstring, err := json.Marshal(mailcopy)
	if err != nil {
		panic("Could not encode struct")
	}
	return string(jsonstring)
}
func (pi *MailPingInformation) SetDeviceInfo(di *DeviceInfo) {
	pi.deviceInfo = di
}

func (pi *MailPingInformation) Validate() bool {
	return (pi.ClientId != "" &&
		pi.MailServerUrl != "" &&
		pi.MailServerCredentials != "" &&
		len(pi.HttpRequestData) > 0 &&
		len(pi.HttpExpectedReply) > 0)
}

func rpcClient(rpcserver string) (*rpc.Client, error) {
	// TODO Need to figure out if we can cache the client, so we don't have to constantly reopen it
	return rpc.DialHTTP("tcp", rpcserver)
}
func (pi *MailPingInformation) StartPoll(rpcserver string) error {
	client, err := rpcClient(rpcserver)
	if err != nil {
		return err
	}
	args := &StartPollArgs{
		Device:   pi.deviceInfo,
		MailInfo: pi,
	}
	var reply PollingResponse
	err = client.Call("BackendPolling.Start", args, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}

func (pi *MailPingInformation) StopPoll(rpcserver string) error {
	client, err := rpcClient(rpcserver)
	if err != nil {
		return err
	}
	args := &StopPollArgs{
		ClientId: pi.deviceInfo.ClientId,
	}
	var reply PollingResponse
	err = client.Call("BackendPolling.Stop", args, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}
