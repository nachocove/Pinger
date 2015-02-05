package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/nachocove/Pinger/Pinger"
)

func init() {
	httpsRouter.HandleFunc("/register", registerDevice)
	httpsRouter.HandleFunc("/defer", deferPolling)
}

const SessionVarClientId = "ClientId"

// registerPostCredentials and registerPostData are (currently) mirror images
// of Pinger.MailPingInformation and Pinger.MailServerCredentials
// This is so that we can change the json interface without needing to touch
// the underlying Pinger code.
// That being said, there has to be a better way of doing this...
type registerPostCredentials struct {
	Username string
	Password string
}
type registerPostData struct {
	ClientId               string
	Platform               string
	MailServerUrl          string
	MailServerCredentials  registerPostCredentials
	Protocol               string
	HttpHeaders            map[string]string // optional
	HttpRequestData        []byte
	HttpExpectedReply      []byte
	HttpNoChangeReply      []byte
	CommandTerminator      []byte // used by imap
	CommandAcknowledgement []byte // used by imap
	ResponseTimeout        int64  // in seconds
	WaitBeforeUse          int64  // in seconds
	PushToken              string // platform dependent push token
	PushService            string // APNS, AWS, GCM, etc.
}

// Validate validate the structure/information to make sure required information exists.
func (pd *registerPostData) Validate() bool {
	return (pd.ClientId != "" &&
		pd.MailServerUrl != "" &&
		len(pd.HttpRequestData) > 0 &&
		len(pd.HttpExpectedReply) > 0)
}

func (pd *registerPostData) AsMailInfo() *Pinger.MailPingInformation {
	// there's got to be a better way to do this...
	pi := Pinger.MailPingInformation{}
	pi.ClientId = pd.ClientId
	pi.Platform = pd.Platform
	pi.MailServerUrl = pd.MailServerUrl
	pi.MailServerCredentials.Username = pd.MailServerCredentials.Username
	pi.MailServerCredentials.Password = pd.MailServerCredentials.Password
	pi.Protocol = pd.Protocol
	pi.HttpHeaders = pd.HttpHeaders
	pi.HttpRequestData = pd.HttpRequestData
	pi.HttpExpectedReply = pd.HttpExpectedReply
	pi.HttpNoChangeReply = pd.HttpNoChangeReply
	pi.CommandTerminator = pd.CommandTerminator
	pi.CommandAcknowledgement = pd.CommandAcknowledgement
	pi.ResponseTimeout = pd.ResponseTimeout
	pi.WaitBeforeUse = pd.WaitBeforeUse
	pi.PushToken = pd.PushToken
	pi.PushService = pd.PushService
	return &pi
}

func registerDevice(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	session, err := context.SessionStore.Get(r, "pinger-session")
	if err != nil {
		context.Logger.Warning("Could not get session")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	requestData, err := httputil.DumpRequest(r, true)
	if context.Config.Global.DumpRequests && err == nil {
		context.Logger.Debug("Received request\n%s", string(requestData))
	}

	encodingStr := r.Header.Get("Content-Type")
	postInfo := registerPostData{}
	switch {
	case encodingStr == "application/json" || encodingStr == "text/json":
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&postInfo)
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
	
	if postInfo.Validate() == false {
		context.Logger.Warning("Missing non-optional data")
		responseError(w, MISSING_REQUIRED_DATA)
		return
	}

	session.Values[SessionVarClientId] = postInfo.ClientId

	_, err = Pinger.StartPoll(context.RpcConnectString, postInfo.AsMailInfo())
	if err != nil {
		context.Logger.Warning("Could not re/start polling for device %s: %s", postInfo.ClientId, err)
		responseError(w, RPC_SERVER_ERROR)
		return
	}
	context.Logger.Debug("Re/Started Polling for %s", postInfo.ClientId)

	err = session.Save(r, w)
	if err != nil {
		context.Logger.Warning("Could not save session")
		responseError(w, SAVE_SESSION_ERROR)
		return
	}
	responseData := make(map[string]string)
	//responseData["Token"] = token
	responseData["Status"] = "OK"
	responseData["Message"] = ""

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("Could not json encode reply: %v", responseData)
		responseError(w, JSON_ENCODE_ERROR)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}

type deferPost struct {
	ClientId  string
	StopToken string
}

func deferPolling(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	session, err := context.SessionStore.Get(r, "pinger-session")
	if err != nil {
		context.Logger.Warning("Could not get session")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "UNKNOWN Encoding", http.StatusBadRequest)
		return
	}

	deferData := deferPost{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&deferData)
	if err != nil {
		context.Logger.Error("Could not parse json %s", err)
		http.Error(w, "Could not parse json", http.StatusBadRequest)
		return
	}
	if session.Values[SessionVarClientId] != deferData.ClientId {
		context.Logger.Error("Client ID %s does not match session", deferData.ClientId)
		http.Error(w, "Unknown Client ID", http.StatusForbidden)
		return
	}
	err = Pinger.DeferPoll(context.RpcConnectString, deferData.ClientId, deferData.StopToken)
	if err != nil {
		context.Logger.Error("Error deferring poll %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responseData := make(map[string]string)
	responseData["Status"] = "OK"
	responseData["Message"] = ""

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("Could not json encode reply: %v", responseData)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}
