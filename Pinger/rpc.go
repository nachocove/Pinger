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

const (
	PollingReplyError = 0
	PollingReplyOK    = 1
	PollingReplyWarn  = 2
)

type BackendPoller interface {
	newMailClientContext(pi *MailPingInformation, doStats bool) (MailClientContextType, error)
	validateClientID(clientID string) error
	Start(args *StartPollArgs, reply *StartPollingResponse) (err error)
	Stop(args *StopPollArgs, reply *PollingResponse) (err error)
	Defer(args *DeferPollArgs, reply *PollingResponse) (err error)
}

type pollMapType map[string]MailClientContextType

func StartPollingRPCServer(config *Configuration, debug, debugSql bool, logger *Logging.Logger) error {
	pollingAPI, err := NewBackendPolling(config, debug, debugSql, logger)
	if err != nil {
		return err
	}
	setGlobal(&config.Global, pollingAPI.aws)
	
	log.SetOutput(ioutil.Discard) // rpc.Register logs a warning for ToggleDebug, which we don't want.

	rpc.Register(pollingAPI)

	go alertAllDevices(pollingAPI.dbm, pollingAPI.logger)

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
	Code    int
	Token   string
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
	logger.Debug("%s: Received StartPoll request", args.getLogPrefix())
	pollMapKey := args.pollMapKey()
	reply.Code = PollingReplyOK
	var client MailClientContextType
	client, ok := (*pollMap)[pollMapKey]
	if ok == true {
		if client == nil {
			err = fmt.Errorf("%s: Could not find poll session in map", args.getLogPrefix())
			return err
		}
		err = client.updateLastContact()
		if err != nil {
			reply.Message = err.Error()
			reply.Code = PollingReplyError
			return nil
		}
		logger.Debug("%s: Found Existing polling session", args.getLogPrefix())
		status, err := client.Status()
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
		client = nil
	} else {
		if client != nil {
			panic("Got a client but ok is false?")
		}
	}
	err = t.validateClientID(args.MailInfo.ClientId)
	if err != nil {
		reply.Message = err.Error()
		reply.Code = PollingReplyError
		return nil
	}

	// nothing started. So start it.
	client, err = t.newMailClientContext(args.MailInfo, false)
	if err != nil {
		message := fmt.Sprintf("%s: Could not create new client: %s", args.getLogPrefix(), err)
		logger.Warning(message)
		reply.Message = message
		reply.Code = PollingReplyError
		return
	}
	(*pollMap)[pollMapKey] = client
	reply.Token = client.getStopToken()
	return
}

type StopPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	StopToken     string

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
	Code    int
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
	logger.Debug("%s: Received stopPoll request", args.getLogPrefix())
	pollMapKey := args.pollMapKey()
	client, ok := (*pollMap)[pollMapKey]
	if ok == false {
		// nothing on file.
		logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "Not Polling"
		err = nil
		return
	} else {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		validToken := client.validateStopToken(args.StopToken)
		if validToken == false {
			logger.Warning("%s: invalid token", args.getLogPrefix())
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
			err = nil
			return
		} else {
			err = client.updateLastContact()
			if err != nil {
				logger.Error("%s: Could not update last contact %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				err = nil
				return
			}
			err = client.stop()
			if err != nil {
				logger.Error("%s:: Error stopping poll: %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				err = nil
				return
			} else {
				delete((*pollMap), args.ClientId)
				reply.Message = "Stopped"
			}
		}
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
	StopToken     string

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
	logger.Debug("%s: Received deferPoll request", args.getLogPrefix())
	pollMapKey := args.pollMapKey()
	client, ok := (*pollMap)[pollMapKey]
	if ok == false {
		// nothing on file.
		logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "Not Polling"
		return nil
	} else {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		validToken := client.validateStopToken(args.StopToken)
		if validToken == false {
			logger.Warning("%s: invalid token", args.getLogPrefix())
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
			return nil
		} else {
			err = client.updateLastContact()
			if err != nil {
				logger.Error("%s: Could not update last contact %s", args.getLogPrefix(), err.Error())
				reply.Code = PollingReplyError
				reply.Message = err.Error()
				return nil
			}
			err = client.deferPoll(args.Timeout)
			if err != nil {
				logger.Error("%s: Error deferring poll: %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			}
		}
	}
	reply.Code = PollingReplyOK
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
	Code         int
	Message      string
	SessionInfos []ClientSessionInfo
}

func (fs *FindSessionsArgs) getLogPrefix() string {
	if fs.logPrefix == "" {
		fs.logPrefix = fmt.Sprintf("%s:%s:%s", fs.DeviceId, fs.ClientId, fs.ClientContext)
	}
	return fs.logPrefix
}

func RPCFindActiveSessions(pollMap *pollMapType, args *FindSessionsArgs, reply *FindSessionsResponse, logger *Logging.Logger) (err error) {
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

		default:
			logger.Debug("%s: Unknown case!", key)
		}
	}
	reply.Code = PollingReplyOK
	reply.Message = ""
	return nil

}
