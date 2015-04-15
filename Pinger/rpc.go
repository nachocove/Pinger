package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"io/ioutil"
	"log"
	"net/http"
	"net/rpc"
)

const (
	RPCProtocolHTTP = "http"
	RPCProtocolUnix = "unix"
)

type RPCServerConfiguration struct {
	Protocol string
	Path     string
	Hostname string
	Port     int
}

func (rpcConf *RPCServerConfiguration) ConnectString() string {
	switch {
	case rpcConf.Protocol == RPCProtocolHTTP:
		return fmt.Sprintf("%s:%d", rpcConf.Hostname, rpcConf.Port)

	case rpcConf.Protocol == RPCProtocolUnix:
		return fmt.Sprintf("%s", rpcConf.Path)
	}
	return ""
}

func (rpcConf *RPCServerConfiguration) String() string {
	return fmt.Sprintf("%s://%s", rpcConf.Protocol, rpcConf.ConnectString())
}

func NewRPCServerConfiguration() RPCServerConfiguration {
	return RPCServerConfiguration{
		Protocol: RPCProtocolHTTP,  // options: "unix", "http"
		Path:     "/tmp/PingerRpc", // used if Protocol is "unix"
		Hostname: "localhost",      // used if Protocol is "http"
		Port:     RPCPort,          // used if Protocol is "http"
	}
}

const RPCPort = 60600

type PollingReplyType int

const (
	PollingReplyError PollingReplyType = 0
	PollingReplyOK    PollingReplyType = 1
	PollingReplyWarn  PollingReplyType = 2
)

func (r PollingReplyType) String() string {
	switch {
	case r == PollingReplyError:
		return "Error"
	case r == PollingReplyOK:
		return "OK"
	case r == PollingReplyWarn:
		return "Warning"
	default:
		panic(fmt.Sprintf("Unknown PollingReplyType: %d", r))
	}
}

type BackendPoller interface {
	newMailClientContext(pi *MailPingInformation, doStats bool) (MailClientContextType, error)
	Start(args *StartPollArgs, reply *StartPollingResponse) (err error)
	Stop(args *StopPollArgs, reply *PollingResponse) (err error)
	Defer(args *DeferPollArgs, reply *PollingResponse) (err error)
	LockMap()
	UnlockMap()
}

type pollMapType map[string]MailClientContextType

func StartPollingRPCServer(config *Configuration, debug bool, logger *Logging.Logger) error {
	pollingAPI, err := NewBackendPolling(config, true, logger)
	if err != nil {
		return err
	}
	setGlobal(&config.Backend)

	log.SetOutput(ioutil.Discard) // rpc.Register logs a warning for ToggleDebug, which we don't want.

	rpc.Register(pollingAPI)
	go FeedbackListener(logger)
	go alertAllDevices(pollingAPI.dbm, pollingAPI.aws, pollingAPI.logger)

	if config.Backend.PingerUpdater > 0 {
		pinger, err := newPingerInfo(pollingAPI.dbm, logger)
		if err != nil {
			return err
		}
		go pinger.Updater(config.Backend.PingerUpdater)
	}

	logger.Debug("Starting RPC server on %s (pinger id %s)", config.Rpc.String(), pingerHostId)
	switch {
	case config.Rpc.Protocol == RPCProtocolHTTP:
		rpc.HandleHTTP()
		err = http.ListenAndServe(config.Rpc.ConnectString(), nil)
		if err != nil {
			return err
		}
	case config.Rpc.Protocol == RPCProtocolUnix:
		panic("UNIX server is not yet implemented")
	}
	return nil
}

// StartPollingResponse is used by the start polling rpc
type StartPollingResponse struct {
	Code    PollingReplyType
	Message string
}

type StartPollArgs struct {
	MailInfo *MailPingInformation
}

func (sa *StartPollArgs) pollMapKey() string {
	return fmt.Sprintf("%s--%s--%s", sa.MailInfo.ClientId, sa.MailInfo.ClientContext, sa.MailInfo.DeviceId)
}

