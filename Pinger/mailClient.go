package Pinger

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/twinj/uuid"
	"path"
	"runtime"
	"strings"
	"sync"
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
	DeviceId              string
	Platform              string
	MailServerUrl         string
	MailServerCredentials struct {
		Username string
		Password string
	}
	Protocol               string
	HttpHeaders            map[string]string // optional
	RequestData            []byte
	ExpectedReply          []byte
	NoChangeReply          []byte
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

func (pi *MailPingInformation) String() string {
	return fmt.Sprintf("%s: NoChangeReply:%s, RequestData:%s, ExpectedReply:%s",
		pi.getLogPrefix(),
		base64.StdEncoding.EncodeToString(pi.NoChangeReply),
		base64.StdEncoding.EncodeToString(pi.RequestData),
		base64.StdEncoding.EncodeToString(pi.ExpectedReply))
}

func (pi *MailPingInformation) cleanup() {
	pi.ClientId = ""
	pi.ClientContext = ""
	pi.DeviceId = ""
	pi.Platform = ""
	pi.MailServerUrl = ""
	pi.MailServerCredentials.Password = ""
	pi.MailServerCredentials.Username = ""
	pi.Protocol = ""
	for k := range pi.HttpHeaders {
		delete(pi.HttpHeaders, k)
	}
	pi.RequestData = nil
	pi.ExpectedReply = nil
	pi.NoChangeReply = nil
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
	if pi.ClientId == "" || pi.MailServerUrl == "" {
		return false
	}
	switch {
	case pi.Protocol == MailClientActiveSync:
		if len(pi.RequestData) <= 0 || len(pi.HttpHeaders) <= 0 {
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
		pi.logPrefix = fmt.Sprintf("%s:%s:%s", pi.DeviceId, pi.ClientId, pi.ClientContext)
	}
	return pi.logPrefix
}

type MailClientContext struct {
	mailClient MailClient // a mail client with the MailClient interface
	stopToken  string
	logger     *Logging.Logger
	errCh      chan error
	stopAllCh  chan int // closed when client is exiting, so that any sub-routine can exit
	exitCh     chan int // used by MailClient{} to signal it has exited
	command    chan PingerCommand
	lastError  error
	stats      *Utils.StatLogger
	di         *DeviceInfo
	pi         *MailPingInformation
	wg         sync.WaitGroup
}

func NewMailClientContext(pi *MailPingInformation, di *DeviceInfo, debug, doStats bool, logger *Logging.Logger) (*MailClientContext, error) {
	client := &MailClientContext{
		logger:    logger.Copy(),
		errCh:     make(chan error),
		stopAllCh: make(chan int),
		exitCh:    make(chan int),
		command:   make(chan PingerCommand, 10),
		stats:     nil,
		pi:        pi,
	}
	client.logger.SetCallDepth(1)
	client.Debug("Validating clientID")
	deviceInfo, err := getDeviceInfo(DefaultPollingContext.dbm, pi.ClientId, pi.ClientContext, pi.DeviceId, client.logger)
	if err != nil {
		return nil, err
	}
	err = deviceInfo.validateClient()
	if err != nil {
		return nil, err
	}
	client.di = deviceInfo
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
		client.Error("Unsupported Mail Protocol %s", client.pi.Protocol)
		return nil, fmt.Errorf("%s: Unsupported Mail Protocol %s", pi.getLogPrefix(), client.pi.Protocol)
	}

	if mailclient == nil {
		return nil, fmt.Errorf("%s: Could not create new Mail Client Pinger", pi.getLogPrefix())
	}
	client.Debug("Starting polls for %s", pi.String())
	uuid.SwitchFormat(uuid.Clean)
	client.stopToken = uuid.NewV4().String()
	client.mailClient = mailclient
	go client.start()
	return client, nil
}

func (client *MailClientContext) Debug(format string, args ...interface{}) {
	client.logger.Debug(fmt.Sprintf("%s: %s", client.pi.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Info(format string, args ...interface{}) {
	client.logger.Info(fmt.Sprintf("%s: %s", client.pi.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Error(format string, args ...interface{}) {
	client.logger.Error(fmt.Sprintf("%s: %s", client.pi.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Warning(format string, args ...interface{}) {
	client.logger.Warning(fmt.Sprintf("%s: %s", client.pi.getLogPrefix(), format), args...)
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
	client.Debug("Cleaning up MailClientContext struct")
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
	defer func() {
		client.Debug("Waiting for subroutines to finish")
		client.wg.Wait()
		client.Debug("Cleaning up")
		client.cleanup()
	}()

	deferTime := time.Duration(client.pi.WaitBeforeUse) * time.Millisecond
	client.Debug("Starting deferTimer for %s", deferTime)
	deferTimer := time.NewTimer(deferTime)
	defer deferTimer.Stop()
	if client.stats != nil {
		go client.stats.TallyResponseTimes()
	}
	maxPollTime := time.Duration(client.pi.MaxPollTimeout) * time.Millisecond
	client.Debug("Setting maxPollTimer for %s", maxPollTime)
	maxPollTimer := time.NewTimer(maxPollTime)
	var longPollStopCh chan int
	for {
		select {
		case <-maxPollTimer.C:
			client.Debug("maxPollTimer expired. Stopping everything.")
			return

		case <-deferTimer.C:
			// defer timer has timed out. Now it's time to do something
			client.Debug("DeferTimer expired. Starting Polling.")
			maxPollTimer.Reset(maxPollTime)
			// launch the longpoll and wait for it to exit
			longPollStopCh = make(chan int)
			client.wg.Add(1)
			go client.mailClient.LongPoll(longPollStopCh, client.exitCh)

		case <-client.exitCh:
			// the mailClient.LongPoll has indicated that it has exited. Clean up.
			client.Debug("LongPoll exited. Stopping.")
			client.Action(PingerStop)

		case err := <-client.errCh:
			// the mailClient.LongPoll has thrown an error. note it.
			client.Debug("Error thrown. Stopping.")
			client.lastError = err

		case cmd := <-client.command:
			switch {
			case cmd == PingerStop:
				close(client.stopAllCh) // tell all goroutines listening on this channel that they can stop now.
				client.Debug("got 'PingerStop' command")
				return

			case cmd == PingerDefer:
				if longPollStopCh != nil {
					longPollStopCh <- 1
					longPollStopCh = nil
				}
				deferTime := time.Duration(client.pi.WaitBeforeUse) * time.Millisecond
				client.Debug("reStarting deferTimer for %s", deferTime)
				deferTimer.Stop()
				deferTimer.Reset(deferTime)
				maxPollTimer.Stop()

			default:
				client.Error("Unknown command %d", cmd)
				continue

			}
		}
	}
}

func (client *MailClientContext) sendError(err error) {
	_, fn, line, _ := runtime.Caller(1)
	client.Error("%s/%s:%d %s", path.Base(path.Dir(fn)), path.Base(fn), line, err)
	client.errCh <- err
}

func (client *MailClientContext) Action(action PingerCommand) error {
	client.command <- action
	return nil
}

func (client *MailClientContext) stop() error {
	if client.mailClient != nil {
		client.Debug("Stopping polls")
		return client.Action(PingerStop)
	}
	return nil
}

func (client *MailClientContext) deferPoll(timeout int64) error {
	if client.mailClient != nil {
		client.Debug("Deferring polls")
		if timeout > 0 {
			client.pi.WaitBeforeUse = timeout
		}
		return client.Action(PingerDefer)
	}
	return fmt.Errorf("Client has stopped. Can not defer")
}
