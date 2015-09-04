package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	RegisterResponse = "{\"Message\":\"\",\"Status\":\"OK\",\"Token\":\"XF9E+5S+WPJzOiOhjCWrTnYZSbgRCdkCVN6KwoOXf/9aWboWsxHk1dFBBSIR3/741Kj9W5aRdfYHdsd8Ul6G1h5FP/yatNeRD7COudSzn8Eh9wNiOhssmgACYIkEnO1uV8IIXXs3py8/mcoLhfbuhqNK\"}"
	DeferRequest     = "{\"ResponseTimeout\":600000,\"Token\":\"XF9E+5S+WPJzOiOhjCWrTnYZSbgRCdkCVN6KwoOXf/9aWboWsxHk1dFBBSIR3/741Kj9W5aRdfYHdsd8Ul6G1h5FP/yatNeRD7COudSzn8Eh9wNiOhssmgACYIkEnO1uV8IIXXs3py8/mcoLhfbuhqNK\",\"UserId\":\"us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49\",\"ClientId\":\"NchoD819080CX2F92X407CXACAC5\",\"DeviceId\":\"NchoD819080CX2F92X407CXACAC5\",\"ClientContext\":\"0964b758\",\"OSVersion\":\"8.4.1\",\"AppBuildVersion\":\"DEV[azimozakil]\",\"AppBuildNumber\":\"1391\"}"
	DeferResponse    = "{\"Message\":\"\",\"Status\":\"OK\"}"
	StopRequest      = "{\"UserId\": \"us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49\", \"DeviceId\": \"NchoD819080CX2F92X407CXACAC5\", \"ClientContext\": \"9b37e8a8\", \"Token\": \"XF9E+5S+WPJzOiOhjCWrTnYZSbgRCdkCVN6KwoOXf/9aWboWsxHk1dFBBSIR3/741Kj9W5aRdfYHdsd8Ul6G1h5FP/yatNeRD7COudSzn8Eh9wNiOhssmgACYIkEnO1uV8IIXXs3py8/mcoLhfbuhqNK\"}"
	StopResponse     = "{\"Message\":\"\",\"Status\":\"OK\"}"
	RegisterAPI      = "register"
	DeferAPI         = "defer"
	StopAPI          = "stop"
)

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
	IMAPAuthenticationBlob string
	IMAPFolderName         string
	IMAPSupportsIdle       bool
	IMAPSupportsExpunge    bool
	IMAPEXISTSCount        int
	IMAPUIDNEXT            int
	logPrefix              string
}
type registerResponseData struct {
	Message string
	Status  string
	Token   string
}

type deferPostData struct {
	Token           string
	ResponseTimeout int64 // in milliseconds
	ClientId        string
	UserId          string
	ClientContext   string
	DeviceId        string
	OSVersion       string
	AppBuildNumber  string
	AppBuildVersion string
}

type deferResponseData struct {
	Message string
	Status  string
}

type stopPostData struct {
	Token         string
	UserId        string
	ClientContext string
	DeviceId      string
}

type stopResponseData struct {
	Message string
	Status  string
}

type MockClientInterface interface {
	init(lta *LTAccount)
	Register() error
	Defer() error
	Stop() error
}

type MockClient struct {
	lta *LTAccount
}

func (rrd registerResponseData) String() string {
	return fmt.Sprintf("Message:%s, Status:%s, Token:%s", rrd.Message, rrd.Status, rrd.Token)
}

func (rd deferResponseData) String() string {
	return fmt.Sprintf("Message:%s, Status:%s, ", rd.Message, rd.Status)
}

func (rd stopResponseData) String() string {
	return fmt.Sprintf("Message:%s, Status:%s, ", rd.Message, rd.Status)
}

func ParseRegisterJSON(registerData []byte) registerPostData {
	registerPost := registerPostData{}
	err := json.Unmarshal(registerData, &registerPost)
	if err != nil {
		logger.Warning("error: %v for %s", err, string(registerData))
	}
	return registerPost
}

func ParseDeferJSON(deferData []byte) deferPostData {
	deferPost := deferPostData{}
	err := json.Unmarshal(deferData, &deferPost)
	if err != nil {
		logger.Warning("error: %v", err)
	}
	return deferPost
}

func ParseStopJSON(stopData []byte) stopPostData {
	stopPost := stopPostData{}
	err := json.Unmarshal(stopData, &stopPost)
	if err != nil {
		logger.Warning("error: %v", err)
	}
	return stopPost
}

