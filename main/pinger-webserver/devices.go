package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"net/http"
	"strings"
)

func init() {
	httpsRouter.HandleFunc("/1/register", registerDevice)
	httpsRouter.HandleFunc("/1/defer", deferPolling)
	httpsRouter.HandleFunc("/1/stop", stopPolling)
}

//const SessionVarClientId = "ClientId"

// registerPostCredentials and registerPostData are (currently) mirror images
// of Pinger.MailPingInformation and Pinger.MailServerCredentials
// This is so that we can change the json interface without needing to touch
// the underlying Pinger code.
// That being said, there has to be a better way of doing this...
type registerPostData struct {
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
	MaxPollTimeout         int64  // maximum time to poll. Default is 2 days.
	OSVersion              string
	AppBuildNumber         string
	AppBuildVersion        string

	logPrefix string
}

func (pd *registerPostData) getLogPrefix() string {
	if pd.logPrefix == "" {
		pd.logPrefix = fmt.Sprintf("%s:%s:%s", pd.DeviceId, pd.ClientId, pd.ClientContext)
	}
	return pd.logPrefix
}

// Validate validate the structure/information to make sure required information exists.
func (pd *registerPostData) Validate() (bool, []string) {
	// TODO Enhance this function to do more security validation.
	ok := true
	MissingFields := []string{}
	if pd.ClientId == "" {
		MissingFields = append(MissingFields, "ClientId")
		ok = false
	}
	if pd.ClientContext == "" {
		MissingFields = append(MissingFields, "ClientContext")
		ok = false
	}
	if pd.DeviceId == "" {
		MissingFields = append(MissingFields, "DeviceId")
		ok = false
	}
	if pd.MailServerUrl == "" {
		MissingFields = append(MissingFields, "MailServerUrl")
		ok = false
	}
	if len(pd.RequestData) <= 0 {
		MissingFields = append(MissingFields, "RequestData")
		ok = false
	}
	if len(pd.NoChangeReply) <= 0 {
		MissingFields = append(MissingFields, "NoChangeReply")
		ok = false
	}
	if pd.ClientContext == "" {
		MissingFields = append(MissingFields, "ClientContext")
		ok = false
	}
	return ok, MissingFields
}

func (pd *registerPostData) AsMailInfo(sessionId string) *Pinger.MailPingInformation {
	// there's got to be a better way to do this...
	pi := Pinger.MailPingInformation{}
	pi.ClientId = pd.ClientId
	pi.ClientContext = pd.ClientContext
	pi.DeviceId = pd.DeviceId
	pi.Platform = pd.Platform
	pi.MailServerUrl = pd.MailServerUrl
	pi.MailServerCredentials.Username = pd.MailServerCredentials.Username
	pi.MailServerCredentials.Password = pd.MailServerCredentials.Password
	pi.Protocol = pd.Protocol
	pi.HttpHeaders = pd.HttpHeaders
	pi.RequestData = pd.RequestData
	pi.ExpectedReply = pd.ExpectedReply
	pi.NoChangeReply = pd.NoChangeReply
	pi.CommandTerminator = pd.CommandTerminator
	pi.CommandAcknowledgement = pd.CommandAcknowledgement
	pi.ResponseTimeout = pd.ResponseTimeout
	pi.WaitBeforeUse = pd.WaitBeforeUse
	pi.PushToken = pd.PushToken
	pi.PushService = pd.PushService
	if pd.MaxPollTimeout == 0 {
		pi.MaxPollTimeout = Pinger.DefaultMaxPollTimeout
	} else {
		pi.MaxPollTimeout = pd.MaxPollTimeout
	}
	pi.OSVersion = pd.OSVersion
	pi.AppBuildNumber = pd.AppBuildNumber
	pi.AppBuildVersion = pd.AppBuildVersion
	pi.SessionId = sessionId
	return &pi
}

