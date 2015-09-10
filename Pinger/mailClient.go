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
	deferPoll(timeout uint64)
	stop()
	updateLastContact() error
	Status() (MailClientStatus, error)
	setStatus(MailClientStatus, error)
	Action(action PingerCommand) error
	getSessionInfo() (*ClientSessionInfo, error)
}

type MailClientContext struct {
	mailClient      MailClient // a mail client with the MailClient interface
	logger          *Logging.Logger
	stopAllCh       chan int // (broadcast) closed when client is exiting, so that any sub-routine can exit
	stopPollCh      chan int // (unicast) closed when we want the longpoll to stop
	command         chan PingerCommand
	lastError       error
	stats           *Utils.StatLogger
	di              *DeviceInfo
	UserId          string
	ClientContext   string
	DeviceId        string
	Protocol        string
	sessionId       string
	WaitBeforeUse   uint64 // in milliseconds
	MaxPollTimeout  uint64 // max polling lifetime in milliseconds. Default 2 days.
	ResponseTimeout uint64 // in milliseconds
	wg              sync.WaitGroup
	status          MailClientStatus
	logPrefix       string
	fsm             *fsm.FSM
	deferTimer      *time.Timer
	maxPollTime     time.Duration
	maxPollTimer    *time.Timer
}

func (client *MailClientContext) getLogPrefix() string {
	if client.logPrefix == "" {
		client.logPrefix = fmt.Sprintf("|device=%s|client=%s|context=%s|session=%s", client.DeviceId, client.UserId, client.ClientContext, client.sessionId)
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
	LongPollReRegister = fmt.Errorf("Need to reregister")
	LongPollNewMail = fmt.Errorf("New mail")
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
	MailClientStatusReDeferred  MailClientStatus = iota
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

	case status == MailClientStatusReDeferred:
		return "Rearmed"

	case status == MailClientStatusStopped:
		return "Stopped"
	}
	panic(fmt.Sprintf("Unknown status %d", status))
}

const (
	DefaultMaxPollTimeout uint64 = 2 * 24 * 60 * 60 * 1000 // 2 days in milliseconds
)

func NewMailClientContext(dbm *gorp.DbMap, aws AWS.AWSHandler, pi *MailPingInformation, debug, doStats bool, logger *Logging.Logger) (*MailClientContext, error) {
	client := &MailClientContext{
		logger:          logger.Copy(),
		stopAllCh:       make(chan int),
		command:         make(chan PingerCommand, 10),
		stats:           nil,
		status:          MailClientStatusInitialized,
		UserId:          pi.UserId,
		ClientContext:   pi.ClientContext,
		DeviceId:        pi.DeviceId,
		Protocol:        pi.Protocol,
		WaitBeforeUse:   pi.WaitBeforeUse,
		MaxPollTimeout:  pi.MaxPollTimeout,
		ResponseTimeout: pi.ResponseTimeout,
		sessionId:       pi.SessionId,
	}
	err := aws.ValidateCognitoID(pi.UserId)
	if err != nil {
		client.Error("Could not validate user id|userId=%s|msgCode=INVALID_USERID", err.Error())
		return nil, err
	}

	client.logger.SetCallDepth(1)

	di, err := pi.newDeviceInfo(newDeviceInfoSqlHandler(dbm), aws, logger)
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
	case strings.EqualFold(client.Protocol, MailClientIMAP):
		mailclient, err = NewIMAPClient(pi, &client.wg, debug, logger)
		if err != nil {
			return nil, err
		}
	default:
		client.Error("Unsupported mail protocol|protocol=%s|msgCode=UNSUP_PROTO", client.Protocol)
		return nil, fmt.Errorf("%s|Unsupported mail protocol|protocol=%s", pi.getLogPrefix(), client.Protocol)
	}

	if mailclient == nil {
		return nil, fmt.Errorf("%s|Could not create new mail client pinger", pi.getLogPrefix())
	}
	client.updateLastContact()
	client.Info("Created new Pinger|%s|msgCode=PINGER_CREATED", pi.String())
	client.mailClient = mailclient
	go client.start()
	return client, nil
}

