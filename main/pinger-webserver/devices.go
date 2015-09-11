package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	PLATFORM_IOS               string = "ios"
	PLATFORM_ANDROID           string = "android"
	EAS_URL_SCHEME             string = "https"
	IMAP_URL_SCHEME            string = "imap"
	PUSH_SERVICE_APNS          string = "APNS"
	PUSH_SERVICE_GCM           string = "GCM"
	MAX_REQUEST_DATA_SIZE             = 10240 // is this enough?
	MAX_NO_CHANGE_REPLY_SIZE          = 10240 // is this enough?
	MAX_OS_VERSION_SIZE               = 32    // is this enough?
	MAX_APP_BUILD_VERSION_SIZE        = 32    // is this enough?
	MAX_APP_BUILD_NUMBER_SIZE         = 32    // is this enough?
	IMAP_LOGIN_CMD                    = "LOGIN"
	IMAP_AUTH_CMD_PLAIN               = "AUTHENTICATE PLAIN"
	IMAP_AUTH_CMD_XOAUTH2             = "AUTHENTICATE XOAUTH2"
	MAX_IMAP_AUTH_CMD_SIZE            = 10240 // As per OAUTH spec - Please use a variable length data type without a specific maximum size to store access tokens.
)

var authTokenKeys map[string][]byte

var clientIdRegex *regexp.Regexp
var deviceIdRegex *regexp.Regexp
var contextRegex *regexp.Regexp
var pushTokenRegex *regexp.Regexp

func init() {
	clientIdRegex = regexp.MustCompile("^(?P<client>us-[a-z]+-[0-9]+:[a-z\\-0-9]+).*$")
	deviceIdRegex = regexp.MustCompile("^(?P<device>Ncho[0-9A-Z]{24})$")
	contextRegex = regexp.MustCompile("^(?P<context>[a-z0-9A-Z]+)$")
	pushTokenRegex = regexp.MustCompile("^(?P<pushtoken>[0-9A-Z]{64})$")
	httpsRouter.HandleFunc("/1/register", registerDevice)
	httpsRouter.HandleFunc("/1/defer", deferPolling)
	httpsRouter.HandleFunc("/1/stop", stopPolling)
	authTokenKeys = make(map[string][]byte)
}

//const SessionVarUserId = "UserId"

// registerPostCredentials and registerPostData are (currently) mirror images
// of Pinger.MailPingInformation and Pinger.MailServerCredentials
// This is so that we can change the json interface without needing to touch
// the underlying Pinger code.
// That being said, there has to be a better way of doing this...
type registerPostData struct {
	ClientId              string
	UserId                string
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
	ExpectedReply          []byte // not used by either EAS or IMAP
	NoChangeReply          []byte
	ResponseTimeout        uint64 // in milliseconds
	WaitBeforeUse          uint64 // in milliseconds
	PushToken              string // platform dependent push token
	PushService            string // APNS, AWS, GCM, etc.
	MaxPollTimeout         uint64 // maximum time to poll. Default is 2 days.
	OSVersion              string
	AppBuildNumber         string
	AppBuildVersion        string
	IMAPAuthenticationBlob string
	IMAPFolderName         string
	IMAPSupportsIdle       bool
	IMAPSupportsExpunge    bool
	IMAPEXISTSCount        uint32
	IMAPUIDNEXT            uint32
}

func getScrubbedLogPrefix(deviceId, userId, context string) string {
	var sUserId, sDeviceId, sContext string
	if isValidUserId(userId) {
		sUserId = userId
	} else {
		sUserId = "INVALID_USERID"
	}
	if isValidDeviceId(deviceId) {
		sDeviceId = deviceId
	} else {
		sDeviceId = "INVALID_DEVICEID"
	}
	if isValidClientContext(context) {
		sContext = context
	} else {
		sContext = "INVALID_CONTEXT"
	}
	return fmt.Sprintf("%s:%s:%s", sDeviceId, sUserId, sContext)
}

func (pd *registerPostData) getLogPrefix() string {
	return getScrubbedLogPrefix(pd.DeviceId, pd.UserId, pd.ClientContext)
}

func isValidUserId(userId string) bool {
	if !govalidator.StringLength(userId, "46", "46") {
		return false
	}
	if !clientIdRegex.MatchString(userId) {
		return false
	}
	return true
}

