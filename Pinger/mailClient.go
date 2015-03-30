package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/twinj/uuid"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"
)

type MailClientContextType interface {
	deferPoll(timeout int64) error
	stop() error
	validateStopToken(token string) bool
	updateLastContact() error
	Status() (MailClientStatus, error)
	Action(action PingerCommand) error
	getStopToken() string
	getSessionInfo() (*ClientSessionInfo, error)
}

type MailClientContext struct {
	mailClient     MailClient // a mail client with the MailClient interface
	stopToken      string
	logger         *Logging.Logger
	errCh          chan error
	stopAllCh      chan int // closed when client is exiting, so that any sub-routine can exit
	exitCh         chan int // used by MailClient{} to signal it has exited
	command        chan PingerCommand
	lastError      error
	stats          *Utils.StatLogger
	di             *DeviceInfo
	ClientId       string
	ClientContext  string
	DeviceId       string
	Protocol       string
	WaitBeforeUse  int64 // in milliseconds
	MaxPollTimeout int64 // max polling lifetime in milliseconds. Default 2 days.
	wg             sync.WaitGroup
	status         MailClientStatus
	logPrefix      string
}

func (client *MailClientContext) getLogPrefix() string {
	if client.logPrefix == "" {
		client.logPrefix = fmt.Sprintf("%s:%s:%s", client.DeviceId, client.ClientId, client.ClientContext)
	}
	return client.logPrefix
}

type MailClient interface {
	LongPoll(stopCh, exitCh chan int)
	Cleanup()
	sendError(err error)
}

const (
	MailClientActiveSync = "ActiveSync"
	MailClientIMAP       = "IMAP"
)

type MailClientStatus int

const (
	MailClientStatusError       = iota
	MailClientStatusInitialized = iota
	MailClientStatusPinging     = iota
	MailClientStatusDeferred    = iota
	MailClientStatusStopped     = iota
)

func (status MailClientStatus) String() string {
	switch {
	case status == MailClientStatusError:
		return "Error"

	case status == MailClientStatusInitialized:
		return "Initialized"

	case status == MailClientStatusPinging:
		return "Active"

	case status == MailClientStatusDeferred:
		return "Waiting"

	case status == MailClientStatusStopped:
		return "Stopped"
	}
	panic(fmt.Sprintf("Unknown status %d", status))
}

const (
	DefaultMaxPollTimeout int64 = 2 * 24 * 60 * 60 * 1000 // 2 days in milliseconds
)

