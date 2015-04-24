package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
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
	aws               *AWS.TestAwsHandler
	sessionId         string
	mailInfo          *MailPingInformation
}

func (s *RPCServerTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbmap, err = dbconfig.initDB(true, s.logger)
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
		pollMap:     nil,
	}}
	s.backend = testingBackend
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPlatform = "ios"
	s.testMailServerUrl = "http://foo"
	s.sessionId = "12345678"
	s.aws = AWS.NewTestAwsHandler()
	s.mailInfo = &MailPingInformation{
		ClientId:      s.testClientId,
		ClientContext: s.testClientContext,
		DeviceId:      s.testDeviceId,
		Platform:      s.testPlatform,
		MailServerUrl: s.testMailServerUrl,
	}

}

func (s *RPCServerTester) SetupTest() {
	s.backend.pollMap = make(pollMapType)
}

func (s *RPCServerTester) TearDownTest() {
}

func TestRPCServer(t *testing.T) {
	s := new(RPCServerTester)
	suite.Run(t, s)
}

type TestingBackend struct {
	BackendPolling
}

func (t *TestingBackend) newMailClientContext(pi *MailPingInformation, doStats bool) (MailClientContextType, error) {
	return &testingMailClientContext{
		logger: t.logger,
		status: MailClientStatusPinging,
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

func (t *TestingBackend) newDbHandler(i interface{}, db DBHandlerType) (interface{}, error) {
	return nil, fmt.Errorf("Not implemented")
}

//func (t *TestingBackend) LockMap() {
//	return
//}
//
//func (t *TestingBackend) UnlockMap() {
//	return
//}
func (s *RPCServerTester) TestPollMap() {
	args := StartPollArgs{
		MailInfo: s.mailInfo,
	}

	s.Equal(fmt.Sprintf("%s--%s--%s", s.mailInfo.ClientId, s.mailInfo.ClientContext, s.mailInfo.DeviceId), args.pollMapKey())
}

func (s *RPCServerTester) TestStartPoll() {
	mailInfo := &MailPingInformation{}
	args := StartPollArgs{
		MailInfo: mailInfo,
	}
	reply := StartPollingResponse{}

	err := s.backend.Start(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("", reply.Message)

	ctx, err := s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusPinging, nil)
	err = s.backend.Start(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("", reply.Message)
	s.Empty(s.backend.pollMap[args.pollMapKey()], "Start should have deleted the entry, replaced with a new one")

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusDeferred, nil)
	err = s.backend.Start(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("", reply.Message)

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusStopped, nil)
	err = s.backend.Start(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("", reply.Message)

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusInitialized, nil)
	err = s.backend.Start(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("", reply.Message)

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusError, fmt.Errorf("Foo"))
	err = s.backend.Start(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyWarn, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyWarn, reply.Code))
	s.Equal("Previous Ping failed with error: Foo", reply.Message)
}

func (s *RPCServerTester) TestDeferPoll() {
	reply := PollingResponse{}
	args := DeferPollArgs{
		ClientId:      s.mailInfo.ClientId,
		ClientContext: s.mailInfo.ClientContext,
		DeviceId:      s.mailInfo.DeviceId,
		Timeout:       30000,
	}

	err := s.backend.Defer(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyError, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyError, reply.Code))
	s.Equal("No active sessions found", reply.Message)

	ctx, err := s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx

	err = s.backend.Defer(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("", reply.Message)

	ctx.setStatus(MailClientStatusStopped, nil)
	err = s.backend.Defer(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyError, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyError, reply.Code))
	s.Equal("Client is not pinging or deferred (Stopped). Can not defer.", reply.Message)
}
func (s *RPCServerTester) TestStopPoll() {
	reply := PollingResponse{}
	args := StopPollArgs{
		ClientId:      s.mailInfo.ClientId,
		ClientContext: s.mailInfo.ClientContext,
		DeviceId:      s.mailInfo.DeviceId,
	}

	err := s.backend.Stop(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyError, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyError, reply.Code))
	s.Equal("No active sessions found", reply.Message)

	ctx, err := s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusPinging, nil)
	err = s.backend.Stop(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("Stopped", reply.Message)
	s.Empty(s.backend.pollMap[args.pollMapKey()], "Stop should have deleted the entry")

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusDeferred, nil)
	err = s.backend.Stop(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("Stopped", reply.Message)

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusStopped, nil)
	err = s.backend.Stop(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("Stopped", reply.Message)

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusInitialized, nil)
	err = s.backend.Stop(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("Stopped", reply.Message)

	ctx, err = s.backend.newMailClientContext(s.mailInfo, false)
	s.backend.pollMap[args.pollMapKey()] = ctx
	ctx.setStatus(MailClientStatusError, fmt.Errorf("Foo"))
	err = s.backend.Stop(&args, &reply)
	s.NoError(err)
	s.Equal(PollingReplyOK, reply.Code, fmt.Sprintf("Should have gotten %s. Got %s", PollingReplyOK, reply.Code))
	s.Equal("Stopped", reply.Message)
}