func (sa *StartPollArgs) getLogPrefix() string {
	return sa.MailInfo.getLogPrefix()
}
func RPCStartPoll(t BackendPoller, pollMap *pollMapType, dbm *gorp.DbMap, args *StartPollArgs, reply *StartPollingResponse, logger *Logging.Logger) (err error) {
	defer func() {
		e := Utils.RecoverCrash(logger)
		if e != nil {
			err = e
		}
		if err != nil {
			logger.Error("%s", err.Error())
		}
	}()
	logger.Info("%s: Received StartPoll request", args.getLogPrefix())
	pollMapKey := args.pollMapKey()
	reply.Code = PollingReplyOK
	var client MailClientContextType
	t.LockMap()
	need_unlock := true
	defer func() {
		if need_unlock {
			t.UnlockMap()
		}
	}()
	client, ok := (*pollMap)[pollMapKey]
	if ok == true {
		if client == nil {
			err = fmt.Errorf("%s: Could not find poll session in map", args.getLogPrefix())
			return err
		}
		status, err := client.Status()
		logger.Debug("%s: Found Existing polling session: status %s, err %v", args.getLogPrefix(), status, err)
		switch {
		case status == MailClientStatusStopped:
			logger.Debug("%s: Polling has stopped.", args.getLogPrefix())

		case status == MailClientStatusPinging:
			logger.Debug("%s: Polling. Stopping it.", args.getLogPrefix())

		case status == MailClientStatusError:
			if err != nil {
				logger.Debug("%s: Not polling. Last error was %s", args.getLogPrefix(), err)
				reply.Message = fmt.Sprintf("Previous Ping failed with error: %s", err.Error())
			} else {
				logger.Debug("%s: Not polling.", args.getLogPrefix())
				reply.Message = fmt.Sprintf("Not polling")
			}
			reply.Code = PollingReplyWarn
		}
		err = client.Action(PingerStop)
		if err != nil {
			reply.Message = err.Error()
			reply.Code = PollingReplyError
			return nil
		}
		delete((*pollMap), pollMapKey)

		client = nil
	} else {
		if client != nil {
			panic("Got a client but ok is false?")
		}
	}
	t.UnlockMap()
	need_unlock = false
	go createNewPingerSession(t, pollMap, pollMapKey, args.MailInfo, logger)
	return nil
}

func createNewPingerSession(t BackendPoller, pollMap *pollMapType, pollMapKey string, mi *MailPingInformation, logger *Logging.Logger) {
	// nothing started. So start it.
	logger.Debug("%s: Creating session", mi.getLogPrefix())
	client, err := t.newMailClientContext(mi, false)
	if err != nil {
		logger.Error("%s: Could not create new client: %s", pollMapKey, err)
		return
	}
	t.LockMap()
	defer func() {
		logger.Debug("%s: Done creating session", mi.getLogPrefix())
		t.UnlockMap()
	}()
	if _, ok := (*pollMap)[pollMapKey]; ok == true {
		// something else snuck in there! Stop this one.
		client.stop()
	} else {
		(*pollMap)[pollMapKey] = client
	}
}

type StopPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string

	logPrefix string
}

func (sp *StopPollArgs) pollMapKey() string {
	return fmt.Sprintf("%s--%s--%s", sp.ClientId, sp.ClientContext, sp.DeviceId)
}

func (sp *StopPollArgs) getLogPrefix() string {
	if sp.logPrefix == "" {
		sp.logPrefix = fmt.Sprintf("%s:%s:%s", sp.DeviceId, sp.ClientId, sp.ClientContext)
	}
	return sp.logPrefix
}

// PollingResponse is used by Stop and Defer
type PollingResponse struct {
	Code    PollingReplyType
	Message string
}

func RPCStopPoll(t BackendPoller, pollMap *pollMapType, dbm *gorp.DbMap, args *StopPollArgs, reply *PollingResponse, logger *Logging.Logger) (err error) {
	defer func() {
		e := Utils.RecoverCrash(logger)
		if e != nil {
			err = e
		}
		if err != nil {
			logger.Error("%s", err.Error())
		}
	}()
	logger.Info("%s: Received stopPoll request", args.getLogPrefix())
	pollMapKey := args.pollMapKey()
	t.LockMap()
	defer t.UnlockMap()
	client, ok := (*pollMap)[pollMapKey]
	if ok {
		delete((*pollMap), pollMapKey)
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		go client.stop()
		reply.Message = "Stopped"
	} else {
		logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "No active sessions found"
		return
	}
	reply.Code = PollingReplyOK
	err = nil
	return
}

type DeferPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Timeout       int64

	logPrefix string
}

