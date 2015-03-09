package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
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
	LongPoll(stopCh, exitCh chan int)
	Cleanup()
}

const (
	MailClientActiveSync = "ActiveSync"
	MailClientIMAP       = "IMAP"
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
	MaxPollTimeout         int64  // max polling lifetime in milliseconds. Default 2 days.
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
	pi.OSVersion = ""
	pi.AppBuildNumber = ""
	pi.AppBuildVersion = ""
}

// Validate validate the structure/information to make sure required information exists.
func (pi *MailPingInformation) Validate() bool {
	if pi.ClientId == "" ||	pi.MailServerUrl == "" {
		return false
	}
	switch {
	case pi.Protocol == MailClientActiveSync:
		if len(pi.HttpRequestData) <= 0 || len(pi.HttpHeaders) <= 0 {
			return false
		}
	case pi.Protocol == MailClientIMAP:
		// not yet supported 
		return false
		
	default:
		// unknown protocols are never supported
		return false
	}
	return true
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
	stopAllCh  chan int // closed when client is exiting, so that any sub-routine can exit
	exitCh     chan int // used by MailClient{} to signal it has exited
	command    chan PingerCommand
	lastError  error
	stats      *Utils.StatLogger
	di         *DeviceInfo
	pi         *MailPingInformation
}

func NewMailClientContext(pi *MailPingInformation, di *DeviceInfo, debug, doStats bool, logger *logging.Logger) (*MailClientContext, error) {
	client := &MailClientContext{
		logger:    logger,
		errCh:     make(chan error),
		stopAllCh: make(chan int),
		exitCh:    make(chan int),
		command:   make(chan PingerCommand, 10),
		stats:     nil,
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
		client.stats = Utils.NewStatLogger(client.stopAllCh, logger, false)
	}
	var mailclient MailClient

	switch {
	case strings.EqualFold(client.pi.Protocol, MailClientActiveSync):
		mailclient, err = NewExchangeClient(client, debug, client.logger)
		if err != nil {
			return nil, err
		}
		//	case strings.EqualFold(client.pi.Protocol, MailClientIMAP):
		//		mailclient, err = NewIMAPClient(client, debug, client.logger)
		//		if err != nil {
		//			return nil, err
		//		}
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
	
	// tell Garbage collection to run. Might not free/remove all instances we free'd above,
	// but it's worth the effort.
	go runtime.GC()
}

func (pi *MailPingInformation) String() string {
	// let's try to be safe and not print this at all. There's a number of fields we don't 
	// want to be printed (Password is one, but also an HttpHeader might give away user info!),
	// So let's just not.
	return "<MailPingInformation; redacted completely>" 
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
	return token != "" && strings.EqualFold(client.stopToken, token)
}

func (client *MailClientContext) start() {
	defer recoverCrash(client.logger)
	defer client.cleanup()

	deferTime := time.Duration(client.pi.WaitBeforeUse) * time.Millisecond
	client.logger.Debug("%s: Starting deferTimer for %s", client.pi.getLogPrefix(), deferTime)
	deferTimer := time.NewTimer(deferTime)
	defer deferTimer.Stop()
	if client.stats != nil {
		go client.stats.TallyResponseTimes()
	}
	maxPollTime := time.Duration(client.pi.MaxPollTimeout) * time.Millisecond
	client.logger.Debug("%s: Setting maxPollTimer for %s", client.pi.getLogPrefix(), maxPollTime)
	maxPollTimer := time.NewTimer(maxPollTime)
	var longPollStopCh chan int
	for {
		client.logger.Debug("%s: top of for loop", client.pi.getLogPrefix())
		select {
		case <-maxPollTimer.C:
			client.logger.Debug("maxPollTimer expired. Stopping everything.")
			return
			
		case <-deferTimer.C:
			// defer timer has timed out. Now it's time to do something
			client.logger.Debug("%s: DeferTimer expired. Starting Polling.", client.pi.getLogPrefix())
			maxPollTimer.Reset(maxPollTime)
			// launch the longpoll and wait for it to exit
			longPollStopCh = make(chan int)
			go client.mailClient.LongPoll(longPollStopCh, client.exitCh)

		case <-client.exitCh:
			// the mailClient.LongPoll has indicated that it has exited. Clean up.
			client.logger.Debug("%s: LongPoll exited. Stopping.", client.pi.getLogPrefix())
			client.Action(PingerStop)

		case err := <-client.errCh:
			// the mailClient.LongPoll has thrown an error. note it.
			client.logger.Debug("%s: Error thrown. Stopping.", client.pi.getLogPrefix())
			client.lastError = err

		case cmd := <-client.command:
			switch {
			case cmd == PingerStop:
				close(client.stopAllCh) // tell all goroutines listening on this channel that they can stop now.
				client.logger.Debug("%s: got 'PingerStop' command", client.pi.getLogPrefix())
				return

			case cmd == PingerDefer:
				if longPollStopCh != nil {
					longPollStopCh<-1
					longPollStopCh = nil
				}
				deferTime := time.Duration(client.pi.WaitBeforeUse) * time.Millisecond
				client.logger.Debug("%s: reStarting deferTimer for %s", client.pi.getLogPrefix(), deferTime)
				deferTimer.Stop()
				deferTimer.Reset(deferTime)
				maxPollTimer.Stop()

			default:
				client.logger.Error("%s: Unknown command %d", client.pi.getLogPrefix(), cmd)
				continue

			}
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