func isValidDeviceId(deviceId string) bool {
	if !deviceIdRegex.MatchString(deviceId) {
		return false
	}
	return true
}

func isValidClientContext(context string) bool {
	if !govalidator.StringLength(context, "5", "64") {
		return false
	}
	if !contextRegex.MatchString(context) {
		return false
	}
	return true
}

func isValidPushToken(pushService, pushToken string) bool {
	if pushService == PUSH_SERVICE_APNS {
		decodedToken, err := AWS.DecodeAPNSPushToken(pushToken)
		if err != nil {
			return false
		}
		if !pushTokenRegex.MatchString(decodedToken) {
			return false
		}
	} else {
		//TODO - add check for android push token format
		return false
	}
	return true
}

func isValidMailServerCredentials(userName, password string) bool {
	if !govalidator.StringLength(userName, "1", "64") { // is this enough? what regex can we use
		return false
	}
	if !govalidator.StringLength(password, "0", "64") { // is this enough? what regex can we use
		return false
	}
	return true
}

func isMailServerURL(rawurl string) bool {
	url, err := url.ParseRequestURI(rawurl)
	if err != nil {
		return false //Couldn't even parse the rawurl
	}
	if len(url.Scheme) == 0 {
		return false //No Scheme found
	} else if url.Scheme != EAS_URL_SCHEME && url.Scheme != IMAP_URL_SCHEME {
		return false
	}
	if len(url.Host) == 0 {
		return false
	}
	return true
}

func isValidPushService(pushService string) bool {
	if pushService != PUSH_SERVICE_APNS && pushService != PUSH_SERVICE_GCM {
		return false
	}
	return true
}

func isValidFolderName(folderName string, validFolderNames []string) bool {
	for _, f := range validFolderNames {
		if f == folderName {
			return true
		}
	}
	return false
}

func isValidIMAPAuthenticationBlob(blob string) bool {
	decodedBlob, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return false
	} else {
		if !strings.HasPrefix(string(decodedBlob), IMAP_LOGIN_CMD) &&
			!strings.HasPrefix(string(decodedBlob), IMAP_AUTH_CMD_PLAIN) &&
			!strings.HasPrefix(string(decodedBlob), IMAP_AUTH_CMD_XOAUTH2) {
			return false
		} else if len(decodedBlob) > MAX_IMAP_AUTH_CMD_SIZE {
			return false
		}
	}
	return true
}

