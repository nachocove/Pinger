package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nachocove/Pinger/Pinger"
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
	MaxPollTimeout         int64  // maximum time to poll. Default is 2 days.
	OSVersion              string
	AppBuildNumber         string
	AppBuildVersion        string
}

// Validate validate the structure/information to make sure required information exists.
func (pd *registerPostData) Validate() (bool, []string) {
	ok := true
	MissingFields := []string{}
	if pd.ClientId == "" {
		MissingFields = append(MissingFields, "ClientId")
		ok = false
	}
	if pd.MailServerUrl == "" {
		MissingFields = append(MissingFields, "MailServerUrl")
		ok = false
	}
	if len(pd.HttpRequestData) <= 0 {
		MissingFields = append(MissingFields, "HttpRequestData")
		ok = false
	}
	if len(pd.HttpNoChangeReply) <= 0 {
		MissingFields = append(MissingFields, "HttpNoChangeReply")
		ok = false
	}
	if pd.ClientContext == "" {
		MissingFields = append(MissingFields, "ClientContext")
		ok = false
	}
	return ok, MissingFields
}

func (pd *registerPostData) AsMailInfo() *Pinger.MailPingInformation {
	// there's got to be a better way to do this...
	pi := Pinger.MailPingInformation{}
	pi.ClientId = pd.ClientId
	pi.ClientContext = pd.ClientContext
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
	if pd.MaxPollTimeout == 0 {
		pi.MaxPollTimeout = Pinger.DefaultMaxPollTimeout
	} else {
		pi.MaxPollTimeout = pd.MaxPollTimeout
	}
	pi.OSVersion = pd.OSVersion
	pi.AppBuildNumber = pd.AppBuildNumber
	pi.AppBuildVersion = pd.AppBuildVersion
	return &pi
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
	postInfo := registerPostData{}
	switch {
	case encodingStr == "application/json" || encodingStr == "text/json":
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
		context.Logger.Warning("Missing non-optional data: %s", strings.Join(missingFields, ","))
		responseError(w, MissingRequiredData, strings.Join(missingFields, ","))
		return
	}

	//	session.Values[SessionVarClientId] = postInfo.ClientId

	reply, err := Pinger.StartPoll(context.RpcConnectString, postInfo.AsMailInfo())
	if err != nil {
		context.Logger.Warning("Could not re/start polling for device %s: %s", postInfo.ClientId, err)
		responseError(w, RPCServerError, "")
		return
	}
	context.Logger.Debug("Re/Started Polling for %s", postInfo.ClientId)

	//	err = session.Save(r, w)
	//	if err != nil {
	//		context.Logger.Warning("Could not save session")
	//		responseError(w, SaveSessionError, "")
	//		return
	//	}
	responseData := make(map[string]string)

	switch {
	case reply.Code == Pinger.PollingReplyOK:
		responseData["Token"] = reply.Token
		responseData["Status"] = "OK"
		responseData["Message"] = ""

	case reply.Code == Pinger.PollingReplyError:
		responseData["Status"] = "ERROR"
		responseData["Message"] = reply.Message

	case reply.Code == Pinger.PollingReplyWarn:
		responseData["Token"] = reply.Token
		responseData["Status"] = "WARN"
		responseData["Message"] = reply.Message

	default:
		context.Logger.Error("Unknown PollingReply Code %d", reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("Could not json encode reply: %v", responseData)
		responseError(w, JSONEncodeError, "")
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}

type deferPost struct {
	ClientId string
	Timeout  int64
	Token    string
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
	//	if session.Values[SessionVarClientId] != deferData.ClientId {
	//		context.Logger.Error("Client ID %s does not match session", deferData.ClientId)
	//		http.Error(w, "Unknown Client ID", http.StatusForbidden)
	//		return
	//	}
	reply, err := Pinger.DeferPoll(context.RpcConnectString, deferData.ClientId, deferData.Timeout, deferData.Token)
	if err != nil {
		context.Logger.Error("Error deferring poll %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		context.Logger.Error("Unknown PollingReply Code %d", reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

type stopPost struct {
	ClientId string
	Token    string
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
	//	if session.Values[SessionVarClientId] != stopData.ClientId {
	//		context.Logger.Error("Client ID %s does not match session", stopData.ClientId)
	//		http.Error(w, "Unknown Client ID", http.StatusForbidden)
	//		return
	//	}
	reply, err := Pinger.StopPoll(context.RpcConnectString, stopData.ClientId, stopData.Token)
	if err != nil {
		context.Logger.Error("Error stopping poll %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		context.Logger.Error("Unknown PollingReply Code %d", reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
