package main

import (
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type devicesTester struct {
	suite.Suite
	logger          *Logging.Logger
	mx              *mux.Router
	n               *negroni.Negroni
	registerPath    string
	fakeUrl         string
	fakeRegisterUrl string
	pingerConfig    *Pinger.Configuration
	config          *Pinger.Configuration
	registerJson    string
	rpcTestPort     int
}

func (s *devicesTester) SetupSuite() {
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	s.registerPath = "/register"
	s.fakeUrl = "http://mypinger.com"
	s.fakeRegisterUrl = s.fakeUrl + s.registerPath
	s.registerJson = "{\"ClientContext\": \"12345\", \"DeviceId\": \"NchoDC28E565X072CX46B1XBF205\", \"WaitBeforeUse\": 3000, \"MailServerCredentials\": {\"Username\": \"janv\", \"Password\": \"Password1\"}, \"ClientId\": \"us-east-1:0005d365-c8ea-470f-8a61-a7f44f145efb\", \"Platform\": \"ios\", \"RequestData\": \"AwFqAAANRUgDNjAwAAFJSksDNgABTANFbWFpbAABAUpLAzIAAUwDQ2FsZW5kYXIAAQEBAQ==\", \"PushService\": \"APNS\", \"ResponseTimeout\": 600000, \"ExpectedReply\": \"\", \"PushToken\": \"AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF\", \"MailServerUrl\": \"https://mail.d2.officeburrito.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=janv@d2.officeburrito.com&DeviceId=NchoDC28E565X072CX46B1XBF205&DeviceType=iPad\", \"HttpHeaders\": {\"MS-ASProtocolVersion\": \"14.1\", \"User-Agent\":\"Apple-iPad3C1/1202.466\", \"Content-Length\":\"52\",\"Content-Type\":\"application/vnd.ms-sync.wbxml\"}, \"NoChangeReply\": \"AwFqAAANRUcDMQABAQ==\", \"Protocol\": \"ActiveSync\", \"OSVersion\": \"8.1\", \"AppBuildVersion\": \"0.9\", \"AppBuildNumber\": \"[dev]janv@nachocove.com\"}"
	s.rpcTestPort = 40800

	rpcConfig := Pinger.RPCServerConfiguration{
		Protocol: "http",
		Hostname: "localhost",
		Port:     s.rpcTestPort,
	}
	s.pingerConfig = Pinger.NewConfiguration()
	s.pingerConfig.Db.Type = "sqlite"
	s.pingerConfig.Db.Filename = ":memory:"
	s.pingerConfig.Rpc = rpcConfig

	s.mx = mux.NewRouter()
	s.mx.HandleFunc(s.registerPath, registerDevice)
	s.config = Pinger.NewConfiguration()
	s.config.Rpc = rpcConfig
	s.n = negroni.New(NewContextMiddleWare(&Context{Logger: s.logger, Config: s.config}))
	s.n.UseHandler(s.mx)
	go s.startRpc()
}

func (s *devicesTester) SetupTest() {
}

func (s *devicesTester) TearDownTest() {
}

func (s *devicesTester) startRpc() {
	err := Pinger.StartPollingRPCServer(s.pingerConfig, true, s.logger)
	if err != nil {
		panic(err)
	}
}

func TestWebDevices(t *testing.T) {
	s := new(devicesTester)
	suite.Run(t, s)
}

func (s *devicesTester) TestRegisterGet() {
	req, err := http.NewRequest("GET", s.fakeRegisterUrl, nil)
	s.NoError(err)
	s.NotNil(req)

	response := httptest.NewRecorder()
	s.NotNil(response)
	s.n.ServeHTTP(response, req)
	s.Equal(400, response.Code)
	s.Contains(response.Body.String(), "UNKNOWN METHOD")
}

func (s *devicesTester) TestRegisterEncodingFail() {
	req, err := http.NewRequest("POST", s.fakeRegisterUrl, strings.NewReader(""))
	s.NoError(err)
	s.NotNil(req)

	response := httptest.NewRecorder()
	s.NotNil(response)
	s.n.ServeHTTP(response, req)
	s.Equal(400, response.Code)
	s.Contains(response.Body.String(), "UNKNOWN Encoding")
}

func (s *devicesTester) TestRegisterContentFail() {
	req, err := http.NewRequest("POST", s.fakeRegisterUrl, strings.NewReader("{}"))
	s.NoError(err)
	s.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	s.NotNil(response)
	s.n.ServeHTTP(response, req)
	s.Equal(400, response.Code)
	s.Contains(response.Body.String(), "MISSING_REQUIRED_DATA")
}

func (s *devicesTester) TestRegisterJsonFail() {
	req, err := http.NewRequest("POST", s.fakeRegisterUrl, strings.NewReader("{"))
	s.NoError(err)
	s.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	s.NotNil(response)
	s.n.ServeHTTP(response, req)
	s.Equal(400, response.Code)
	s.Contains(response.Body.String(), "Could not parse json")
}

func (s *devicesTester) TestRegisterRPCFail() {
	s.config.Rpc.Port = 10
	s.config.Server.TokenAuthKey = "01234567890123456789012345678901"
	req, err := http.NewRequest("POST", s.fakeRegisterUrl, strings.NewReader(s.registerJson))
	s.NoError(err)
	s.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	s.NotNil(response)
	s.n.ServeHTTP(response, req)
	s.Equal(500, response.Code)
	s.Contains(response.Body.String(), "RPC_SERVER_ERROR")
}

func (s *devicesTester) TestPostInfoValidate() {
	pd := &registerPostData{}
	err := pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Missing required fields")
	s.Contains(err.Error(), "Protocol")
	s.Contains(err.Error(), "ClientId")
	s.Contains(err.Error(), "ClientContext")
	s.Contains(err.Error(), "DeviceId")
	s.Contains(err.Error(), "MailServerUrl")
	s.Contains(err.Error(), "Protocol")
	s.Contains(err.Error(), "WaitBeforeUse")

	pd.ClientId = "foo"
	err = pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Missing required fields")
	s.Contains(err.Error(), "Protocol")
	s.NotContains(err.Error(), "ClientId")
	s.Contains(err.Error(), "ClientContext")
	s.Contains(err.Error(), "DeviceId")
	s.Contains(err.Error(), "MailServerUrl")
	s.Contains(err.Error(), "Protocol")
	s.Contains(err.Error(), "WaitBeforeUse")

	pd.ClientContext = "bar"
	err = pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Missing required fields")
	s.Contains(err.Error(), "Protocol")
	s.NotContains(err.Error(), "ClientId")
	s.NotContains(err.Error(), "ClientContext")
	s.Contains(err.Error(), "DeviceId")
	s.Contains(err.Error(), "MailServerUrl")
	s.Contains(err.Error(), "Protocol")
	s.Contains(err.Error(), "WaitBeforeUse")

	pd.WaitBeforeUse = 10
	err = pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Missing required fields")
	s.Contains(err.Error(), "Protocol")
	s.NotContains(err.Error(), "ClientId")
	s.NotContains(err.Error(), "ClientContext")
	s.Contains(err.Error(), "DeviceId")
	s.Contains(err.Error(), "MailServerUrl")
	s.Contains(err.Error(), "Protocol")
	s.NotContains(err.Error(), "WaitBeforeUse")

	pd.Protocol = "foo"
	err = pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Missing required fields")
	s.NotContains(err.Error(), "ClientId")
	s.NotContains(err.Error(), "ClientContext")
	s.Contains(err.Error(), "DeviceId")
	s.Contains(err.Error(), "MailServerUrl")
	s.Contains(err.Error(), "Protocol")
	s.NotContains(err.Error(), "WaitBeforeUse")
	s.Contains(err.Error(), "Protocol foo is not known")

	pd.Protocol = "ActiveSync"
	err = pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Missing required fields")
	s.NotContains(err.Error(), "Protocol")
	s.NotContains(err.Error(), "ClientId")
	s.NotContains(err.Error(), "ClientContext")
	s.Contains(err.Error(), "DeviceId")
	s.Contains(err.Error(), "MailServerUrl")
	s.NotContains(err.Error(), "WaitBeforeUse")
	s.NotContains(err.Error(), "is not known")
	s.Contains(err.Error(), "RequestData")
	s.Contains(err.Error(), "NoChangeReply")

	pd.RequestData = []byte("323232323")
	err = pd.Validate()
	s.NotContains(err.Error(), "RequestData")
	s.Contains(err.Error(), "NoChangeReply")

	pd.NoChangeReply = []byte("323232323")
	err = pd.Validate()
	s.NotContains(err.Error(), "RequestData")
	s.NotContains(err.Error(), "NoChangeReply")

	pd.Platform = "foo"
	err = pd.Validate()
	s.Error(err)
	s.Contains(err.Error(), "Platform foo is not known")

	pd.Platform = "ios"
	err = pd.Validate()
	s.Error(err)
	s.NotContains(err.Error(), "Platform")
}

//func (s *devicesTester) TestRegisterContentSuccess() {
//  config.Rpc.Port = rpcTestPort
//	req, err := http.NewRequest("POST", fakeRegisterUrl, strings.NewReader(registerJson))
//	s.NoError(err)
//	s.NotNil(req)
//	req.Header.Add("Content-Type", "application/json")
//
//	response := httptest.NewRecorder()
//	s.NotNil(response)
//	s.n.ServeHTTP(response, req)
//	s.Equal(200, response.Code)
//	s.Contains(response.Body.String(), "\"Status\":\"OK\"")
//}
