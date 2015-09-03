package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

const (
	EASRegisterRequest = "{\"MailServerUrl\":\"https://ltmail.officeburrito.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=test2@officeburrito.com&DeviceId=NchoD819080CX2F92X407CXACAC5&DeviceType=iPhone\",\"Protocol\":\"ActiveSync\",\"Platform\":\"ios\",\"ResponseTimeout\":600000,\"WaitBeforeUse\":60000,\"PushToken\":\"+5vEJ8vmOmbrp+HIRwvHYdLSWoqol1SWBceKDktgkKM=\",\"PushService\":\"APNS\",\"MailServerCredentials\":{\"Username\":\"test2@officeburrito.com\",\"Password\":\"Password1\"},\"HttpHeaders\":{\"User-Agent\":\"Apple-iPhone4C1/1208.321\",\"MS-ASProtocolVersion\":\"14.1\",\"Content-Length\":\"52\",\"Content-Type\":\"application/vnd.ms-sync.wbxml\"},\"RequestData\":\"AwFqAAANRUgDNjAwAAFJSksDNgABTANFbWFpbAABAUpLAzIAAUwDQ2FsZW5kYXIAAQEBAQ==\",\"ExpectedReply\":null,\"NoChangeReply\":\"AwFqAAANRUcDMQABAQ==\",\"IMAPAuthenticationBlob\":null,\"IMAPFolderName\":null,\"IMAPSupportsIdle\":false,\"IMAPSupportsExpunge\":false,\"IMAPEXISTSCount\":0,\"IMAPUIDNEXT\":0,\"UserId\":\"us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49\",\"ClientId\":\"NchoD819080CX2F92X407CXACAC5\",\"DeviceId\":\"NchoD819080CX2F92X407CXACAC5\",\"ClientContext\":\"9b37e8a8\",\"OSVersion\":\"8.4.1\",\"AppBuildVersion\":\"DEV[azimozakil]\",\"AppBuildNumber\":\"1391\"}"
)

type MockEASClient struct {
	lta *LTAccount
}

func (rpData *registerPostData) updateRegisterData(lta *LTAccount) {
	//'UserId':'us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49',
	//'ClientId':'NchoD819080CX2F92X407CXACAC5',
	//'DeviceId':'NchoD819080CX2F92X407CXACAC5',
	//'ClientContext':'9b37e8a8',
	rpData.ClientContext = lta.accountName
}

func (dpData *deferPostData) updateDeferData(lta *LTAccount) {
	dpData.ClientContext = lta.accountName
	dpData.Token = lta.token
}

func (spData *stopPostData) updateStopData(lta *LTAccount) {
	spData.ClientContext = lta.accountName
	spData.Token = lta.token
}

func (m *MockEASClient) init(lta *LTAccount) {
	m.lta = lta
}

func (m *MockEASClient) Register() error {
	rpData := ParseRegisterJSON([]byte(EASRegisterRequest))
	rpData.updateRegisterData(m.lta)
	err, response := m.doRequestResponse(RegisterAPI, GetRegisterJSONBytes(rpData))
	rrData := ParseRegisterResponse(response)
	m.lta.token = rrData.Token
	logger.Info("RegisterResponse: %s", rrData.String())
	return err
}

func (m *MockEASClient) Defer() error {
	dpData := ParseDeferJSON([]byte(DeferRequest))
	dpData.updateDeferData(m.lta)
	err, response := m.doRequestResponse(DeferAPI, GetDeferJSONBytes(dpData))
	drData := ParseDeferResponse(response)
	logger.Info("DeferResponse %s", drData.String())
	return err
}

func (m *MockEASClient) Stop() error {
	spData := ParseStopJSON([]byte(StopRequest))
	spData.updateStopData(m.lta)
	err, response := m.doRequestResponse(StopAPI, GetStopJSONBytes(spData))
	srData := ParseStopResponse(response)
	logger.Info("StopResponse %s", srData.String())
	return err
}

func (m *MockEASClient) doRequestResponse(requestAPI string, postData []byte) (error, []byte) {
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
