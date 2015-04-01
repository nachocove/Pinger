package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS/testHandler"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"testing"
)

type RPCServerTester struct {
	suite.Suite
	backend           *TestingBackend
	logger            *Logging.Logger
	dbmap             *gorp.DbMap
	testClientId      string
	testClientContext string
	testDeviceId      string
	testPlatform      string
	testMailServerUrl string
	aws               *testHandler.TestAwsHandler
	sessionId         string
}

func (s *RPCServerTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbmap, err = initDB(&dbconfig, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}

	config := NewConfiguration()
	config.Db.Type = "sqlite"
	config.Db.Filename = ":memory:"

	testingBackend := &TestingBackend{BackendPolling{
		dbm:         s.dbmap,
		logger:      s.logger,
		loggerLevel: -1,
		debug:       true,
		pollMap:     make(pollMapType),
	}}
	s.backend = testingBackend
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPlatform = "ios"
	s.testMailServerUrl = "http://foo"
	s.sessionId = "12345678"
	s.aws = testHandler.NewTestAwsHandler()
}

func (s *RPCServerTester) SetupTest() {
}

func (s *RPCServerTester) TearDownTest() {
}

func TestRPCServer(t *testing.T) {
	s := new(deviceInfoTester)
	suite.Run(t, s)
}

type TestingBackend struct {
	BackendPolling
}

func (t *TestingBackend) newMailClientContext(pi *MailPingInformation, doStats bool) (MailClientContextType, error) {
	return &testingMailClientContext{
		logger: t.logger,
	}, nil
}

func (t *TestingBackend) Start(args *StartPollArgs, reply *StartPollingResponse) (err error) {
	return RPCStartPoll(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (t *TestingBackend) Stop(args *StopPollArgs, reply *PollingResponse) (err error) {
	return RPCStopPoll(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (t *TestingBackend) Defer(args *DeferPollArgs, reply *PollingResponse) (err error) {
	return RPCDeferPoll(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (t *TestingBackend) FindActiveSessions(args *FindSessionsArgs, reply *FindSessionsResponse) (err error) {
	return RPCFindActiveSessions(t, &t.pollMap, t.dbm, args, reply, t.logger)
}

func (s *RPCServerTester) TestRPCStartPoll() {
	mailInfo := &MailPingInformation{}
	args := StartPollArgs{
		MailInfo: mailInfo,
	}
	reply := StartPollingResponse{}

	diInDb, err := getDeviceInfo(s.dbmap, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.Nil(diInDb)

	err = s.backend.Start(&args, &reply)
	fmt.Println(reply)
	s.NoError(err)
	s.Equal(PollingReplyError, reply.Code)
	s.NotEmpty(reply.Message)

	args.MailInfo = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		MailServerUrl: s.testMailServerUrl,
	}
	err = s.backend.Start(&args, &reply)
	fmt.Println(reply)
	s.NoError(err)
	s.Equal(PollingReplyError, reply.Code)
	s.NotEmpty(reply.Message)
}
