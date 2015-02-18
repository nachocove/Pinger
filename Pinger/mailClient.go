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

const (
	MailClientActivesync = "ActiveSync"
)

type MailClientStatus int

const (
	MailClientStatusPinging = iota
	MailClientStatusError   = iota
)

type MailClient interface {
	LongPoll(wait *sync.WaitGroup) error
	Action(action PingerCommand) error
	Status() (MailClientStatus, error)
}

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
	ResponseTimeout        int64  // in seconds
	WaitBeforeUse          int64  // in seconds
	PushToken              string // platform dependent push token
	PushService            string // APNS, AWS, GCM, etc.

	// private
	mailClient       MailClient // a mail client with the MailClient interface
	_userCredentials map[string]string
	stopToken        string
}

func (pi *MailPingInformation) status() (MailClientStatus, error) {
	return pi.mailClient.Status()
}

func (pi *MailPingInformation) String() string {
	mailcopy := *pi
	mailcopy.MailServerCredentials.Password = "REDACTED"
	jsonstring, err := json.Marshal(mailcopy)
	if err != nil {
		panic("Could not encode struct")
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

func (pi *MailPingInformation) validateStopToken(token string) bool {
	return pi.stopToken == token
}

func (pi *MailPingInformation) start(debug, doStats bool, logger *logging.Logger) (string, error) {
	var client MailClient
	var err error

	switch {
	case strings.EqualFold(pi.Protocol, MailClientActivesync):
		client, err = NewExchangeClient(pi, debug, doStats, logger)
		if err != nil {
			return "", err
		}
	default:
		msg := fmt.Sprintf("Unsupported Mail Protocol %s", pi.Protocol)
		logger.Error(msg)
		return "", errors.New(msg)
	}

	if client == nil {
		return "", errors.New("Could not create new Mail Client Pinger")
	}
	logger.Debug("%s: Starting polls", pi.ClientId)
	err = client.LongPoll(nil) // MUST NOT BLOCK. Is expected to create a goroutine to do the long poll.
	if err != nil {
		return "", err
	}
	pi.createToken()
	pi.mailClient = client
	return pi.stopToken, nil
}

func (pi *MailPingInformation) stop(debug bool, logger *logging.Logger) error {
	if pi.mailClient == nil {
		logger.Debug("%s: Stopping polls", pi.ClientId)
		return pi.mailClient.Action(PingerStop)
	}
	return nil
}

func (pi *MailPingInformation) deferPoll(timeout int64, debug bool, logger *logging.Logger) error {
	if pi.mailClient == nil {
		panic("pi.mailClient = nil. Perhaps the mailclient has not been started?")
	}
	logger.Debug("%s: Deferring polls", pi.ClientId)
	if timeout > 0 {
		pi.WaitBeforeUse = timeout
	}
	return pi.mailClient.Action(PingerDefer)
}

func (pi *MailPingInformation) validateClientId() error {
	if pi.ClientId == "" {
		return errors.New("Empty client ID is not valid")
	}
	return DefaultPollingContext.config.Aws.validateCognitoId(pi.ClientId)
}