func ParseRegisterResponse(response []byte) registerResponseData {
	registerResponse := registerResponseData{}
	err := json.Unmarshal(response, &registerResponse)
	if err != nil {
		logger.Warning("error: %v", err)
	}
	return registerResponse
}

func ParseDeferResponse(response []byte) deferResponseData {
	deferResponse := deferResponseData{}
	err := json.Unmarshal(response, &deferResponse)
	if err != nil {
		logger.Warning("error: %v", err)
	}
	return deferResponse
}

func ParseStopResponse(response []byte) stopResponseData {
	stopResponse := stopResponseData{}
	err := json.Unmarshal(response, &stopResponse)
	if err != nil {
		logger.Warning("error: %v", err)
	}
	return stopResponse
}

func GetRegisterJSONBytes(rpData registerPostData) []byte {
	JSONBytes, err := json.Marshal(rpData)
	if err != nil {
		return nil
	}
	return JSONBytes
}

func GetDeferJSONBytes(dpData deferPostData) []byte {
	JSONBytes, err := json.Marshal(dpData)
	if err != nil {
		return nil
	}
	return JSONBytes
}

func GetStopJSONBytes(spData stopPostData) []byte {
	JSONBytes, err := json.Marshal(spData)
	if err != nil {
		return nil
	}
	return JSONBytes
}

func (rpData *registerPostData) updateRegisterData(lta *LTAccount) {
	rpData.ClientContext = lta.accountName
	rpData.MailServerUrl = strings.Replace(rpData.MailServerUrl, "EMAIL_SERVER_NAME", lta.emailServerName, 1)
}

func (dpData *deferPostData) updateDeferData(lta *LTAccount) {
	dpData.ClientContext = lta.accountName
	dpData.Token = lta.token
}

func (spData *stopPostData) updateStopData(lta *LTAccount) {
	spData.ClientContext = lta.accountName
	spData.Token = lta.token
}

func (m *MockClient) init(lta *LTAccount) {
	m.lta = lta
}

func (m *MockClient) RegisterWithRequest(registerRequest string) error {
	rpData := ParseRegisterJSON([]byte(registerRequest))
	rpData.updateRegisterData(m.lta)
	err, response := m.doRequestResponse(RegisterAPI, GetRegisterJSONBytes(rpData))
	rrData := ParseRegisterResponse(response)
	m.lta.token = rrData.Token
	logger.Info("RegisterResponse: %s", rrData.String())
	return err
}

func (m *MockClient) Defer() error {
	dpData := ParseDeferJSON([]byte(DeferRequest))
	dpData.updateDeferData(m.lta)
	err, response := m.doRequestResponse(DeferAPI, GetDeferJSONBytes(dpData))
	drData := ParseDeferResponse(response)
	logger.Info("DeferResponse %s", drData.String())
	return err
}

func (m *MockClient) Stop() error {
	spData := ParseStopJSON([]byte(StopRequest))
	spData.updateStopData(m.lta)
	err, response := m.doRequestResponse(StopAPI, GetStopJSONBytes(spData))
	srData := ParseStopResponse(response)
	logger.Info("StopResponse %s", srData.String())
	return err
}

func (m *MockClient) doRequestResponse(requestAPI string, postData []byte) (error, []byte) {
	//logger.Debug("Starting doRequestResponse")
	defer func() {
		//logger.Debug("Exiting doRequestResponse")
	}()

	var err error
	URL := m.lta.user.pingerURL + "/" + requestAPI
	logger.Info("Connection to %s", URL)
	logger.Info("Sending Body %s", postData)
	requestBody := bytes.NewReader(postData)
	req, err := http.NewRequest("POST", URL, requestBody)
	if err != nil {
		logger.Error("Failed to create request: %s", err.Error())
		return err, nil
	}

	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "application/json")
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	//logger.Debug("Making Connection to pinger")
	response, err := m.lta.httpClient.Do(req)
	if err != nil {
		logger.Error("Error doing post %s", err)
		return err, nil
	}
	if response.StatusCode != 200 {
		logger.Error("Did not get 200 status code: %d", response.StatusCode)
		return fmt.Errorf("Did not get 200 status code: %d", response.StatusCode), nil
	}
	var responseBytes []byte
	responseBytes = make([]byte, 10240) // read up to 10K

	n, err := response.Body.Read(responseBytes)
	if err != nil && err != io.EOF {
		logger.Error("Failed to read response: %s", err.Error())
		return err, nil
	}
	response.Body.Close()
	//logger.Debug("response [%s]", string(responseBytes[:n]))
	return nil, responseBytes[:n]
}
