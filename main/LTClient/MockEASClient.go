package main

import ()

const (
	EASRegisterRequest = "{\"MailServerUrl\":\"https://EMAIL_SERVER_NAME/Microsoft-Server-ActiveSync?Cmd=Ping&User=test2@officeburrito.com&DeviceId=NchoD819080CX2F92X407CXACAC5&DeviceType=iPhone\",\"Protocol\":\"ActiveSync\",\"Platform\":\"ios\",\"ResponseTimeout\":600000,\"WaitBeforeUse\":60000,\"PushToken\":\"+5vEJ8vmOmbrp+HIRwvHYdLSWoqol1SWBceKDktgkKM=\",\"PushService\":\"APNS\",\"MailServerCredentials\":{\"Username\":\"test2@officeburrito.com\",\"Password\":\"Password1\"},\"HttpHeaders\":{\"User-Agent\":\"Apple-iPhone4C1/1208.321\",\"MS-ASProtocolVersion\":\"14.1\",\"Content-Length\":\"52\",\"Content-Type\":\"application/vnd.ms-sync.wbxml\"},\"RequestData\":\"AwFqAAANRUgDNjAwAAFJSksDNgABTANFbWFpbAABAUpLAzIAAUwDQ2FsZW5kYXIAAQEBAQ==\",\"ExpectedReply\":null,\"NoChangeReply\":\"AwFqAAANRUcDMQABAQ==\",\"IMAPAuthenticationBlob\":null,\"IMAPFolderName\":null,\"IMAPSupportsIdle\":false,\"IMAPSupportsExpunge\":false,\"IMAPEXISTSCount\":0,\"IMAPUIDNEXT\":0,\"UserId\":\"us-east-1:7ffdd228-2d97-4139-aa5b-2545bab32c49\",\"ClientId\":\"NchoD819080CX2F92X407CXACAC5\",\"DeviceId\":\"NchoD819080CX2F92X407CXACAC5\",\"ClientContext\":\"9b37e8a8\",\"OSVersion\":\"8.4.1\",\"AppBuildVersion\":\"DEV[azimozakil]\",\"AppBuildNumber\":\"1391\"}"
)

type MockEASClient struct {
	MockClient
}

func (m *MockEASClient) Register() error {
	return m.RegisterWithRequest(EASRegisterRequest)
}
