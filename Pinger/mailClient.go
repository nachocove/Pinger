package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/looplab/fsm"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"
)

type MailClientContextType interface {
	deferPoll(timeout int64)
	stop()
	updateLastContact() error
	Status() (MailClientStatus, error)
	setStatus(MailClientStatus, error)
	Action(action PingerCommand) error
	getSessionInfo() (*ClientSessionInfo, error)
}

type MailClientContext struct {
	mailClient     MailClient // a mail client with the MailClient interface
	logger         *Logging.Logger
	stopAllCh      chan int // (broadcast) closed when client is exiting, so that any sub-routine can exit
	command        chan PingerCommand
	lastError      error
	stats          *Utils.StatLogger
	di             *DeviceInfo
	ClientId       string
	ClientContext  string
	DeviceId       string
	Protocol       string
	sessionId      string
	WaitBeforeUse  int64 // in milliseconds
	MaxPollTimeout int64 // max polling lifetime in milliseconds. Default 2 days.
	wg             sync.WaitGroup
	status         MailClientStatus
	logPrefix      string
	fsm            *fsm.FSM
	deferTimer     *time.Timer
	maxPollTime    time.Duration
	maxPollTimer   *time.Timer
}

func (client *MailClientContext) getLogPrefix() string {
	if client.logPrefix == "" {
		client.logPrefix = fmt.Sprintf("%s:%s:%s:%s", client.DeviceId, client.ClientId, client.ClientContext, client.sessionId)
	}
	return client.logPrefix
}

func (client *MailClientContext) setStatus(status MailClientStatus, err error) {
	client.status = status
	client.lastError = err
}

var LongPollReRegister error
var LongPollNewMail error

func init() {
	LongPollReRegister = fmt.Errorf("Need Registger")
	LongPollNewMail = fmt.Errorf("New Mail")
}

type MailClient interface {
	LongPoll(stopPollCh, stopAllCh chan int, errCh chan error)
	Cleanup()
}

const (
	MailClientActiveSync = "ActiveSync"
	MailClientIMAP       = "IMAP"
)

type MailClientStatus int