func NewMailClientContext(dbm *gorp.DbMap, aws AWS.AWSHandler, pi *MailPingInformation, debug, doStats bool, logger *Logging.Logger) (*MailClientContext, error) {
	client := &MailClientContext{
		logger:         logger.Copy(),
		errCh:          make(chan error),
		stopAllCh:      make(chan int),
		exitCh:         make(chan int),
		command:        make(chan PingerCommand, 10),
		stats:          nil,
		status:         MailClientStatusInitialized,
		ClientId:       pi.ClientId,
		ClientContext:  pi.ClientContext,
		DeviceId:       pi.DeviceId,
		Protocol:       pi.Protocol,
		WaitBeforeUse:  pi.WaitBeforeUse,
		MaxPollTimeout: pi.MaxPollTimeout,
	}
	client.logger.SetCallDepth(1)

	di, err := newDeviceInfoPI(dbm, aws, pi, logger)
	if err != nil {
		return nil, err
	}
	if di == nil {
		panic("Could not create device info")
	}
	client.Debug("Validating client info")
	err = di.validateClient()
	if err != nil {
		return nil, err
	}
	client.di = di
	if doStats {
		client.stats = Utils.NewStatLogger(client.stopAllCh, logger, false)
	}
	var mailclient MailClient

	switch {
	case strings.EqualFold(client.Protocol, MailClientActiveSync):
		mailclient, err = NewExchangeClient(di, pi, &client.wg, client.errCh, client.stopAllCh, debug, logger)
		if err != nil {
			return nil, err
		}
		//	case strings.EqualFold(client.Protocol, MailClientIMAP):
		//		mailclient, err = NewIMAPClient(client, debug, client.logger)
		//		if err != nil {
		//			return nil, err
		//		}
	default:
		client.Error("Unsupported Mail Protocol %s", client.Protocol)
		return nil, fmt.Errorf("%s: Unsupported Mail Protocol %s", pi.getLogPrefix(), client.Protocol)
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
	client.logger.Debug(fmt.Sprintf("%s: %s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Info(format string, args ...interface{}) {
	client.logger.Info(fmt.Sprintf("%s: %s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Error(format string, args ...interface{}) {
	client.logger.Error(fmt.Sprintf("%s: %s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Warning(format string, args ...interface{}) {
	client.logger.Warning(fmt.Sprintf("%s: %s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Status() (MailClientStatus, error) {
	if client.lastError != nil {
		return MailClientStatusError, client.lastError
	}
	return client.status, nil
}

func (client *MailClientContext) cleanup() {
	client.Debug("Cleaning up MailClientContext struct")
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
	defer Utils.RecoverCrash(client.logger)
	defer func() {
		client.status = MailClientStatusStopped
		client.Debug("Waiting for subroutines to finish")
		client.wg.Wait()
		client.Debug("Cleaning up")
		client.cleanup()
	}()

	client.status = MailClientStatusDeferred
	deferTime := time.Duration(client.WaitBeforeUse) * time.Millisecond
	client.Debug("Starting deferTimer for %s", deferTime)
	deferTimer := time.NewTimer(deferTime)
	defer deferTimer.Stop()
	if client.stats != nil {
		go client.stats.TallyResponseTimes()
	}
	maxPollTime := time.Duration(client.MaxPollTimeout) * time.Millisecond
	client.Debug("Setting maxPollTimer for %s", maxPollTime)
	maxPollTimer := time.NewTimer(maxPollTime)
	var longPollStopCh chan int
	for {
		select {
		case <-maxPollTimer.C:
			client.Debug("maxPollTimer expired. Stopping everything.")
			client.status = MailClientStatusStopped
			return

		case <-deferTimer.C:
			// defer timer has timed out. Now it's time to do something
			client.Info("DeferTimer expired. Starting Polling.")
			maxPollTimer.Reset(maxPollTime)
			// launch the longpoll and wait for it to exit
			longPollStopCh = make(chan int)
			client.wg.Add(1)
			go client.mailClient.LongPoll(longPollStopCh, client.exitCh)
			client.status = MailClientStatusPinging

		case <-client.exitCh:
			// the mailClient.LongPoll has indicated that it has exited. Clean up.
			client.status = MailClientStatusStopped
			client.Info("LongPoll exited. Stopping.")
			client.Action(PingerStop)

		case err := <-client.errCh:
			// the mailClient.LongPoll has thrown an error. note it.
			client.status = MailClientStatusError
			client.Warning("Error thrown. Stopping.")
			client.lastError = err

		case cmd := <-client.command:
			switch {
			case cmd == PingerStop:
				client.status = MailClientStatusStopped
				close(client.stopAllCh) // tell all goroutines listening on this channel that they can stop now.
				client.Debug("got 'PingerStop' command")
				return

			case cmd == PingerDefer:
				if longPollStopCh != nil {
					longPollStopCh <- 1
					longPollStopCh = nil
				}
				deferTime := time.Duration(client.WaitBeforeUse) * time.Millisecond
				client.Debug("reStarting deferTimer for %s", deferTime)
				deferTimer.Stop()
				deferTimer.Reset(deferTime)
				maxPollTimer.Stop()
				client.status = MailClientStatusDeferred

			default:
				client.Error("Unknown command %d", cmd)
				continue

			}
		}
	}
}

func sendError(errCh chan error, err error, logger *Logging.Logger) {
	_, fn, line, _ := runtime.Caller(1)
	logger.Error("%s/%s:%d %s", path.Base(path.Dir(fn)), path.Base(fn), line, err)
	errCh <- err
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
			client.WaitBeforeUse = timeout
		}
		return client.Action(PingerDefer)
	}
	return fmt.Errorf("Client has stopped. Can not defer")
}

func (client *MailClientContext) updateLastContact() error {
	return client.di.updateLastContact()
}

func (client *MailClientContext) getStopToken() string {
	return client.stopToken
}

type ClientSessionInfo struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Status        MailClientStatus
	Error         string
}

func (client *MailClientContext) sessionInfo() *ClientSessionInfo {
	status, err := client.Status()
	info := ClientSessionInfo{
		ClientId:      client.ClientId,
		ClientContext: client.ClientContext,
		DeviceId:      client.DeviceId,
		Status:        status,
	}
	if err != nil {
		info.Error = err.Error()
	}
	return &info
}

func (client *MailClientContext) getSessionInfo() (*ClientSessionInfo, error) {
	switch {
	case client.mailClient == nil:
		return nil, fmt.Errorf("Entry has no active client")

	case client.ClientId == "" || client.ClientContext == "" || client.DeviceId == "":
		return nil, fmt.Errorf("entry has been cleaned up.")
	}
	return client.sessionInfo(), nil
}
