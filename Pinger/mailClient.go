package Pinger

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"net/rpc"
	"sync"
	"encoding/hex"
	"crypto/sha256"
)

// MailClientType the type of the mail client
type MailClientType string

const (
	MailClientExchange MailClientType = "exchange"
)

type MailClient interface {
	LongPoll(wait *sync.WaitGroup) error
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
	deviceInfo      *DeviceInfo
	mailClient      MailClient // a mail client with the MailClient interface
	rpcClient       *rpc.Client
	userCredentials map[string]string
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

func (pi *MailPingInformation) getRpcClient(rpcserver string) error {
	var err error
	pi.rpcClient, err = rpc.DialHTTP("tcp", rpcserver)
	return err
}
func (pi *MailPingInformation) StartPoll(rpcserver string) error {
	err := pi.getRpcClient(rpcserver)
	if err != nil {
		return err
	}
	return pi.startPoll()
}

func (pi *MailPingInformation) startPoll() error {
	if pi.rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply PollingResponse
	err := pi.rpcClient.Call("BackendPolling.Start", &StartPollArgs{MailInfo: pi}, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}

func (pi *MailPingInformation) StopPoll(rpcserver string) error {
	err := pi.getRpcClient(rpcserver)
	if err != nil {
		return err
	}
	return pi.stopPoll()
}

func (pi *MailPingInformation) stopPoll() error {
	if pi.rpcClient == nil {
		panic("Can not call startPoll without rpcClient set")
	}
	var reply PollingResponse
	err := pi.rpcClient.Call("BackendPolling.Stop", &StopPollArgs{ClientId: pi.deviceInfo.ClientId}, &reply)
	if err != nil {
		return err
	}
	if reply.Code != PollingReplyOK {
		return errors.New(fmt.Sprintf("RPC server responded with %d:%s", reply.Code, reply.Message))
	}
	return nil
}

func (pi *MailPingInformation) RestartPoll(rpcserver string) error {
	err := pi.getRpcClient(rpcserver)
	if err != nil {
		return err
	}
	err = pi.stopPoll()
	if err != nil {
		return err
	}
	err = pi.startPoll()
	if err != nil {
		return err
	}
	return nil
}

func (pi *MailPingInformation) Start(debug bool, logger *logging.Logger) error {
	client, err := NewExchangeClient(pi, debug, logger)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("Could not create new Mail Client Pinger")
	}
	err = client.LongPoll(nil) // MUST NOT BLOCK. Is expected to create a goroutine to do the long poll.
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

func (pi *MailPingInformation) UserCredentials() (map[string]string, error) {
	if pi.userCredentials == nil {
		data := make(map[string]string)
		err := json.Unmarshal([]byte(pi.MailServerCredentials), &data)
		if err != nil {
			return nil, err
		}
		pi.userCredentials = data
	}
	return pi.userCredentials, nil
}

func (pi *MailPingInformation) UserSha256(username string) string {
	userCreds, err := pi.UserCredentials()
	if err != nil {
		panic(err.Error())
	}
	h := sha256.New()
	_, err = h.Write([]byte(userCreds["Username"]))
	if err != nil {
		panic(err.Error())
	}
	md := h.Sum(nil)
	return hex.EncodeToString(md)
}