func (dp *DeferPollArgs) pollMapKey() string {
	return fmt.Sprintf("%s--%s--%s", dp.ClientId, dp.ClientContext, dp.DeviceId)
}

func (dp *DeferPollArgs) getLogPrefix() string {
	if dp.logPrefix == "" {
		dp.logPrefix = fmt.Sprintf("%s:%s:%s", dp.DeviceId, dp.ClientId, dp.ClientContext)
	}
	return dp.logPrefix
}

func RPCDeferPoll(t BackendPoller, pollMap *pollMapType, dbm *gorp.DbMap, args *DeferPollArgs, reply *PollingResponse, logger *Logging.Logger) (err error) {
	defer func() {
		e := Utils.RecoverCrash(logger)
		if e != nil {
			err = e
		}
		if err != nil {
			logger.Error("%s", err.Error())
		}
	}()
	logger.Info("%s: Received deferPoll request", args.getLogPrefix())
	reply.Code = PollingReplyOK
	reply.Message = ""
	pollMapKey := args.pollMapKey()
	client, ok := (*pollMap)[pollMapKey]
	if ok {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		status, err := client.Status()
		if err != nil {
			return err
		}
		if status != MailClientStatusPinging && status != MailClientStatusDeferred {
			reply.Code = PollingReplyError
			reply.Message = fmt.Sprintf("Client is not pinging or deferred (%s). Can not defer.", status)
		} else {
			go client.deferPoll(args.Timeout)
		}
	} else {
		logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "No active sessions found"
		return
	}
	return nil
}

type FindSessionsArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	MaxSessions   int

	logPrefix string
}

type FindSessionsResponse struct {
	Code         PollingReplyType
	Message      string
	SessionInfos []ClientSessionInfo
}

func (fs *FindSessionsArgs) getLogPrefix() string {
	if fs.logPrefix == "" {
		fs.logPrefix = fmt.Sprintf("%s:%s:%s", fs.DeviceId, fs.ClientId, fs.ClientContext)
	}
	return fs.logPrefix
}

func RPCFindActiveSessions(t BackendPoller, pollMap *pollMapType, dbm *gorp.DbMap, args *FindSessionsArgs, reply *FindSessionsResponse, logger *Logging.Logger) (err error) {
	defer func() {
		e := Utils.RecoverCrash(logger)
		if e != nil {
			err = e
		}
	}()
	logger.Debug("Received findActiveSessions request with options %s", args.getLogPrefix())
	for key, poll := range *pollMap {
		if args.MaxSessions > 0 && len(reply.SessionInfos) >= args.MaxSessions {
			logger.Debug("Max sessions read (%d). Stopping search.", len(reply.SessionInfos))
			break
		}
		session, err := poll.getSessionInfo()
		if err != nil {
			logger.Debug("%s: %s", key, err.Error())
			continue
		}

		switch {
		case args.ClientId == "" && args.ClientContext == "" && args.DeviceId == "":
			reply.SessionInfos = append(reply.SessionInfos, *session)

		case args.ClientId != "" && session.ClientId == args.ClientId:
			reply.SessionInfos = append(reply.SessionInfos, *session)

		case args.ClientContext != "" && session.ClientContext == args.ClientContext:
			reply.SessionInfos = append(reply.SessionInfos, *session)

		case args.DeviceId != "" && session.DeviceId == args.DeviceId:
			reply.SessionInfos = append(reply.SessionInfos, *session)
		}
	}
	reply.Code = PollingReplyOK
	reply.Message = ""
	return nil

}

type AliveCheckArgs struct {
}

type AliveCheckResponse struct {
	Code    PollingReplyType
	Message string
}

func RPCAliveCheck(t BackendPoller, pollMap *pollMapType, dbm *gorp.DbMap, args *AliveCheckArgs, reply *AliveCheckResponse, logger *Logging.Logger) (err error) {
	defer func() {
		e := Utils.RecoverCrash(logger)
		if e != nil {
			err = e
		}
	}()
	logger.Info("Received aliveCheck request")
	if globals.config.PingerUpdater > 0 {
		logger.Warning("Running both auto-updater and a remote Alive Check")
	}
	_, err = newPingerInfo(dbm, logger) // this updates the timestamp
	if err != nil {
		return err
	}
	reply.Code = PollingReplyOK
	reply.Message = ""
	return nil
}
