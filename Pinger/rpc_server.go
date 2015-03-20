package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils"
)

// PollingResponse is used by Stop and Defer
type PollingResponse struct {
	Code    int
	Message string
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

func (sa *StartPollArgs) getLogPrefix() string {
	return sa.MailInfo.getLogPrefix()
}

func (t *BackendPolling) Start(args *StartPollArgs, reply *StartPollingResponse) error {
	defer Utils.RecoverCrash(t.logger)
	t.logger.Debug("%s: Received StartPoll request", args.getLogPrefix())
	pollMapKey := t.pollMapKey(args.MailInfo.ClientId, args.MailInfo.ClientContext, args.MailInfo.DeviceId)
	reply.Code = PollingReplyOK
	var client *MailClientContext
	client, ok := t.pollMap[pollMapKey]
	if ok == true {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll session in map", args.getLogPrefix())
		}
		err := updateLastContact(t.dbm, args.MailInfo.ClientId, args.MailInfo.ClientContext, args.MailInfo.DeviceId, t.logger)
		if err != nil {
			reply.Message = err.Error()
			reply.Code = PollingReplyError
			return nil
		}
		t.logger.Debug("%s: Found Existing polling session", args.getLogPrefix())
		status, err := client.Status()
		switch {
		case status == MailClientStatusStopped:
			t.logger.Debug("%s: Polling has stopped.", args.getLogPrefix())

		case status == MailClientStatusPinging:
			t.logger.Debug("%s: Polling. Stopping it.", args.getLogPrefix())

		case status == MailClientStatusError:
			if err != nil {
				t.logger.Debug("%s: Not polling. Last error was %s", args.getLogPrefix(), err)
				reply.Message = fmt.Sprintf("Previous Ping failed with error: %s", err.Error())
			} else {
				t.logger.Debug("%s: Not polling.", args.getLogPrefix())
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
	err := validateClientID(args.MailInfo.ClientId)
	if err != nil {
		return err
	}

	di, err := newDeviceInfoPI(t.dbm, args.MailInfo, t.logger)
	if err != nil {
		message := fmt.Sprintf("Could not save deviceInfo: %s", err)
		t.logger.Warning(message)
		reply.Message = message
		reply.Code = PollingReplyError
		return nil
	}
	t.logger.Debug("%s: created/updated device info", args.getLogPrefix())

	// nothing started. So start it.
	client, err = NewMailClientContext(args.MailInfo, di, t.debug, false, t.logger)
	if err != nil {
		message := fmt.Sprintf("%s: Could not create new client: %s", args.getLogPrefix(), err)
		t.logger.Warning(message)
		reply.Message = message
		reply.Code = PollingReplyError
		return nil
	}
	t.pollMap[pollMapKey] = client
	reply.Token = client.stopToken
	return nil
}

type StopPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	StopToken     string

	logPrefix string
}

func (sp *StopPollArgs) getLogPrefix() string {
	if sp.logPrefix == "" {
		sp.logPrefix = fmt.Sprintf("%s:%s:%s", sp.DeviceId, sp.ClientId, sp.ClientContext)
	}
	return sp.logPrefix
}

func (t *BackendPolling) Stop(args *StopPollArgs, reply *PollingResponse) error {
	defer Utils.RecoverCrash(t.logger)
	t.logger.Debug("%s: Received stopPoll request", args.getLogPrefix())
	pollMapKey := t.pollMapKey(args.ClientId, args.ClientContext, args.DeviceId)
	client, ok := t.pollMap[pollMapKey]
	if ok == false {
		// nothing on file.
		t.logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "Not Polling"
		return nil
	} else {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		validToken := client.validateStopToken(args.StopToken)
		if validToken == false {
			t.logger.Warning("%s: invalid token", args.getLogPrefix())
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
			return nil
		} else {
			err := updateLastContact(t.dbm, args.ClientId, args.ClientContext, args.DeviceId, t.logger)
			if err != nil {
				t.logger.Error("%s: Could not update last contact %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			}
			err = client.stop()
			if err != nil {
				t.logger.Error("%s:: Error stopping poll: %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			} else {
				delete(t.pollMap, args.ClientId)
				reply.Message = "Stopped"
			}
		}
	}
	reply.Code = PollingReplyOK
	return nil
}

type DeferPollArgs struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Timeout       int64
	StopToken     string

	logPrefix string
}

func (dp *DeferPollArgs) getLogPrefix() string {
	if dp.logPrefix == "" {
		dp.logPrefix = fmt.Sprintf("%s:%s:%s", dp.DeviceId, dp.ClientId, dp.ClientContext)
	}
	return dp.logPrefix
}

func (t *BackendPolling) Defer(args *DeferPollArgs, reply *PollingResponse) error {
	defer Utils.RecoverCrash(t.logger)
	t.logger.Debug("%s: Received deferPoll request", args.getLogPrefix())
	pollMapKey := t.pollMapKey(args.ClientId, args.ClientContext, args.DeviceId)
	client, ok := t.pollMap[pollMapKey]
	if ok == false {
		// nothing on file.
		t.logger.Warning("%s: No active sessions found for key %s", args.getLogPrefix(), pollMapKey)
		reply.Code = PollingReplyError
		reply.Message = "Not Polling"
		return nil
	} else {
		if client == nil {
			return fmt.Errorf("%s: Could not find poll item in map", args.getLogPrefix())
		}
		validToken := client.validateStopToken(args.StopToken)
		if validToken == false {
			t.logger.Warning("%s: invalid token", args.getLogPrefix())
			reply.Message = "Token does not match"
			reply.Code = PollingReplyError
			return nil
		} else {
			err := updateLastContact(t.dbm, args.ClientId, args.ClientContext, args.DeviceId, t.logger)
			if err != nil {
				t.logger.Error("%s: Could not update last contact %s", args.getLogPrefix(), err.Error())
				reply.Code = PollingReplyError
				reply.Message = err.Error()
				return nil
			}
			err = client.deferPoll(args.Timeout)
			if err != nil {
				t.logger.Error("%s: Error deferring poll: %s", args.getLogPrefix(), err.Error())
				reply.Message = err.Error()
				reply.Code = PollingReplyError
				return nil
			}
		}
	}
	reply.Code = PollingReplyOK
	return nil
}

func validateClientID(clientID string) error {
	if clientID == "" {
		return fmt.Errorf("Empty client ID is not valid")
	}
	return DefaultPollingContext.config.Aws.ValidateCognitoID(clientID)
}

type FindSessionsArgs struct {
	ClientId string
	ClientContext string
	DeviceId string
	MaxSessions int

	logPrefix string
}

type SessionInfo struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Status        MailClientStatus
	Url           string
	Error         string
}

type FindSessionsResponse struct {
	Code        int
	Message     string
	SessionInfos []SessionInfo
}

func (fs *FindSessionsArgs) getLogPrefix() string {
	if fs.logPrefix == "" {
		fs.logPrefix = fmt.Sprintf("%s:%s:%s", fs.DeviceId, fs.ClientId, fs.ClientContext)
	}
	return fs.logPrefix
}

func (t *BackendPolling) appendSessionInfo(sessionInfos []SessionInfo, mcc *MailClientContext) []SessionInfo {
	status, err := mcc.Status()
	info := SessionInfo{
		ClientId: mcc.pi.ClientId,
		ClientContext: mcc.pi.ClientContext,
		DeviceId: mcc.pi.DeviceId,
		Status: status,
		Url: mcc.pi.MailServerUrl,
		}
	if err != nil {
		info.Error = err.Error()
	}
	return append(sessionInfos, info)
}

func (t *BackendPolling) FindActiveSessions(args *FindSessionsArgs, reply *FindSessionsResponse) error {
	defer Utils.RecoverCrash(t.logger)
	t.logger.Debug("Received findActiveSessions request with options %s", args.getLogPrefix())
	for key, poll := range t.pollMap {
		if args.MaxSessions > 0 && len(reply.SessionInfos) >= args.MaxSessions {
			t.logger.Debug("Max sessions read (%d). Stopping search.", len(reply.SessionInfos))
			break
		} 
		switch {
		case poll.pi == nil:
			t.logger.Debug("%s: entry has no pi.", key)
			continue
			
		case poll.mailClient == nil:
			t.logger.Debug("%s: Entry has no active client", key)
			continue
			
		case poll.pi.ClientId == "" || poll.pi.ClientContext == "" || poll.pi.DeviceId == "":
			t.logger.Debug("%s: entry has been cleaned up.", key)
			continue
			
		case args.ClientId == "" && args.ClientContext == "" && args.DeviceId == "":
			reply.SessionInfos = t.appendSessionInfo(reply.SessionInfos, poll)
			continue
		
		case args.ClientId != "" && poll.pi.ClientId == args.ClientId:
			reply.SessionInfos = t.appendSessionInfo(reply.SessionInfos, poll)
			continue

		case args.ClientContext != "" && poll.pi.ClientContext == args.ClientContext:
			reply.SessionInfos = t.appendSessionInfo(reply.SessionInfos, poll)
			continue

		case args.DeviceId != "" && poll.pi.DeviceId == args.DeviceId:
			reply.SessionInfos = t.appendSessionInfo(reply.SessionInfos, poll)
			continue
			
		default:
			t.logger.Debug("%s: Unknown case!", key)
			continue
		}
	}
	reply.Code = PollingReplyOK
	reply.Message = ""
	return nil	
}
