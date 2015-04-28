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
	db                DBHandler
	testClientId      string
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
	dbconfig := DBConfiguration{Type: DBTypeSqlite, Filename: ":memory:"}
	s.dbmap, err = dbconfig.initDB(true, s.logger)
	if err != nil {
		panic(err)
	}
	s.aws = AWS.NewTestAwsHandler()

	s.db = newDbHandler(DBHandlerSql, s.dbmap, s.aws)
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPlatform = "ios"
	s.testPushToken = "AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF"
	s.testPushService = "APNS"
	s.testProtocol = "ActiveSync"
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
func (client *testingMailClientContext) deferPoll(timeout int64) {
	return
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
	client, err := NewMailClientContext(s.db, s.aws, pi, debug, doStats, s.logger)
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
		SessionId:     s.sessionId,
	}
	client, err = NewMailClientContext(s.db, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)
	s.Equal(fmt.Sprintf("%s:%s:%s:%s: Unsupported Mail Protocol %s", s.testDeviceId, s.testClientId, s.testClientContext, s.sessionId, ""), err.Error())

	pi = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		Protocol:      "Foo",
		SessionId:     s.sessionId,
	}
	client, err = NewMailClientContext(s.db, s.aws, pi, debug, doStats, s.logger)
	s.Nil(client)
	s.Error(err)

	s.Equal(fmt.Sprintf("%s:%s:%s:%s: Unsupported Mail Protocol %s", s.testDeviceId, s.testClientId, s.testClientContext, s.sessionId, "Foo"), err.Error())
	pi = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		PushService:   s.testPushService,
		PushToken:     s.testPushToken,
		Protocol:      s.testProtocol,
		SessionId:     s.sessionId,
	}
	client, err = NewMailClientContext(s.db, s.aws, pi, debug, doStats, s.logger)
	s.NotNil(client)
	s.NoError(err)
	s.NotNil(client.mailClient)
}
