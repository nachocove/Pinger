package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/op/go-logging"
	"github.com/twinj/uuid"
	"path"
	"runtime"
	"strings"
	"time"
)

type MailClient interface {
	LongPoll(chan int)
	Cleanup()
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

// MailPingInformation the bag of information we get from the client. None of this is saved in the DB.
type MailPingInformation struct {
	ClientId              string
	ClientContext         string
	Platform              string
	MailServerUrl         string
	MailServerCredentials struct {
		Username string
		Password string
	}
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
	OSVersion              string
	AppBuildVersion        string
	AppBuildNumber         string

	logPrefix string
}

func (pi *MailPingInformation) cleanup() {
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

func (pi *MailPingInformation) getLogPrefix() string {
	if pi.logPrefix == "" {
		pi.logPrefix = fmt.Sprintf("%s@%s", pi.ClientId, pi.ClientContext)
	}
	return pi.logPrefix
}

type MailClientContext struct {
	mailClient MailClient // a mail client with the MailClient interface
	stopToken  string
	logger     *logging.Logger
	errCh      chan error
	stopCh     chan int
	command    chan PingerCommand
	lastError  error
	stats      *Utils.StatLogger
	di         *DeviceInfo
	pi         *MailPingInformation
}

func NewMailClientContext(pi *MailPingInformation, di *DeviceInfo, debug, doStats bool, logger *logging.Logger) (*MailClientContext, error) {
	client := &MailClientContext{
		logger:  logger,
		errCh:   make(chan error),
		stopCh:  make(chan int),
		command: make(chan PingerCommand, 10),
		stats:   nil,
	}
	logger.Debug("%s: Validating clientID", pi.ClientId)
	deviceInfo, err := getDeviceInfo(DefaultPollingContext.dbm, pi.ClientId, client.logger)
	if err != nil {
		return nil, err
	}
	err = deviceInfo.validateClient()
	if err != nil {
		return nil, err
	}
	client.di = deviceInfo
	client.pi = pi
	if doStats {
		client.stats = Utils.NewStatLogger(client.stopCh, logger, false)
	}
	var mailclient MailClient

	switch {
	case strings.EqualFold(client.pi.Protocol, MailClientActiveSync):
		mailclient, err = NewExchangeClient(client, debug, client.logger)
		if err != nil {
			return nil, err
		}
	default:
		msg := fmt.Sprintf("%s: Unsupported Mail Protocol %s", client.pi.ClientId, client.pi.Protocol)
		client.logger.Error(msg)
		return nil, errors.New(msg)
	}

	if mailclient == nil {
		return nil, fmt.Errorf("%s: Could not create new Mail Client Pinger", client.pi.ClientId)
	}
	client.logger.Debug("%s: Starting polls", client.pi.ClientId)
	uuid.SwitchFormat(uuid.Clean)
	client.stopToken = uuid.NewV4().String()
	client.mailClient = mailclient
	go client.start()
	return client, nil
}

func (client *MailClientContext) Status() (MailClientStatus, error) {
	if client.lastError != nil {
		return MailClientStatusError, client.lastError
	}
	if client.mailClient != nil {
		return MailClientStatusPinging, nil
	} else {
		return MailClientStatusStopped, nil
	}
}
func (client *MailClientContext) cleanup() {
	client.logger.Debug("%s: Cleaning up MailClientContext struct", client.pi.getLogPrefix())
	if client.pi != nil {
		client.pi.cleanup()
		client.pi = nil
	}
	if client.di != nil {
		client.di.cleanup()
		client.di = nil // let garbage collector handle it.
	}
	if client.mailClient != nil {
		client.mailClient.Cleanup()
		client.mailClient = nil
	}
	client.stopToken = ""
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

func UserSha256(username string) string {
	h := sha256.New()
	_, err := h.Write([]byte(username))
	if err != nil {
		panic(err.Error())
	}
	md := h.Sum(nil)
	return hex.EncodeToString(md)
}

func (client *MailClientContext) validateStopToken(token string) bool {
	client.logger.Debug("Comparing token: should be '%s', is '%s'", client.stopToken, token)
	return token != "" && strings.EqualFold(client.stopToken, token)
}

func (client *MailClientContext) start() {
	defer recoverCrash(client.logger)
	defer client.cleanup()

	client.logger.Debug("%s: Starting deferTimer for %d msecs", client.pi.getLogPrefix(), client.pi.WaitBeforeUse)
	deferTimer := time.NewTimer(time.Duration(client.pi.WaitBeforeUse) * time.Millisecond)
	defer deferTimer.Stop()
	if client.stats != nil {
		go client.stats.TallyResponseTimes()
	}
	client.logger.Debug("%s: Setting maxPollTimer for %d msecs", client.pi.getLogPrefix(), client.pi.WaitBeforeUse)
	maxPollTime := time.Duration(client.pi.MaxPollTimeout) * time.Millisecond
	maxPollTimer := time.NewTimer(maxPollTime)
	maxPollTimer.Stop()
	defer maxPollTimer.Stop()
	exitCh := make(chan int)
	clients := 0

	for {
		select {
		case <-deferTimer.C:
			client.logger.Debug("%s: DeferTimer expired. Starting Polling (clients %d).", client.pi.getLogPrefix(), clients)
			maxPollTimer.Reset(maxPollTime)
			clients ++
			go client.mailClient.LongPoll(exitCh)
			
		case <-exitCh:
			clients --
			client.logger.Debug("%s: LongPoll exited. Stopping (%d).", client.pi.getLogPrefix(), clients)
			client.Action(PingerStop)

		case err := <-client.errCh:
			client.logger.Debug("%s: Error thrown. Stopping.", client.pi.getLogPrefix())
			client.lastError = err
			client.Action(PingerStop)

		case cmd := <-client.command:
			switch {
			case cmd == PingerStop:
				close(client.stopCh) // tell all goroutines listening on this channel that they can stop now.
				client.logger.Debug("%s: got 'PingerStop' command", client.pi.getLogPrefix())
				return

			case cmd == PingerDefer:
				client.logger.Debug("%s: reStarting deferTimer for %d msecs", client.pi.getLogPrefix(), client.pi.WaitBeforeUse)
				client.Action(PingerStop)
				deferTimer.Reset(time.Duration(client.pi.WaitBeforeUse) * time.Millisecond)
				maxPollTimer.Stop()

			default:
				client.logger.Error("%s: Unknown command %d", client.pi.getLogPrefix(), cmd)
				continue

			}
		case <-maxPollTimer.C:
			client.logger.Debug("maxPollTimer expired. Stopping everything.")
			return
		}
	}
}

func (client *MailClientContext) sendError(err error) {
	_, fn, line, _ := runtime.Caller(1)
	client.logger.Error("%s: %s/%s:%d %s", client.pi.getLogPrefix(), path.Base(path.Dir(fn)), path.Base(fn), line, err)
	client.errCh <- err
}

func (client *MailClientContext) Action(action PingerCommand) error {
	client.command <- action
	return nil
}

func (client *MailClientContext) stop() error {
	if client.mailClient != nil {
		client.logger.Debug("%s: Stopping polls", client.pi.getLogPrefix())
		return client.Action(PingerStop)
	}
	return nil
}

func (client *MailClientContext) deferPoll(timeout int64) error {
	if client.mailClient != nil {
		client.logger.Debug("%s: Deferring polls", client.pi.getLogPrefix())
		if timeout > 0 {
			client.pi.WaitBeforeUse = timeout
		}
		return client.Action(PingerDefer)
	}
	return fmt.Errorf("Client has stopped. Can not defer")
}
