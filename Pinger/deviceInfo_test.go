package Pinger

import (
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"github.com/stretchr/testify/require"
	"testing"
)

type deviceInfoTester struct {
	suite.Suite
	dbmap             *gorp.DbMap
	logger            *Logging.Logger
	testClientId      string
	testClientContext string
	testDeviceId      string
	testPushToken     string
	testPushService   string
	testMailProtocol  string
	testPlatform      string
	testOSVersion     string
	testAppVersion    string
	testAppnumber     string
}

func (s *deviceInfoTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbmap, err = initDB(&dbconfig, true, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPushToken = "AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF"
	s.testPushService = "APNS"
	s.testPlatform = "ios"
	s.testOSVersion = "8.1"
	s.testAppVersion = "0.9"
	s.testAppnumber = "(dev) Foo"
}

func (s *deviceInfoTester) SetupTest() {
	s.dbmap.TruncateTables()
}

func (s *deviceInfoTester) TearDownTest() {
}

func TestDeviceInfo(t *testing.T) {
	s := new(deviceInfoTester)
	suite.Run(t, s)
}

func (s *deviceInfoTester) TestDeviceInfoValidate() {
	var err error

	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppnumber,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.validate()
	s.NoError(err)

	di.ClientId = ""
	err = di.validate()
	s.EqualError(err, "ClientID can not be empty")
	di.ClientId = s.testClientId

	di.ClientContext = ""
	err = di.validate()
	s.EqualError(err, "ClientContext can not be empty")
	di.ClientContext = s.testClientContext

	di.DeviceId = ""
	err = di.validate()
	s.EqualError(err, "DeviceId can not be empty")
	di.DeviceId = s.testDeviceId

	di.Platform = ""
	err = di.validate()
	s.EqualError(err, "Platform can not be empty")

	di.Platform = "foo"
	err = di.validate()
	s.EqualError(err, "Platform foo is not known")
	di.Platform = s.testPlatform

	di.PushService = ""
	err = di.validate()
	s.EqualError(err, "PushService can not be empty")

	di.PushService = "foo"
	err = di.validate()
	s.EqualError(err, "PushService foo is not known")
	di.PushService = s.testPushService
}

func (s *deviceInfoTester) TestDeviceInfoCleanup() {
	var err error
	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppnumber,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	di.insert(s.dbmap)

	di.cleanup()
	s.Equal("", di.ClientId)
	s.Equal("", di.ClientContext)
	s.Equal("", di.DeviceId)
	s.Equal("", di.PushToken)
	s.Equal("", di.PushService)
	s.Equal("", di.Platform)
	s.Equal("", di.OSVersion)
	s.Equal("", di.AppBuildNumber)
	s.Equal("", di.AppBuildVersion)
	s.Equal(0, di.Id)
}

func (s *deviceInfoTester) TestDeviceInfoCreate() {
	var err error
	deviceList, err := getAllMyDeviceInfo(s.dbmap, s.logger)
	s.Equal(len(deviceList), 0)

	di, err := newDeviceInfo(
		s.testClientId,
		"",
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppnumber,
		s.logger)
	s.Error(err, "ClientContext can not be empty")
	s.Nil(di)

	di, err = newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppnumber,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	s.Equal(s.testClientId, di.ClientId)
	s.Equal(s.testClientContext, di.ClientContext)
	s.Equal(s.testDeviceId, di.DeviceId)
	s.Equal(s.testPushToken, di.PushToken)
	s.Equal(s.testPushService, di.PushService)
	s.Equal(s.testPlatform, di.Platform)
	s.Equal(s.testOSVersion, di.OSVersion)
	s.Equal(s.testAppVersion, di.AppBuildVersion)
	s.Equal(s.testAppnumber, di.AppBuildNumber)

	s.Equal(0, di.Id)
	s.Empty(di.Pinger)
	s.Equal(0, di.Created)
	s.Equal(0, di.Updated)
	s.Empty(di.AWSEndpointArn)

	di.dbm = s.dbmap
	lastContact, lastContactRequest, err := di.getContactInfo(false)
	s.Error(err)
	s.Equal(0, lastContact)
	s.Equal(0, lastContactRequest)

	deviceList, err = getAllMyDeviceInfo(s.dbmap, s.logger)
	s.Equal(0, len(deviceList))

	diInDb, err := getDeviceInfo(s.dbmap, s.testClientId, s.testClientContext, s.testDeviceId, s.logger)
	s.NoError(err)
	s.Nil(diInDb)

	err = di.insert(s.dbmap)
	s.NoError(err)
	s.NotEmpty(di.Pinger)
	s.True(di.Created > 0)
	s.True(di.Updated > 0)
	s.Empty(di.AWSEndpointArn)
	lastContact, lastContactRequest, err = di.getContactInfo(false)
	s.NoError(err)
	s.True(lastContact > 0)
	s.Equal(0, lastContactRequest)

	s.Equal(pingerHostId, di.Pinger)

	diInDb, err = getDeviceInfo(s.dbmap, s.testClientId, s.testClientContext, s.testDeviceId, s.logger)
	s.NoError(err)
	s.NotNil(diInDb)
	s.Equal(di.Id, diInDb.Id)

	deviceList, err = getAllMyDeviceInfo(s.dbmap, s.logger)
	s.NoError(err)
	s.Equal(1, len(deviceList))
	s.NotNil(di.dbm)
	di.cleanup()
}

func (s *deviceInfoTester) TestDeviceInfoUpdate() {
	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppnumber,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.insert(s.dbmap)
	s.NoError(err)

	di, err = getDeviceInfo(s.dbmap, s.testClientId, s.testClientContext, s.testDeviceId, s.logger)
	s.NoError(err)
	s.NotNil(di)

	di2, err := getDeviceInfo(s.dbmap, s.testClientId, s.testClientContext, s.testDeviceId, s.logger)
	s.NoError(err)
	s.NotNil(di2)
	s.Equal(di.Id, di2.Id)

	di.AWSEndpointArn = "some endpoint"
	_, err = di.update()
	s.NoError(err)
	s.NotEmpty(di.AWSEndpointArn)

	di.AWSEndpointArn = "some other endpoint"
	_, err = di.update()
	s.NoError(err)
	s.NotEmpty(di.AWSEndpointArn)

	di2.AWSEndpointArn = "yet another endpoint"
	s.Panics(func() { di2.update() })

	changed, err := di.updateDeviceInfo(di.ClientContext, di.DeviceId, di.PushService, di.PushToken,
		di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.False(changed)
	s.NotEmpty(di.AWSEndpointArn)

	newToken := "some updated token"
	changed, err = di.updateDeviceInfo(di.ClientContext, di.DeviceId, di.PushService, newToken,
		di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newToken, di.PushToken)
	s.Empty(di.AWSEndpointArn)

	lastContact, _, err := di.getContactInfo(false)
	s.NoError(err)
	err = di.updateLastContact()
	s.NoError(err)

	lastContact2, _, err := di.getContactInfo(false)
	s.NoError(err)

	s.True(lastContact2 > lastContact)
	di.cleanup()
}

func (s *deviceInfoTester) TestDeviceInfoDelete() {
	var err error

	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppnumber,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.insert(s.dbmap)
	s.NoError(err)
	di, err = getDeviceInfo(s.dbmap, s.testClientId, s.testClientContext, s.testDeviceId, s.logger)
	s.NoError(err)
	s.NotNil(di)

	di.cleanup()

	di = nil

	di, err = getDeviceInfo(s.dbmap, s.testClientId, s.testClientContext, s.testDeviceId, s.logger)
	s.NoError(err)
	s.Nil(di)
}

func (s *deviceInfoTester) TestDevicePushMessageCreate() {
	di := DeviceInfo{Platform: "ios", PushService: AWS.PushServiceAPNS, ClientContext: "FOO"}
	var days_28 int64 = 2419200

	message, err := di.pushMessage(PingerNotificationRegister, days_28)
	s.NoError(err)
	s.NotEmpty(message)
	s.Equal(
		"{\"APNS\":\"{\\\"aps\\\":{\\\"content-available\\\":1,\\\"sound\\\":\\\"silent.wav\\\"},\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}}\",\"APNS_SANDBOX\":\"{\\\"aps\\\":{\\\"content-available\\\":1,\\\"sound\\\":\\\"silent.wav\\\"},\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}}\",\"GCM\":\"{\\\"collapse_key\\\":\\\"10e23d6b0b515fbff01dff49948afebea929a763\\\",\\\"data\\\":{\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}},\\\"delay_while_idle\\\":false,\\\"time_to_live\\\":2419200}\",\"default\":\"{\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}}\"}",
		message)
}
