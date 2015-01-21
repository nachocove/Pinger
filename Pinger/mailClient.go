package Pinger

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"net/rpc"
	"github.com/op/go-logging"
)

// MailClientType the type of the mail client
type MailClientType string

const (
	MailClientExchange MailClientType = "exchange"
)

type MailClient interface {
	Listen(pi* MailPingInformation, wait *sync.WaitGroup) error
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
	mailClient MailClient  // a mail client with the MailClient interface
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
	return pi.startPoll(client)
}

func (pi *MailPingInformation) startPoll(client *rpc.Client) error {
	var reply PollingResponse
	err := client.Call("BackendPolling.Start", &StartPollArgs{MailInfo: pi}, &reply)
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
	return pi.stopPoll(client)
}

func (pi *MailPingInformation) stopPoll(client *rpc.Client) error {
	var reply PollingResponse
	err := client.Call("BackendPolling.Stop", &StopPollArgs{ClientId: pi.deviceInfo.ClientId}, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}

func (pi *MailPingInformation) RestartPoll(rpcserver string) error {
	client, err := rpcClient(rpcserver)
	if err != nil {
		return err
	}
	err = pi.stopPoll(client)
	if err != nil {
		return err
	}
	err = pi.startPoll(client)
	if err != nil {
		return err
	}
	return nil
}

func (pi *MailPingInformation) Start(debug bool, logger *logging.Logger) error {
	client := NewExchangeClient(pi, debug, logger)
	if client == nil {
		return errors.New("Could not create new Mail Client Pinger")
	}
	err := client.Listen(pi, nil)
	if err != nil {
		return err
	}
	pi.mailClient = client
	return nil
}

func (pi *MailPingInformation) Stop(debug bool, logger *logging.Logger) error {
	if pi.mailClient == nil {
		panic("pi.mailClient = nil. Perhaps the mailclient has not be started?")
	}
	pi.mailClient.Action(Stop)
	return nil
}