// Validate validate the structure/information to make sure required information exists.
func (pd *registerPostData) Validate(context *Context) (bool, []string) {
	ok := true
	invalidFields := []string{}
	if !isValidUserId(pd.UserId) {
		ok = false
		invalidFields = append(invalidFields, "UserId")
	}
	if !isValidDeviceId(pd.DeviceId) {
		ok = false
		invalidFields = append(invalidFields, "DeviceId")
	}
	if !isValidClientContext(pd.ClientContext) {
		ok = false
		invalidFields = append(invalidFields, "ClientContextId")
	}
	if pd.Platform != PLATFORM_IOS && pd.Platform != PLATFORM_ANDROID {
		ok = false
		invalidFields = append(invalidFields, "Platform")
	}
	if !isMailServerURL(pd.MailServerUrl) {
		ok = false
		invalidFields = append(invalidFields, "MailServerUrl")
	}
	if pd.ResponseTimeout > 3600000 { //1 hr max ok?
		ok = false
		invalidFields = append(invalidFields, "ResponseTimeout")
	}
	if pd.WaitBeforeUse > 600000 { //10 minutes max ok?
		ok = false
		invalidFields = append(invalidFields, "WaitBeforeUse")
	}
	if pd.MaxPollTimeout == 0 || pd.MaxPollTimeout > Pinger.DefaultMaxPollTimeout { //2 days max
		pd.MaxPollTimeout = Pinger.DefaultMaxPollTimeout
	}
	if !isValidPushService(pd.PushService) {
		ok = false
		invalidFields = append(invalidFields, "PushService")
		invalidFields = append(invalidFields, "PushToken") // we can't validate PushToken if we don't know the service type
	} else {
		if !isValidPushToken(pd.PushService, pd.PushToken) {
			ok = false
			invalidFields = append(invalidFields, "PushToken")
		}
	}
	if len(pd.OSVersion) > MAX_OS_VERSION_SIZE || !govalidator.IsASCII(pd.OSVersion) {
		ok = false
		invalidFields = append(invalidFields, "OSVersion")
	}
	if len(pd.AppBuildVersion) > MAX_APP_BUILD_VERSION_SIZE || !govalidator.IsASCII(pd.AppBuildVersion) {
		ok = false
		invalidFields = append(invalidFields, "AppBuildVersion")
	}
	if len(pd.AppBuildNumber) > MAX_APP_BUILD_NUMBER_SIZE || !govalidator.IsInt(pd.AppBuildNumber) {
		ok = false
		invalidFields = append(invalidFields, "AppBuildNumber")
	}
	if pd.Protocol == Pinger.MailClientActiveSync {
		if !isValidMailServerCredentials(pd.MailServerCredentials.Username, pd.MailServerCredentials.Password) {
			ok = false
			invalidFields = append(invalidFields, "MailServerCredentials")
		}
		// TODO - validate HTTP Headers
		//"HttpHeaders":{"User-Agent":"Apple-iPhone4C1/1208.321",
		// "MS-ASProtocolVersion":"14.1","Content-Length":"53",
		// "Content-Type":"application/vnd.ms-sync.wbxml"}
		// there can be other stuff too.
		//TODO - validate WBXML
		if len(pd.RequestData) > MAX_REQUEST_DATA_SIZE {
			ok = false
			invalidFields = append(invalidFields, "RequestData")
		}
		//TODO - validate WBXML
		if len(pd.RequestData) > MAX_NO_CHANGE_REPLY_SIZE {
			ok = false
			invalidFields = append(invalidFields, "NoChangeReply")
		}
		pd.ExpectedReply = nil
		pd.IMAPAuthenticationBlob = ""
		pd.IMAPFolderName = ""
		pd.IMAPSupportsIdle = false
		pd.IMAPSupportsExpunge = false
		pd.IMAPEXISTSCount = 0
		pd.IMAPUIDNEXT = 0
	} else if pd.Protocol == Pinger.MailClientIMAP {
		pd.MailServerCredentials.Username = "" // the IMAP creds aren't passed in this way
		pd.MailServerCredentials.Password = ""
		pd.HttpHeaders = nil
		pd.RequestData = nil
		pd.NoChangeReply = nil
		pd.ExpectedReply = nil
		if !isValidIMAPAuthenticationBlob(pd.IMAPAuthenticationBlob) {
			ok = false
			invalidFields = append(invalidFields, "IMAPAuthenticationBlob")
		}
		if !isValidFolderName(pd.IMAPFolderName, context.Config.Server.IMAPFolderNames) {
			ok = false
			invalidFields = append(invalidFields, "IMAPFolderName")
		}
		// no checks needed for the following as their types are enough
		//IMAPSupportsIdle       bool
		//IMAPSupportsExpunge    bool
		//IMAPEXISTSCount        uint
		//IMAPUIDNEXT            uint
	} else {
		ok = false
		invalidFields = append(invalidFields, "Protocol")
	}
	return ok, invalidFields
}

func (pd *registerPostData) checkForMissingFields(logger *Logging.Logger) (bool, []string) {
	ok := true
	MissingFields := []string{}
	if pd.UserId == "" {
		if pd.ClientId != "" { // old client
			pd.UserId = pd.ClientId
			logger.Info("%s: Old client using ClientId (%s) instead of UserId.", pd.getLogPrefix(), pd.ClientId)
		} else {
			MissingFields = append(MissingFields, "UserId")
			ok = false
		}
	}
	if pd.DeviceId == "" {
		MissingFields = append(MissingFields, "DeviceId")
		ok = false
	}
	if pd.MailServerUrl == "" {
		MissingFields = append(MissingFields, "MailServerUrl")
		ok = false
	}
	if pd.ClientContext == "" {
		MissingFields = append(MissingFields, "ClientContext")
		ok = false
	}
	if pd.Protocol == Pinger.MailClientActiveSync {
		if len(pd.RequestData) <= 0 {
			MissingFields = append(MissingFields, "RequestData")
			ok = false
		}
		if len(pd.NoChangeReply) <= 0 {
			MissingFields = append(MissingFields, "NoChangeReply")
			ok = false
		}
	} else if pd.Protocol == Pinger.MailClientIMAP {
		if len(pd.IMAPAuthenticationBlob) <= 0 {
			MissingFields = append(MissingFields, "IMAPAuthenticationBlob")
			ok = false
		}
		if len(pd.IMAPFolderName) <= 0 {
			MissingFields = append(MissingFields, "IMAPFolderName")
			ok = false
		}
		if pd.IMAPEXISTSCount < 0 {
			MissingFields = append(MissingFields, "IMAPEXISTSCount")
			ok = false
		}
		if pd.IMAPUIDNEXT < 0 {
			MissingFields = append(MissingFields, "IMAPUIDNEXT")
			ok = false
		}
	} else {
		MissingFields = append(MissingFields, "Protocol")
		ok = false
	}
	return ok, MissingFields
}