func makeSessionId(token string) (string, error) {
	ha := sha256.Sum256([]byte(token))
	myId := make([]byte, 8)
	n := hex.Encode(myId, ha[0:4])
	if n <= 0 {
		return "", fmt.Errorf("Could not encode to hex string")
	}
	return string(myId), nil
}

func registerDevice(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	//	session, err := context.SessionStore.Get(r, "pinger-session")
	//	if err != nil {
	//		context.Logger.Warning("Could not get session")
	//		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	//		return
	//	}
	encodingStr := r.Header.Get("Content-Type")
	// TODO Check the length of the encodingStr. We roughly know how long it can reasonably be.
	postInfo := registerPostData{}
	switch {
	case encodingStr == "application/json" || encodingStr == "text/json":
		// TODO guess a reasonable max and check it here.
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&postInfo)
		if err != nil {
			context.Logger.Error("Could not parse json %s", err)
			http.Error(w, "Could not parse json", http.StatusBadRequest)
			return
		}

	default:
		context.Logger.Debug("Bad encoding %s", encodingStr)
		http.Error(w, "UNKNOWN Encoding", http.StatusBadRequest)
		return
	}
	ok, missingFields := postInfo.Validate()
	if ok == false {
		context.Logger.Warning("%s: Missing non-optional data: %s", postInfo.getLogPrefix(), strings.Join(missingFields, ","))
		responseError(w, MissingRequiredData, strings.Join(missingFields, ","))
		return
	}
	token, err := context.Config.Server.CreateAuthToken(postInfo.ClientId, postInfo.ClientContext, postInfo.DeviceId)
	if err != nil {
		context.Logger.Error("%s: error creating token %s", postInfo.getLogPrefix(), err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//	session.Values[SessionVarClientId] = postInfo.ClientId
	sessionId, err := makeSessionId(token)
	reply, err := Pinger.StartPoll(&context.Config.Rpc, postInfo.AsMailInfo(sessionId))
	if err != nil {
		context.Logger.Warning("%s: Could not re/start polling for device: %s", postInfo.getLogPrefix(), err)
		responseError(w, RPCServerError, "")
		return
	}
	context.Logger.Debug("%s: Re/Started Polling", postInfo.getLogPrefix())

	//	err = session.Save(r, w)
	//	if err != nil {
	//		context.Logger.Warning("Could not save session")
	//		responseError(w, SaveSessionError, "")
	//		return
	//	}
	responseData := make(map[string]string)

	switch {
	case reply.Code == Pinger.PollingReplyOK:
		responseData["Token"] = token
		responseData["Status"] = "OK"
		responseData["Message"] = ""

	case reply.Code == Pinger.PollingReplyError:
		responseData["Status"] = "ERROR"
		responseData["Message"] = reply.Message

	case reply.Code == Pinger.PollingReplyWarn:
		responseData["Token"] = token
		responseData["Status"] = "WARN"
		responseData["Message"] = reply.Message

	default:
		context.Logger.Error("%s: Unknown PollingReply Code %d", postInfo.getLogPrefix(), reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("%s: Could not json encode reply: %v", postInfo.getLogPrefix(), responseData)
		responseError(w, JSONEncodeError, "")
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}

type deferPost struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Timeout       int64
	Token         string

	logPrefix string
}

func (dp *deferPost) getLogPrefix() string {
	if dp.logPrefix == "" {
		dp.logPrefix = fmt.Sprintf("%s:%s:%s", dp.DeviceId, dp.ClientId, dp.ClientContext)
	}
	return dp.logPrefix
}

func deferPolling(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	//	session, err := context.SessionStore.Get(r, "pinger-session")
	//	if err != nil {
	//		context.Logger.Warning("Could not get session")
	//		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	//		return
	//	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "UNKNOWN Encoding", http.StatusBadRequest)
		return
	}

	deferData := deferPost{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&deferData)
	if err != nil {
		context.Logger.Error("Could not parse json %s", err)
		http.Error(w, "Could not parse json", http.StatusBadRequest)
		return
	}

	var reply *Pinger.PollingResponse

	_, err = context.Config.Server.ValidateAuthToken(deferData.ClientId, deferData.ClientContext, deferData.DeviceId, deferData.Token)
	if err != nil {
		reply = &Pinger.PollingResponse{
			Code:    Pinger.PollingReplyError,
			Message: "Token is not valid",
		}
	} else {
		//	if session.Values[SessionVarClientId] != deferData.ClientId {
		//		context.Logger.Error("Client ID %s does not match session", deferData.ClientId)
		//		http.Error(w, "Unknown Client ID", http.StatusForbidden)
		//		return
		//	}
		reply, err = Pinger.DeferPoll(&context.Config.Rpc, deferData.ClientId, deferData.ClientContext, deferData.DeviceId, deferData.Timeout)
		if err != nil {
			context.Logger.Error("%s: Error deferring poll %s", deferData.getLogPrefix(), err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	responseData := make(map[string]string)
	switch {
	case reply.Code == Pinger.PollingReplyError:
		responseData["Status"] = "ERROR"
		responseData["Message"] = reply.Message

	case reply.Code == Pinger.PollingReplyOK:
		responseData["Status"] = "OK"
		responseData["Message"] = ""

	case reply.Code == Pinger.PollingReplyWarn:
		responseData["Status"] = "WARN"
		responseData["Message"] = reply.Message

	default:
		context.Logger.Error("%s: Unknown PollingReply Code %d", deferData.getLogPrefix(), reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("%s: Could not json encode reply: %v", deferData.getLogPrefix(), responseData)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}

type stopPost struct {
	ClientId      string
	ClientContext string
	DeviceId      string
	Token         string

	logPrefix string
}

func (sp *stopPost) getLogPrefix() string {
	if sp.logPrefix == "" {
		sp.logPrefix = fmt.Sprintf("%s:%s:%s", sp.DeviceId, sp.ClientId, sp.ClientContext)
	}
	return sp.logPrefix
}

func stopPolling(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	//	session, err := context.SessionStore.Get(r, "pinger-session")
	//	if err != nil {
	//		context.Logger.Warning("Could not get session")
	//		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	//		return
	//	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "UNKNOWN Encoding", http.StatusBadRequest)
		return
	}

	stopData := stopPost{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&stopData)
	if err != nil {
		context.Logger.Error("Could not parse json %s", err)
		http.Error(w, "Could not parse json", http.StatusBadRequest)
		return
	}

	var reply *Pinger.PollingResponse

	_, err = context.Config.Server.ValidateAuthToken(stopData.ClientId, stopData.ClientContext, stopData.DeviceId, stopData.Token)
	if err != nil {
		reply = &Pinger.PollingResponse{
			Code:    Pinger.PollingReplyError,
			Message: "Token is not valid",
		}
	} else {
		//	if session.Values[SessionVarClientId] != stopData.ClientId {
		//		context.Logger.Error("Client ID %s does not match session", stopData.ClientId)
		//		http.Error(w, "Unknown Client ID", http.StatusForbidden)
		//		return
		//	}
		reply, err = Pinger.StopPoll(&context.Config.Rpc, stopData.ClientId, stopData.ClientContext, stopData.DeviceId)
		if err != nil {
			context.Logger.Error("%s: Error stopping poll %s", stopData.getLogPrefix(), err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	responseData := make(map[string]string)
	switch {
	case reply.Code == Pinger.PollingReplyError:
		responseData["Status"] = "ERROR"
		responseData["Message"] = reply.Message

	case reply.Code == Pinger.PollingReplyOK:
		responseData["Status"] = "OK"
		responseData["Message"] = ""

	case reply.Code == Pinger.PollingReplyWarn:
		responseData["Status"] = "WARN"
		responseData["Message"] = reply.Message

	default:
		context.Logger.Error("%s: Unknown PollingReply Code %d", stopData.getLogPrefix(), reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("%s: Could not json encode reply: %v", stopData.getLogPrefix(), responseData)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}
