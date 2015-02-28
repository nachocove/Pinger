package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"github.com/twinj/uuid"
	"strings"
	"sync"
)

type MailClient interface {
	LongPoll(wait *sync.WaitGroup) error
	Action(action PingerCommand) error
	Status() (MailClientStatus, error)
	SelfDelete()
}

const (
	MailClientActiveSync = "ActiveSync"
)

type MailClientStatus int

const (
	MailClientStatusError   = iota
	MailClientStatusPinging = iota
	MailClientStatusStopped = iota
)

const (
	DefaultMaxPollTimeout int64 = 2 * 24 * 60 * 60 * 1000 // 2 days in milliseconds
)

type MailServerCredentials struct {
	Username string
	Password string
}

// MailPingInformation the bag of information we get from the client. None of this is saved in the DB.
type MailPingInformation struct {
	ClientId               string
	ClientContext          string
	Platform               string
	MailServerUrl          string
	MailServerCredentials  MailServerCredentials
	Protocol               string
	HttpHeaders            map[string]string // optional
	HttpRequestData        []byte
	HttpExpectedReply      []byte
	HttpNoChangeReply      []byte
	CommandTerminator      []byte // used by imap
	CommandAcknowledgement []byte // used by imap
	ResponseTimeout        int64  // in milliseconds
	WaitBeforeUse          int64  // in milliseconds
	PushToken              string // platform dependent push token
	PushService            string // APNS, AWS, GCM, etc.
	MaxPollTimeout         int64  // max polling lifetime. Default 2 days.

	// private
	mailClient MailClient // a mail client with the MailClient interface
	stopToken  string
	logger     *logging.Logger
}

func (pi *MailPingInformation) status() (MailClientStatus, error) {
	if pi.mailClient != nil {
		return pi.mailClient.Status()
	} else {
		return MailClientStatusStopped, nil
	}
}
func (pi *MailPingInformation) SelfDelete() {
	pi.logger.Debug("%s@%s: Cleaning up MailPingInformation struct", pi.ClientId, pi.ClientContext)
	pi.ClientId = ""
	pi.ClientContext = ""
	pi.Platform = ""
	pi.MailServerUrl = ""
	pi.MailServerCredentials.Password = ""
	pi.MailServerCredentials.Username = ""
	pi.Protocol = ""
	for k := range pi.HttpHeaders {
		delete(pi.HttpHeaders, k)
	}
	pi.HttpRequestData = nil
	pi.HttpExpectedReply = nil
	pi.HttpNoChangeReply = nil
	pi.CommandTerminator = nil
	pi.CommandAcknowledgement = nil
	pi.PushToken = ""
	pi.PushService = ""
	if pi.mailClient != nil {
		pi.mailClient.SelfDelete()
		pi.mailClient = nil
	}
	pi.stopToken = ""
}

func (pi *MailPingInformation) String() string {
	mailcopy := *pi
	mailcopy.MailServerCredentials.Password = "REDACTED"
	jsonstring, err := json.Marshal(mailcopy)
	if err != nil {
		panic("Could not encode struct")
	}
	if pi.MailServerCredentials.Password == "REDACTED" {
		panic("This should not have happened")
	}
	return string(jsonstring)
}

// Validate validate the structure/information to make sure required information exists.
func (pi *MailPingInformation) Validate() bool {
	return (pi.ClientId != "" &&
		pi.MailServerUrl != "" &&
		pi.MailServerCredentials.Username != "" &&
		pi.MailServerCredentials.Password != "" &&
		len(pi.HttpRequestData) > 0 &&
		len(pi.HttpExpectedReply) > 0)
}

func UserSha256(username string) string {
	h := sha256.New()
	_, err := h.Write([]byte(username))
	if err != nil {
		panic(err.Error())
	}
	md := h.Sum(nil)
	return hex.EncodeToString(md)
}

func (pi *MailPingInformation) createToken() {
	if pi.stopToken == "" {
		uuid.SwitchFormat(uuid.Clean)
		pi.stopToken = uuid.NewV4().String()
	}
}

func (pi *MailPingInformation) validateToken(token string) bool {
	return pi.stopToken == token
}

func (pi *MailPingInformation) start(debug, doStats bool) (string, error) {
	var client MailClient
	var err error

	pi.logger.Debug("%s: Validating clientID", pi.ClientId)
	err = pi.validateClientId()
	if err != nil {
		return "", err
	}

	deviceInfo, err := getDeviceInfo(DefaultPollingContext.dbm, pi.ClientId, pi.logger)
	if err != nil {
		return "", err
	}
	err = deviceInfo.validateClient()
	if err != nil {
		return "", err
	}

	switch {
	case strings.EqualFold(pi.Protocol, MailClientActiveSync):
		client, err = NewExchangeClient(pi, deviceInfo, debug, doStats, pi.logger)
		if err != nil {
			return "", err
		}
	default:
		msg := fmt.Sprintf("%s: Unsupported Mail Protocol %s", pi.ClientId, pi.Protocol)
		pi.logger.Error(msg)
		return "", errors.New(msg)
	}

	if client == nil {
		return "", fmt.Errorf("%s: Could not create new Mail Client Pinger", pi.ClientId)
	}
	pi.logger.Debug("%s: Starting polls", pi.ClientId)
	err = client.LongPoll(nil) // MUST NOT BLOCK. Is expected to create a goroutine to do the long poll.
	if err != nil {
		return "", err
	}
	pi.createToken()
	pi.mailClient = client
	return pi.stopToken, nil
}

func (pi *MailPingInformation) stop(debug bool) error {
	if pi.mailClient != nil {
		pi.logger.Debug("%s: Stopping polls", pi.ClientId)
		return pi.mailClient.Action(PingerStop)
	}
	return nil
}

func (pi *MailPingInformation) deferPoll(timeout int64, debug bool, logger *logging.Logger) error {
	if pi.mailClient != nil {
		logger.Debug("%s: Deferring polls", pi.ClientId)
		if timeout > 0 {
			pi.WaitBeforeUse = timeout
		}
		return pi.mailClient.Action(PingerDefer)
	}
	return fmt.Errorf("Client has stopped. Can not defer")
}

func (pi *MailPingInformation) validateClientId() error {
	if pi.ClientId == "" {
		return errors.New("Empty client ID is not valid")
	}
	return DefaultPollingContext.config.Aws.validateCognitoID(pi.ClientId)
}
