package main

const (
	YahooRegisterRequest = "{\"MailServerUrl\":\"imap://EMAIL_SERVER_NAME:993\",\"Protocol\":\"IMAP\",\"Platform\":\"ios\",\"ResponseTimeout\":600000,\"WaitBeforeUse\":60000,\"PushToken\":\"+5vEJ8vmOmbrp+HIRwvHYdLSWoqol1SWBceKDktgkKM=\",\"PushService\":\"APNS\",\"MailServerCredentials\":null,\"HttpHeaders\":{},\"RequestData\":null,\"ExpectedReply\":null,\"NoChangeReply\":null,\"IMAPAuthenticationBlob\":\"QVVUSEVOVElDQVRFIFBMQUlOCkFHRjZhVzF1WVdOb2IwQjVZV2h2Ynk1amIyMEFablZ1Ym5rdVpHbGw=\",\"IMAPFolderName\":\"INBOX\",\"IMAPSupportsIdle\":false,\"IMAPSupportsExpunge\":false,\"IMAPEXISTSCount\":0,\"IMAPUIDNEXT\":0,\"UserId\":\"us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49\",\"ClientId\":\"NchoD819080CX2F92X407CXACAC5\",\"DeviceId\":\"NchoD819080CX2F92X407CXACAC5\",\"ClientContext\":\"0964b758\",\"OSVersion\":\"8.4.1\",\"AppBuildVersion\":\"DEV[azimozakil]\",\"AppBuildNumber\":\"1391\"}"
	GmailRegisterRequest = "{\"MailServerUrl\":\"imap://EMAIL_SERVER_NAME:993\",\"Protocol\":\"IMAP\",\"Platform\":\"ios\",\"ResponseTimeout\":600000,\"WaitBeforeUse\":60000,\"PushToken\":\"+5vEJ8vmOmbrp+HIRwvHYdLSWoqol1SWBceKDktgkKM=\",\"PushService\":\"APNS\",\"MailServerCredentials\":null,\"HttpHeaders\":{},\"RequestData\":null,\"ExpectedReply\":null,\"NoChangeReply\":null,\"IMAPAuthenticationBlob\":\"QVVUSEVOVElDQVRFIFhPQVVUSDIgZFhObGNqMWhlbWx0Ym1GamFHOUFaMjFoYVd3dVkyOXRBV0YxZEdnOVFtVmhjbVZ5SUhsaE1qa3VNM2RJT1ZWWE9FdE1Va2hGTms1V1NYWnZiMDFGZG05bFVHOTJhR2x3Um1GNFVHeDVUVzVHVXpaS1prUktjRlJsZWpocWJGWlBUV05LVG5kSlFYbHBSRFZwWTBNQkFRPT0=\",\"IMAPFolderName\":\"INBOX\",\"IMAPSupportsIdle\":true,\"IMAPSupportsExpunge\":true,\"IMAPEXISTSCount\":0,\"IMAPUIDNEXT\":0,\"UserId\":\"us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49\",\"ClientId\":\"NchoD819080CX2F92X407CXACAC5\",\"DeviceId\":\"NchoD819080CX2F92X407CXACAC5\",\"ClientContext\":\"812e04ee\",\"OSVersion\":\"8.4.1\",\"AppBuildVersion\":\"DEV[azimozakil]\",\"AppBuildNumber\":\"1391\"}"
)

type MockIMAPClient struct {
	MockClient
}

func getRandomRegisterRequest() string {
	if rng.Intn(2) == 0 {
		logger.Info("Yahoooo.ooooo")
		return YahooRegisterRequest
	} else {
		logger.Info("Gmail.ooooo")
		return GmailRegisterRequest
	}
}

func (m *MockIMAPClient) Register() error {
	return m.RegisterWithRequest(getRandomRegisterRequest())
}
