package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"testing"
)

type mailClientTester struct {
	suite.Suite
	logger            *Logging.Logger
	dbmap             *gorp.DbMap
	testUserId        string
	testClientContext string
	testDeviceId      string
	testPlatform      string
	testPushToken     string
	testPushService   string
	testProtocol      string
	aws               *AWS.TestAwsHandler
	sessionId         string
}

func (s *mailClientTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbmap, err = initDB(&dbconfig, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
	s.testUserId = "sometestUserId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPlatform = "ios"
	s.testPushToken = "AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF"
	s.testPushService = "APNS"
	s.testProtocol = "ActiveSync"
	s.aws = AWS.NewTestAwsHandler()
	s.sessionId = "12345678"
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
	logger    *Logging.Logger
	status    MailClientStatus
	lastError error
}

func (client *testingMailClientContext) stop() {
	return
}
func (client *testingMailClientContext) deferPoll(timeout uint64, requestData []byte) {
	return
}
func (client *testingMailClientContext) updateLastContact() error {
	return nil
}
func (client *testingMailClientContext) Status() (MailClientStatus, error) {
	return client.status, client.lastError
}
func (client *testingMailClientContext) setStatus(status MailClientStatus, err error) {
	client.status = status
	client.lastError = err
}
func (client *testingMailClientContext) Action(action PingerCommand) error {
	return nil
}
func (client *testingMailClientContext) getSessionInfo() (*ClientSessionInfo, error) {
	return nil, nil
}

func (s *mailClientTester) TestMailClient() {
	pi := &MailPingInformation{}
	pi.SessionId = s.sessionId
	debug := true
	doStats := false
	client, err := NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)
	s.Equal("UserId can not be empty", err.Error())

	// validity of the device information is tested in the deviceInfo_test.
	// only bother with things that mailClient is responsible for
	pi = &MailPingInformation{
		UserId:        s.testUserId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		SessionId:     s.sessionId,
	}
	client, err = NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)

	s.Equal(fmt.Sprintf("|device=%s|client=%s|context=%s|session=%s|Unsupported mail protocol|protocol=%s", s.testDeviceId, s.testUserId, s.testClientContext, s.sessionId, ""), err.Error())

	pi = &MailPingInformation{
		UserId:        s.testUserId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		Protocol:      "Foo",
		SessionId:     s.sessionId,
	}
	client, err = NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)

	s.Equal(fmt.Sprintf("|device=%s|client=%s|context=%s|session=%s|Unsupported mail protocol|protocol=%s", s.testDeviceId, s.testUserId, s.testClientContext, s.sessionId, "Foo"), err.Error())
	pi = &MailPingInformation{
		UserId:        s.testUserId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		Protocol:      s.testProtocol,
		SessionId:     s.sessionId,
	}
	client, err = NewMailClientContext(s.dbmap, s.aws, pi, debug, doStats, s.logger)
	s.NotNil(client)
	s.NoError(err)
	s.NotNil(client.mailClient)
}
