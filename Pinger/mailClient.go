package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/op/go-logging"
	"github.com/twinj/uuid"
	"sync"
)

// MailClientType the type of the mail client
type MailClientType string

const (
	MailClientExchange MailClientType = "exchange"
)

type MailClientStatus int

const (
	MailClientStatusPinging = iota
	MailClientStatusError   = iota
)

type MailClient interface {
	LongPoll(wait *sync.WaitGroup) error
	Action(action int) error
	Status() (MailClientStatus, error)
}

type MailPingInformation struct {
	ClientId               string
	Platform               string
	MailServerUrl          string
	MailServerCredentials  string            // json encoded, presumably {"username": <foo>, "password": <bar>}
	Protocol               string            // usually http (is this needed?)
	HttpHeaders            map[string]string // optional
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
	mailClient       MailClient // a mail client with the MailClient interface
	_userCredentials map[string]string
	stopToken        string
}

func (pi *MailPingInformation) status() (MailClientStatus, error) {
	return pi.mailClient.Status()
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

// Validate validate the structure/information to make sure required information exists.
func (pi *MailPingInformation) Validate() bool {
	return (pi.ClientId != "" &&
		pi.MailServerUrl != "" &&
		pi.MailServerCredentials != "" &&
		len(pi.HttpRequestData) > 0 &&
		len(pi.HttpExpectedReply) > 0)
}

func (pi *MailPingInformation) userCredentials() (map[string]string, error) {
	if pi._userCredentials == nil {
		data := make(map[string]string)
		err := json.Unmarshal([]byte(pi.MailServerCredentials), &data)
		if err != nil {
			return nil, err
		}
		pi._userCredentials = data
	}
	return pi._userCredentials, nil
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
	client, err := NewExchangeClient(pi, debug, doStats, logger)
	if err != nil {
		return "", err
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
		return pi.mailClient.Action(Stop)
	}
	return nil
}

func (pi *MailPingInformation) deferPoll(debug bool, logger *logging.Logger) error {
	if pi.mailClient == nil {
		panic("pi.mailClient = nil. Perhaps the mailclient has not be started?")
	}
	logger.Debug("%s: Stopping polls", pi.ClientId)
	pi.mailClient.Action(Stop)
	// TODO Should wait for it to be done
	logger.Debug("%s: Starting polls", pi.ClientId)
	err := pi.mailClient.LongPoll(nil) // MUST NOT BLOCK. Is expected to create a goroutine to do the long poll.
	if err != nil {
		return err
	}
	return nil
}

func (pi *MailPingInformation) validateClientId() error {
	if pi.ClientId == "" {
		return errors.New("Empty client ID is not valid")
	}
	return DefaultPollingContext.config.Aws.validateCognitoId(pi.ClientId)
}
