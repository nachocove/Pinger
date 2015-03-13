package main

import (
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/nachocove/Pinger/Pinger"
	"os"
)

var logger *logging.Logger
var mx *mux.Router
var n *negroni.Negroni
var registerPath string = "/register"
var fakeUrl string = "http://mypinger.com"
var fakeRegisterUrl string = fakeUrl + registerPath
var pingerConfig *Pinger.Configuration
var config *Configuration

func TestMain(m *testing.M) {
	testDbFilename := "/tmp/unittest.db"
	formatStr := "%{time:2006-01-02T15:04:05.000} %{level} %{shortfile}:%{shortfunc} %{message}"
	format := logging.MustStringFormatter(formatStr)
	logging.SetFormatter(format)
	logger = logging.MustGetLogger("unittest")

	os.Remove(testDbFilename)
	
	rpcConfig := Pinger.RPCServerConfiguration{
		Protocol: "http",
		Hostname: "localhost",
		Port: 40800,
	}
	pingerConfig = &Pinger.Configuration{
		Rpc: rpcConfig,
		Db: Pinger.DBConfiguration{Type: "sqlite", Filename: testDbFilename},
	}
	mx = mux.NewRouter()
	mx.HandleFunc(registerPath, registerDevice)
	config = &Configuration{Rpc: rpcConfig}
	n = negroni.New(NewContextMiddleWare(&Context{Logger: logger, Config: config}))
	n.UseHandler(mx)

	go startRpc(pingerConfig)
	
	defer os.Remove(testDbFilename)

	os.Exit(m.Run())
}

func startRpc(config *Pinger.Configuration) {
	err := Pinger.StartPollingRPCServer(pingerConfig, true, logger)
	if err != nil {
		panic(err)
	}	
}
func TestRegisterGet(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("GET", fakeRegisterUrl, nil)
	assert.NoError(err)
	assert.NotNil(req)

	response := httptest.NewRecorder()
	assert.NotNil(response)
	n.ServeHTTP(response, req)
	assert.Equal(400, response.Code)
	assert.Contains(response.Body.String(), "UNKNOWN METHOD")
}

func TestRegisterEncodingFail(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("POST", fakeRegisterUrl, strings.NewReader(""))
	assert.NoError(err)
	assert.NotNil(req)

	response := httptest.NewRecorder()
	assert.NotNil(response)
	n.ServeHTTP(response, req)
	assert.Equal(400, response.Code)
	assert.Contains(response.Body.String(), "UNKNOWN Encoding")
}

func TestRegisterContentFail(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("POST", fakeRegisterUrl, strings.NewReader("{}"))
	assert.NoError(err)
	assert.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	assert.NotNil(response)
	n.ServeHTTP(response, req)
	assert.Equal(400, response.Code)
	assert.Contains(response.Body.String(), "MISSING_REQUIRED_DATA")
}

func TestRegisterJsonFail(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("POST", fakeRegisterUrl, strings.NewReader("{"))
	assert.NoError(err)
	assert.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	assert.NotNil(response)
	n.ServeHTTP(response, req)
	assert.Equal(400, response.Code)
	assert.Contains(response.Body.String(), "Could not parse json")
}

func TestRegisterRPCFail(t *testing.T) {
	assert := assert.New(t)
	registerJson := "{\"ClientContext\": \"12345\", \"DeviceId\": \"NchoDC28E565X072CX46B1XBF205\", \"WaitBeforeUse\": 3000, \"MailServerCredentials\": {\"Username\": \"janv\", \"Password\": \"Password1\"}, \"ClientId\": \"us-east-1:0005d365-c8ea-470f-8a61-a7f44f145efb\", \"Platform\": \"ios\", \"RequestData\": \"AwFqAAANRUgDNjAwAAFJSksDNgABTANFbWFpbAABAUpLAzIAAUwDQ2FsZW5kYXIAAQEBAQ==\", \"PushService\": \"APNS\", \"ResponseTimeout\": 600000, \"ExpectedReply\": \"\", \"PushToken\": \"AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF\", \"MailServerUrl\": \"https://mail.d2.officeburrito.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=janv@d2.officeburrito.com&DeviceId=NchoDC28E565X072CX46B1XBF205&DeviceType=iPad\", \"HttpHeaders\": {\"MS-ASProtocolVersion\": \"14.1\", \"User-Agent\":\"Apple-iPad3C1/1202.466\", \"Content-Length\":\"52\",\"Content-Type\":\"application/vnd.ms-sync.wbxml\"}, \"NoChangeReply\": \"AwFqAAANRUcDMQABAQ==\", \"Protocol\": \"ActiveSync\", \"OSVersion\": \"8.1\", \"AppBuildVersion\": \"0.9\", \"AppBuildNumber\": \"[dev]janv@nachocove.com\"}"
	req, err := http.NewRequest("POST", fakeRegisterUrl, strings.NewReader(registerJson))
	assert.NoError(err)
	assert.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	assert.NotNil(response)
	n.ServeHTTP(response, req)
	assert.Equal(500, response.Code)
	assert.Contains(response.Body.String(), "RPC_SERVER_ERROR")
}

func TestRegisterContentSuccess(t *testing.T) {
	assert := assert.New(t)
	registerJson := "{\"ClientContext\": \"12345\", \"DeviceId\": \"NchoDC28E565X072CX46B1XBF205\", \"WaitBeforeUse\": 3000, \"MailServerCredentials\": {\"Username\": \"janv\", \"Password\": \"Password1\"}, \"ClientId\": \"us-east-1:0005d365-c8ea-470f-8a61-a7f44f145efb\", \"Platform\": \"ios\", \"RequestData\": \"AwFqAAANRUgDNjAwAAFJSksDNgABTANFbWFpbAABAUpLAzIAAUwDQ2FsZW5kYXIAAQEBAQ==\", \"PushService\": \"APNS\", \"ResponseTimeout\": 600000, \"ExpectedReply\": \"\", \"PushToken\": \"AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF\", \"MailServerUrl\": \"https://mail.d2.officeburrito.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=janv@d2.officeburrito.com&DeviceId=NchoDC28E565X072CX46B1XBF205&DeviceType=iPad\", \"HttpHeaders\": {\"MS-ASProtocolVersion\": \"14.1\", \"User-Agent\":\"Apple-iPad3C1/1202.466\", \"Content-Length\":\"52\",\"Content-Type\":\"application/vnd.ms-sync.wbxml\"}, \"NoChangeReply\": \"AwFqAAANRUcDMQABAQ==\", \"Protocol\": \"ActiveSync\", \"OSVersion\": \"8.1\", \"AppBuildVersion\": \"0.9\", \"AppBuildNumber\": \"[dev]janv@nachocove.com\"}"
	req, err := http.NewRequest("POST", fakeRegisterUrl, strings.NewReader(registerJson))
	assert.NoError(err)
	assert.NotNil(req)
	req.Header.Add("Content-Type", "application/json")

	response := httptest.NewRecorder()
	assert.NotNil(response)
	n.ServeHTTP(response, req)
	assert.Equal(200, response.Code)
	assert.Contains(response.Body.String(), "\"Status\":\"OK\"")
}