const (
	MailClientStatusError       MailClientStatus = iota
	MailClientStatusInitialized MailClientStatus = iota
	MailClientStatusPinging     MailClientStatus = iota
	MailClientStatusDeferred    MailClientStatus = iota
	MailClientStatusStopped     MailClientStatus = iota
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
		stopAllCh:      make(chan int),
		command:        make(chan PingerCommand, 10),
		stats:          nil,
		status:         MailClientStatusInitialized,
		ClientId:       pi.ClientId,
		ClientContext:  pi.ClientContext,
		DeviceId:       pi.DeviceId,
		Protocol:       pi.Protocol,
		WaitBeforeUse:  pi.WaitBeforeUse,
		MaxPollTimeout: pi.MaxPollTimeout,
		sessionId:      pi.SessionId,
	}
	err := aws.ValidateCognitoID(pi.ClientId)
	if err != nil {
		client.Error("Could not validate client ID: %s", err.Error())
		return nil, err
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
		mailclient, err = NewExchangeClient(pi, &client.wg, debug, logger)
		if err != nil {
			return nil, err
		}
		//	case strings.EqualFold(client.Protocol, MailClientIMAP):
		//		mailclient, err = NewIMAPClient(pi, &client.wg, debug, logger)
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
	client.updateLastContact()
	client.Debug("Starting polls for %s", pi.String())
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
	client.di.cleanup()
	client.di = nil
	client.Debug("Cleaning up MailClientContext struct")
	if client.mailClient != nil {
		client.mailClient.Cleanup()
		client.mailClient = nil
	}

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

const (
	FSMInit     = "init"
	FSMDeferred = "deferred"
	FSMPinging  = "pinging"
	FSMStopped  = "stopped"
)

func (client *MailClientContext) initFsm() {
	client.fsm = fsm.NewFSM(
		FSMInit,
		fsm.Events{
			{Name: FSMInit,
				Src: []string{""},
				Dst: FSMInit},
			{Name: FSMDeferred,
				Src: []string{FSMStopped, FSMInit},
				Dst: FSMDeferred},
			{Name: FSMPinging,
				Src: []string{FSMDeferred, FSMStopped},
				Dst: FSMPinging},
			{Name: FSMStopped,
				Src: []string{FSMPinging, FSMDeferred},
				Dst: FSMStopped},
		},
		fsm.Callbacks{
			"leave_init":     client.leaveInit,
			"enter_deferred": client.enterDeferred,
			"leave_deferred": client.exitDeferred,
			"enter_pinging":  client.enterPinging,
			"enter_stopped":  client.enterStopped,
		},
	)
	client.Debug("FSM initialized")
}

func (client *MailClientContext) leaveInit(e *fsm.Event) {
	client.maxPollTime = time.Duration(client.MaxPollTimeout) * time.Millisecond
	client.Debug("Setting maxPollTimer for %s", client.maxPollTime)
	client.maxPollTimer = time.NewTimer(client.maxPollTime)
	client.deferTimer = time.NewTimer(time.Duration(client.WaitBeforeUse) * time.Millisecond)
}

func (client *MailClientContext) enterDeferred(e *fsm.Event) {
	client.deferTimer.Stop()
	deferTime := time.Duration(client.WaitBeforeUse) * time.Millisecond
	client.Debug("Starting deferTimer for %s", deferTime)
	client.deferTimer.Reset(deferTime)
	client.setStatus(MailClientStatusDeferred, nil)
}

func (client *MailClientContext) exitDeferred(e *fsm.Event) {
	client.deferTimer.Stop()
}

func (client *MailClientContext) enterPinging(e *fsm.Event) {
	stopPollCh := e.Args[0].(chan int)
	errCh := e.Args[1].(chan error)
	client.setStatus(MailClientStatusPinging, nil)
	go client.mailClient.LongPoll(stopPollCh, client.stopAllCh, errCh)
}

func (client *MailClientContext) enterStopped(e *fsm.Event) {
	msg := e.Args[0].(string)
	status := e.Args[1].(MailClientStatus)
	err := e.Args[2].(error)
	client.Info(msg)
	client.setStatus(status, err)
}

func logError(err error, logger *Logging.Logger) {
	_, fn, line, _ := runtime.Caller(1)
	logger.Error("%s/%s:%d %s", path.Base(path.Dir(fn)), path.Base(fn), line, err)
}

func (client *MailClientContext) start() {
	defer Utils.RecoverCrash(client.logger)
	defer func() {
		client.setStatus(MailClientStatusStopped, nil)
		client.Debug("Waiting for subroutines to finish")
		client.wg.Wait()
		client.Debug("Cleaning up")
		client.cleanup()
	}()
	if client.stats != nil {
		go client.stats.TallyResponseTimes()
	}

	stopPollCh := make(chan int)
	errCh := make(chan error)
	rearmingCount := 0

	client.initFsm()
	err := client.fsm.Event(FSMDeferred)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case <-client.maxPollTimer.C:
			client.di.PushRegister()
			err = client.fsm.Event(FSMStopped, "maxPollTimer expired. Stopping everything.", MailClientStatusStopped, nil)
			if err != nil {
				panic(err)
			}
			return

		case <-client.deferTimer.C:
			err = client.fsm.Event(FSMPinging, stopPollCh, errCh)
			if err != nil {
				panic(err)
			}

		case err := <-errCh:
			switch {
			case err == LongPollNewMail:
				// TODO Should we send a push notification each time we've rearmed? Or just
				// on the first go-around (rearmingCount == 0)?
				client.Info("Sending push message for new mail")
				err = client.di.PushNewMail()
				if err != nil {
					if client.di.aws.IgnorePushFailures() == false {
						logError(err, client.logger)
						return
					} else {
						client.Warning("Push failed but ignored: %s", err.Error())
					}
				}
				client.Debug("New mail notification sent")
				err = client.fsm.Event(FSMStopped, "Stopping (new mail push sent)", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}

				if rearmingCount < 3 {
					rearmingCount++
					client.WaitBeforeUse = int64(time.Duration(10)*time.Minute) / int64(time.Millisecond)
					client.Info("Rearming LongPoll (%d)", rearmingCount)
					err = client.fsm.Event(FSMDeferred)
					if err != nil {
						panic(err)
					}
				} else {
					return
				}

			case err == LongPollReRegister:
				err1 := client.di.PushRegister()
				if err1 != nil {
					// don't bother with this error. The real/main error is the http status. Just log it.
					client.Error("Push failed but ignored: %s", err1.Error())
				}
				err = client.fsm.Event(FSMStopped, "Client needs reregister. Stopping.", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				return

			default:
				// the mailClient.LongPoll has thrown an error. note it.
				err = client.fsm.Event(FSMStopped, fmt.Sprintf("Error Thrown: %s. Stopping", err.Error), MailClientStatusError, err)
				if err != nil {
					panic(err)
				}
				return
			}

		case cmd := <-client.command:
			switch {
			case cmd == PingerStop:
				close(client.stopAllCh) // tell all goroutines listening on this channel that they can stop now.
				err = client.fsm.Event(FSMStopped, "got 'PingerStop' command", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				return

			case cmd == PingerDefer:
				err = client.fsm.Event(FSMStopped, "Got 'PingerDefer' command", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				// this comes from the client, which means we need to reset the count.
				rearmingCount = 0
				err = client.fsm.Event(FSMDeferred)
				if err != nil {
					panic(err)
				}

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

func (client *MailClientContext) stop() {
	if client.mailClient == nil {
		client.Warning("Client is stopped. Can not stop")
		return
	}
	client.Debug("Stopping polls")
	err := client.updateLastContact()
	if err != nil {
		client.Error("Could not update last contact: %s", err.Error())
	}
	err = client.Action(PingerStop)
	if err != nil {
		client.Error("Could not send stop action: %s", err.Error())
	}
	return
}

func (client *MailClientContext) deferPoll(timeout int64) {
	if client.mailClient == nil {
		client.Warning("Client is stopped. Can not defer")
		return
	}
	client.Debug("Deferring polls")
	err := client.updateLastContact()
	if err != nil {
		client.Error("Could not update last contact: %s", err.Error())
	}
	if timeout > 0 {
		client.WaitBeforeUse = timeout
	}
	err = client.Action(PingerDefer)
	if err != nil {
		client.Error("Could not send defer action: %s", err.Error())

	}
}

func (client *MailClientContext) updateLastContact() error {
	return client.di.updateLastContact()
}

type ClientSessionInfo struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	SessionId     string
	Status        MailClientStatus
	Error         string
}

func (client *MailClientContext) sessionInfo() *ClientSessionInfo {
	status, err := client.Status()
	info := ClientSessionInfo{
		ClientId:      client.ClientId,
		ClientContext: client.ClientContext,
		DeviceId:      client.DeviceId,
		SessionId:     client.sessionId,
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