func (pd *registerPostData) AsMailInfo(sessionId string) *Pinger.MailPingInformation {
	// there's got to be a better way to do this...
	pi := Pinger.MailPingInformation{}
	pi.UserId = pd.UserId
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
	pi.ResponseTimeout = pd.ResponseTimeout
	pi.WaitBeforeUse = pd.WaitBeforeUse
	pi.PushToken = pd.PushToken
	pi.PushService = pd.PushService
	pi.MaxPollTimeout = pd.MaxPollTimeout
	pi.OSVersion = pd.OSVersion
	pi.AppBuildNumber = pd.AppBuildNumber
	pi.AppBuildVersion = pd.AppBuildVersion
	pi.IMAPAuthenticationBlob = pd.IMAPAuthenticationBlob
	pi.IMAPFolderName = pd.IMAPFolderName
	pi.IMAPSupportsIdle = pd.IMAPSupportsIdle
	pi.IMAPSupportsExpunge = pd.IMAPSupportsExpunge
	pi.IMAPEXISTSCount = pd.IMAPEXISTSCount
	pi.IMAPUIDNEXT = pd.IMAPUIDNEXT

	pi.SessionId = sessionId

	return &pi
}

func makeSessionId(token string) (string, error) {
	ha := sha256.Sum256([]byte(token))
	myId := make([]byte, 16)
	n := hex.Encode(myId, ha[0:8])
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
	ok, missingFields := postInfo.checkForMissingFields(context.Logger)
	if ok == false {
		context.Logger.Warning("%s: Missing non-optional data: %s", postInfo.getLogPrefix(), strings.Join(missingFields, ","))
		responseError(w, InvalidData, strings.Join(missingFields, ","))
		return
	}
	ok, invalidFields := postInfo.Validate(context)
	if ok == false {
		context.Logger.Warning("%s: Invalid data: %s", postInfo.getLogPrefix(), strings.Join(invalidFields, ","))
		responseError(w, InvalidData, strings.Join(invalidFields, ","))
		return
	}
	token, key, err := context.Config.Server.CreateAuthToken(postInfo.UserId, postInfo.ClientContext, postInfo.DeviceId)
	if err != nil {
		context.Logger.Error("%s: error creating token %s", postInfo.getLogPrefix(), err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authTokenKeys[token] = key

	//	session.Values[SessionVarUserId] = postInfo.UserId
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
	UserId        string
	ClientContext string
	DeviceId      string
	Timeout       uint64
	Token         string

	logPrefix string
}

func (dp *deferPost) getLogPrefix() string {
	return getScrubbedLogPrefix(dp.DeviceId, dp.UserId, dp.ClientContext)
}

// Validate validate the structure/information to make sure required information exists.
func (dp *deferPost) Validate(context *Context) (bool, []string) {
	ok := true
	invalidFields := []string{}
	if dp.UserId == "" || !isValidUserId(dp.UserId) {
		ok = false
		invalidFields = append(invalidFields, "UserId")
	}
	if dp.DeviceId == "" || !isValidDeviceId(dp.DeviceId) {
		ok = false
		invalidFields = append(invalidFields, "DeviceId")
	}
	if dp.ClientContext == "" || !isValidClientContext(dp.ClientContext) {
		ok = false
		invalidFields = append(invalidFields, "ClientContextId")
	}
	if dp.Timeout > 600000 { //10 minutes max ok?
		ok = false
		invalidFields = append(invalidFields, "Timeout")
	}
	return ok, invalidFields
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
	if deferData.UserId == "" && deferData.ClientId != "" { // old client
		deferData.UserId = deferData.ClientId
		context.Logger.Info("%s: Old client using ClientId (%s) instead of UserId.", deferData.getLogPrefix(), deferData.ClientId)
	}
	ok, invalidFields := deferData.Validate(context)
	if ok == false {
		context.Logger.Warning("%s: Invalid data: %s", deferData.getLogPrefix(), strings.Join(invalidFields, ","))
		responseError(w, InvalidData, strings.Join(invalidFields, ","))
		return
	}
	key, ok := authTokenKeys[deferData.Token]
	if !ok {
		reply = &Pinger.PollingResponse{
			Code:    Pinger.PollingReplyError,
			Message: "Token is not valid",
		}
	} else {
		valid := context.Config.Server.ValidateAuthToken(deferData.UserId, deferData.ClientContext, deferData.DeviceId, deferData.Token, key)
		if !valid {
			reply = &Pinger.PollingResponse{
				Code:    Pinger.PollingReplyError,
				Message: "Token is not valid",
			}
		} else {
			//	if session.Values[SessionVarUserId] != deferData.UserId {
			//		context.Logger.Error("Client ID %s does not match session", deferData.UserId)
			//		http.Error(w, "Unknown Client ID", http.StatusForbidden)
			//		return
			//	}
			context.Logger.Debug("%s: Token is valid", deferData.getLogPrefix())
			// deferData.Timeout is not sent by the client. It defaults to 0
			reply, err = Pinger.DeferPoll(&context.Config.Rpc, deferData.UserId, deferData.ClientContext, deferData.DeviceId, deferData.Timeout)
			if err != nil {
				context.Logger.Error("%s: Error deferring poll %s", deferData.getLogPrefix(), err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
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
	UserId        string
	ClientContext string
	DeviceId      string
	Token         string

	logPrefix string
}

// Validate validate the structure/information to make sure required information exists.
func (sp *stopPost) Validate(context *Context) (bool, []string) {
	ok := true
	invalidFields := []string{}
	if sp.UserId == "" || !isValidUserId(sp.UserId) {
		ok = false
		invalidFields = append(invalidFields, "UserId")
	}
	if sp.DeviceId == "" || !isValidDeviceId(sp.DeviceId) {
		ok = false
		invalidFields = append(invalidFields, "DeviceId")
	}
	if sp.ClientContext == "" || !isValidClientContext(sp.ClientContext) {
		ok = false
		invalidFields = append(invalidFields, "ClientContextId")
	}
	return ok, invalidFields
}

func (sp *stopPost) getLogPrefix() string {
	return getScrubbedLogPrefix(sp.DeviceId, sp.UserId, sp.ClientContext)
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
	if stopData.UserId == "" && stopData.ClientId != "" { // old client
		stopData.UserId = stopData.ClientId
		context.Logger.Info("%s: Old client using ClientId (%s) instead of UserId.", stopData.getLogPrefix(), stopData.ClientId)
	}
	ok, invalidFields := stopData.Validate(context)
	if ok == false {
		context.Logger.Warning("%s: Invalid data: %s", stopData.getLogPrefix(), strings.Join(invalidFields, ","))
		responseError(w, InvalidData, strings.Join(invalidFields, ","))
		return
	}
	key, ok := authTokenKeys[stopData.Token]
	if !ok {
		reply = &Pinger.PollingResponse{
			Code:    Pinger.PollingReplyError,
			Message: "Token is not valid",
		}
	} else {
		valid := context.Config.Server.ValidateAuthToken(stopData.UserId, stopData.ClientContext, stopData.DeviceId, stopData.Token, key)
		if !valid {
			reply = &Pinger.PollingResponse{
				Code:    Pinger.PollingReplyError,
				Message: "Token is not valid",
			}
		} else {
			//	if session.Values[SessionVarUserId] != stopData.UserId {
			//		context.Logger.Error("User ID %s does not match session", stopData.UserId)
			//		http.Error(w, "Unknown User ID", http.StatusForbidden)
			//		return
			//	}
			context.Logger.Debug("%s: Deleting key for token %s", stopData.getLogPrefix(), stopData.Token)
			delete(authTokenKeys, stopData.Token)
			reply, err = Pinger.StopPoll(&context.Config.Rpc, stopData.UserId, stopData.ClientContext, stopData.DeviceId)
			if err != nil {
				context.Logger.Error("%s: Error stopping poll %s", stopData.getLogPrefix(), err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
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
