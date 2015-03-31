package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS/testHandler"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"testing"
)

type mailClientTester struct {
	suite.Suite
	logger            *Logging.Logger
	dbmap             *gorp.DbMap
	testClientId      string
	testClientContext string
	testDeviceId      string
	testPlatform      string
	testPushToken     string
	testPushService   string
	testProtocol      string
	aws               *testHandler.TestAwsHandler
}

func (s *mailClientTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbmap, err = initDB(&dbconfig, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPlatform = "ios"
	s.testPushToken = "AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF"
	s.testPushService = "APNS"
	s.testProtocol = "ActiveSync"
	s.aws = testHandler.NewTestAwsHandler()
	setGlobal(&BackendConfiguration{})
}

func (s *mailClientTester) SetupTest() {
}

func (s *mailClientTester) TearDownTest() {
}

func TestMailClient(t *testing.T) {
	s := new(mailClientTester)
	suite.Run(t, s)
}

type testingMailClientContext struct {
	logger *Logging.Logger
}

func (client *testingMailClientContext) stop() error {
	client.logger.Debug("stop")
	return nil
}
func (client *testingMailClientContext) validateStopToken(token string) bool {
	client.logger.Debug("validateStopToken")
	return true
}
func (client *testingMailClientContext) deferPoll(timeout int64) error {
	client.logger.Debug("deferPoll")
	return nil
}
func (client *testingMailClientContext) updateLastContact() error {
	client.logger.Debug("updateLastContact")
	return nil
}
func (client *testingMailClientContext) Status() (MailClientStatus, error) {
	client.logger.Debug("Status")
	return MailClientStatusPinging, nil
}
func (client *testingMailClientContext) Action(action PingerCommand) error {
	client.logger.Debug("Action")
	return nil
}
func (client *testingMailClientContext) getStopToken() string {
	client.logger.Debug("getStopToken")
	return "1234"
}
func (client *testingMailClientContext) getSessionInfo() (*ClientSessionInfo, error) {
	client.logger.Debug("getSessionInfo")
	return nil, nil
}

func (s *mailClientTester) TestMailClient() {
	pi := &MailPingInformation{}
	debug := true
	doStats := false
	client, err := NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)
	s.Equal("ClientID can not be empty", err.Error())

	// validity of the device information is tested in the deviceInfo_test.
	// only bother with things that mailClient is responsible for
	pi = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
	}
	client, err = NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)
	s.Equal(fmt.Sprintf("%s:%s:%s: Unsupported Mail Protocol %s", s.testDeviceId, s.testClientId, s.testClientContext, ""), err.Error())

	pi = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		Protocol:      "Foo",
	}
	client, err = NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)

	s.Equal(fmt.Sprintf("%s:%s:%s: Unsupported Mail Protocol %s", s.testDeviceId, s.testClientId, s.testClientContext, "Foo"), err.Error())
	pi = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		Protocol:      s.testProtocol,
	}
	client, err = NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.NotNil(client)
	s.NoError(err)
	s.NotEmpty(client.stopToken)
	s.NotNil(client.mailClient)
}