func (client *MailClientContext) Debug(format string, args ...interface{}) {
	client.logger.Debug(fmt.Sprintf("%s|message=%s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Info(format string, args ...interface{}) {
	client.logger.Info(fmt.Sprintf("%s|message=%s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Error(format string, args ...interface{}) {
	client.logger.Error(fmt.Sprintf("%s|message=%s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Warning(format string, args ...interface{}) {
	client.logger.Warning(fmt.Sprintf("%s|message=%s", client.getLogPrefix(), format), args...)
}

func (client *MailClientContext) Status() (MailClientStatus, error) {
	if client.lastError != nil {
		return MailClientStatusError, client.lastError
	}
	return client.status, nil
}

func (client *MailClientContext) cleanup() {
	if client.di != nil {
		client.di.cleanup()
		client.di = nil
	}
	client.Debug("Cleaning up mail client|msgCode=PINGER_CLEANUP")
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
			"leave_pinging":  client.exitPinging,
			"enter_stopped":  client.enterStopped,
			"leave_stopped":  client.exitStopped,
		},
	)
	client.Debug("FSM initialized")
}

func (client *MailClientContext) leaveInit(e *fsm.Event) {
	client.maxPollTime = time.Duration(client.MaxPollTimeout) * time.Millisecond
	client.Debug("Setting max poll timer|maxPollTimer=%s", client.maxPollTime)
	client.maxPollTimer = time.NewTimer(client.maxPollTime)
	client.deferTimer = time.NewTimer(time.Duration(client.WaitBeforeUse) * time.Millisecond)
}

func (client *MailClientContext) enterDeferred(e *fsm.Event) {
	status := e.Args[0].(MailClientStatus)
	client.deferTimer.Stop()
	deferTime := time.Duration(client.WaitBeforeUse) * time.Millisecond
	client.Debug("Enter defer|deferTimer=%s", deferTime)
	client.deferTimer.Reset(deferTime)
	client.setStatus(status, nil)
}

func (client *MailClientContext) exitDeferred(e *fsm.Event) {
	client.Debug("Exit defer|pollstate=%s", client.status)
	client.deferTimer.Stop()
}

func (client *MailClientContext) enterPinging(e *fsm.Event) {
	client.stopPollCh = make(chan int)
	errCh := e.Args[0].(chan error)
	client.setStatus(MailClientStatusPinging, nil)
	client.Debug("Enter pinging|pollstate=%s", client.status)
	go client.mailClient.LongPoll(client.stopPollCh, client.stopAllCh, errCh)
}

func (client *MailClientContext) exitPinging(e *fsm.Event) {
	client.Debug("Exit pinging|pollstate=%s", client.status)
	close(client.stopPollCh)
}

func (client *MailClientContext) enterStopped(e *fsm.Event) {
	client.Debug("Enter stopped|pollstate=%s", client.status)
	msg := e.Args[0].(string)
	status := e.Args[1].(MailClientStatus)
	err, ok := e.Args[2].(error)
	if !ok {
		err = nil
	}
	client.Info(msg)
	client.setStatus(status, err)
}

func (client *MailClientContext) exitStopped(e *fsm.Event) {
	client.Debug("Exit stopped|pollstate=%s", client.status)
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
	client.Info("Starting state machine...|msgCode=START_SM")

	errCh := make(chan error)
	rearmingCount := 0
	tooFastResponse := (time.Duration(client.ResponseTimeout) * time.Millisecond) / 4
	var timeSent time.Time
	rearmTimeout := time.Duration(globals.config.ReArmTimeout) * time.Minute

	client.initFsm()
	err := client.fsm.Event(FSMDeferred, MailClientStatusDeferred)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case <-client.maxPollTimer.C:
			client.Info("MaxPoll timer expired. Sending ReRegister push message")
			perr := client.di.PushRegister()
			if perr != nil {
				if perr == APNSInvalidToken {
					client.Warning("Invalid token reported by Apple, deleting device|token=%s||msgCode=INVALID_PUSH_TOKEN", client.di.PushToken)
					client.di.cleanup()
					client.di = nil
				} else {
					client.Warning("Error reported by Apple|token=%s|err=%s", client.di.PushToken, perr)
				}
			}
			err = client.fsm.Event(FSMStopped, "maxPollTimer expired. Stopping everything.", MailClientStatusStopped, nil)
			if err != nil {
				panic(err)
			}
			return

		case <-client.deferTimer.C:
			client.Info("Defer timer expired. Starting poll")
			err = client.fsm.Event(FSMPinging, errCh)
			if err != nil {
				panic(err)
			}
			timeSent = time.Now()

		case err := <-errCh:
			switch {
			case err == LongPollNewMail:
				client.Info("New mail detected, checking notification status|timeSince=%s|rearmingCount=%d|msgCode=NEW_MAIL", time.Since(timeSent), rearmingCount)
				pushSent := false
				if time.Since(timeSent) > tooFastResponse || rearmingCount == 0 {
					client.Info("Sending push message for new mail")
					err = client.di.PushNewMail()
					if err != nil {
						if client.di.aws.IgnorePushFailures() == false {
							if err == APNSInvalidToken {
								client.Warning("Invalid Token reported by Apple for token '%s'.Deleting device|msgCode=INVALID_PUSH_TOKEN", client.di.PushToken)
								client.di.cleanup()
								client.di = nil
							} else {
								client.Error("Failed to push: %s|msgCode=PUSH_ERROR", err)
							}
							logError(err, client.logger)
							return
						} else {
							client.Warning("Push failed but ignored: %s|msgCode=PUSH_ERROR", err.Error())
						}
					}
					pushSent = true
					client.Info("Newmail notification sent|msgCode=PUSH_SENT")
				} else {
					client.Info("Newmail notification not sent|msgCode=PUSH_NOT_SENT")
				}
				var msg string
				if pushSent {
					msg = "Stopping - newmail push notification sent"
				} else {
					msg = "Stopping - no push notification sent"
				}
				client.Info("Stopping Poll|msgCode=STOP_POLL")

				err = client.fsm.Event(FSMStopped, msg, MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				if rearmingCount < 3 {
					rearmingCount++
					client.WaitBeforeUse = uint64(rearmTimeout) / uint64(time.Millisecond)
					client.Info("Rearming poll|rearmingCount=%d|rearmTimeout=%s|msgCode=REARMED", rearmingCount, rearmTimeout)
					err = client.fsm.Event(FSMDeferred, MailClientStatusReDeferred)
					if err != nil {
						panic(err)
					}
				} else {
					client.Info("Rearming count exceeded, stopping|rearmingCount=%d", rearmingCount)
					return
				}

			case err == LongPollReRegister:
				client.Info("LongPollReRegister message received. Sending ReRegister push message")
				err1 := client.di.PushRegister()
				if err1 != nil {
					// don't bother with this error. The real/main error is the http status. Just log it.
					client.Error("Push failed but ignored|err=%s", err1.Error())
					if err1 == APNSInvalidToken {
						client.Warning("Invalid token reported by Apple, deleting device|token=%s|msgCode=INVALID_PUSH_TOKEN", client.di.PushToken)
						client.di.cleanup()
						client.di = nil
					} else {
						client.Warning("Error reported by Apple|token=%s|err=%s|msgCode=PUSH_ERROR", client.di.PushToken, err1)
					}
				}
				err = client.fsm.Event(FSMStopped, "Client needs reregister, stopping poll", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				return

			default:
				// the mailClient.LongPoll has thrown an error. note it.
				err = client.fsm.Event(FSMStopped, fmt.Sprintf("Error thrown: %s, stopping poll", err.Error()), MailClientStatusError, err)
				if err != nil {
					panic(err)
				}
				return
			}

		case cmd := <-client.command:
			switch {
			case cmd == PingerStop:
				close(client.stopAllCh) // tell all goroutines listening on this channel that they can stop now.
				err = client.fsm.Event(FSMStopped, "Got PingerStop command", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				return

			case cmd == PingerDefer:
				err = client.fsm.Event(FSMStopped, "Got PingerDefer command", MailClientStatusStopped, nil)
				if err != nil {
					panic(err)
				}
				// this comes from the client, which means we need to reset the count.
				rearmingCount = 0
				err = client.fsm.Event(FSMDeferred, MailClientStatusDeferred)
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
		client.Warning("Poll is already stopped")
		return
	}
	client.Info("Stopping poll")
	err := client.updateLastContact()
	if err != nil {
		client.Error("Could not update last contact|err=%s", err.Error())
	}
	err = client.Action(PingerStop)
	if err != nil {
		client.Error("Could not send stop action|err=%s", err.Error())
	}
	return
}

func (client *MailClientContext) deferPoll(timeout uint64) {
	if client.mailClient == nil {
		client.Warning("Poll is stopped, cannot defer it")
		return
	}
	client.Info("Processing defer poll request")
	err := client.updateLastContact()
	if err != nil {
		client.Error("Could not update last contact|err=%s", err.Error())
	}
	if timeout > 0 {
		client.WaitBeforeUse = timeout
	}
	err = client.Action(PingerDefer)
	if err != nil {
		client.Error("Could not send defer action|err=%s", err.Error())

	}
}

func (client *MailClientContext) updateLastContact() error {
	return client.di.updateLastContact()
}

type ClientSessionInfo struct {
	UserId        string
	ClientContext string
	DeviceId      string
	SessionId     string
	Status        MailClientStatus
	Error         string
}

func (client *MailClientContext) sessionInfo() *ClientSessionInfo {
	status, err := client.Status()
	info := ClientSessionInfo{
		UserId:        client.UserId,
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

	case client.UserId == "" || client.ClientContext == "" || client.DeviceId == "":
		return nil, fmt.Errorf("Entry has been cleaned up.")
	}
	return client.sessionInfo(), nil
}
