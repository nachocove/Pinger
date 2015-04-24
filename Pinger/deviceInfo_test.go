package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

type deviceInfoTester struct {
	suite.Suite
	dbm               *gorp.DbMap
	db                DeviceInfoDbHandler
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
	testAppNumber     string
	aws               *AWS.TestAwsHandler
	sessionId         string
}

func (s *deviceInfoTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbm, err = dbconfig.initDB(true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
	s.db = newDeviceInfoSqlHandler(s.dbm)
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
	s.testPushToken = "AEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEF"
	s.testPushService = "APNS"
	s.testPlatform = "ios"
	s.testOSVersion = "8.1"
	s.testAppVersion = "0.9"
	s.testAppNumber = "(dev) Foo"
	s.sessionId = "12345678"
}

func (s *deviceInfoTester) SetupTest() {
	s.dbm.TruncateTables()
	s.aws = AWS.NewTestAwsHandler()
	globals = nil
	setGlobal(NewBackendConfiguration())
}

func (s *deviceInfoTester) TearDownTest() {
	globals = nil
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
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
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
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)
	di.insert(nil)

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
	deviceList, err := getAllMyDeviceInfo(s.db, s.aws, s.logger)
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
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
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
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
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
	s.Equal(s.testAppNumber, di.AppBuildNumber)

	s.Equal(0, di.Id)
	s.Empty(di.Pinger)
	s.Equal(0, di.Created)
	s.Equal(0, di.Updated)
	s.Empty(di.AWSEndpointArn)

	// get but don't create. Since it doesn't exist, there will be an error
	lastContact, lastContactRequest, err := di.getContactInfo(false)
	s.Error(err)
	s.Equal(0, lastContact)
	s.Equal(0, lastContactRequest)

	deviceList, err = getAllMyDeviceInfo(s.db, s.aws, s.logger)
	s.Equal(0, len(deviceList))

	diInDb, err := getDeviceInfo(s.db, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.Nil(diInDb)

	err = di.insert(nil)
	s.NoError(err)
	s.NotEmpty(di.Pinger)
	s.True(di.Created > 0)
	s.True(di.Updated > 0)
	s.Empty(di.AWSEndpointArn)

	// getDeviceInfo creates the data. Fetch it here and verify
	lastContact, lastContactRequest, err = di.getContactInfo(false)
	s.NoError(err)
	s.True(lastContact > 0)
	s.Equal(0, lastContactRequest)

	s.Equal(pingerHostId, di.Pinger)

	diInDb, err = getDeviceInfo(s.db, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.NotNil(diInDb)
	s.Equal(di.Id, diInDb.Id)

	deviceList, err = getAllMyDeviceInfo(s.db, s.aws, s.logger)
	s.NoError(err)
	s.Equal(1, len(deviceList))
	s.NotNil(di.db)
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
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.insert(nil)
	s.NoError(err)

	di, err = getDeviceInfo(s.db, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.NotNil(di)

	di2, err := getDeviceInfo(s.db, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.NotNil(di2)
	s.Equal(di.Id, di2.Id)

	di.AWSEndpointArn = "some endpoint"
	_, err = di.update()
	s.NoError(err)
	s.NotEqual("", di.AWSEndpointArn)

	di.AWSEndpointArn = "some other endpoint"
	_, err = di.update()
	s.NoError(err)
	s.NotEqual("", di.AWSEndpointArn)

	di2.AWSEndpointArn = "yet another endpoint"
	s.Panics(func() { di2.update() })

	changed, err := di.updateDeviceInfo(di.PushService, di.PushToken, di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.False(changed)

	newToken := "some updated token"
	changed, err = di.updateDeviceInfo(di.PushService, newToken, di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newToken, di.PushToken)
	s.Equal("", di.AWSEndpointArn)

	newService := "GCM"
	changed, err = di.updateDeviceInfo(newService, di.PushToken, di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newService, di.PushService)
	s.NotEqual("", di.PushToken) // this should have gotten set to the passed in value (which is the same as before)
	s.Equal("", di.AWSEndpointArn)

	newPlatform := "android"
	changed, err = di.updateDeviceInfo(di.PushService, di.PushToken, newPlatform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newPlatform, di.Platform)
	s.NotEqual("", di.PushToken)   // this should have gotten set to the passed in value (which is the same as before)
	s.NotEqual("", di.PushService) // this should have gotten set to the passed in value (which is the same as before)
	s.Equal("", di.AWSEndpointArn)

	newOsVersion := "11111"
	changed, err = di.updateDeviceInfo(di.PushService, di.PushToken, di.Platform, newOsVersion, di.AppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newOsVersion, di.OSVersion)

	newAppBuildVersion := "22222"
	changed, err = di.updateDeviceInfo(di.PushService, di.PushToken, di.Platform, di.OSVersion, newAppBuildVersion, di.AppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newAppBuildVersion, di.AppBuildVersion)

	newAppBuildNumber := "33333"
	changed, err = di.updateDeviceInfo(di.PushService, di.PushToken, di.Platform, di.OSVersion, di.AppBuildVersion, newAppBuildNumber)
	s.NoError(err)
	s.True(changed)
	s.Equal(newAppBuildNumber, di.AppBuildNumber)
}

func (s *deviceInfoTester) TestDeviceInfoUpdateContact() {
	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.insert(nil)
	s.NoError(err)

	lastContact, _, err := di.getContactInfo(false)
	s.NoError(err)
	err = di.updateLastContact()
	s.NoError(err)

	lastContact2, _, err := di.getContactInfo(false)
	s.NoError(err)

	s.True(lastContact2 > lastContact)

	_, lastContactRequest, err := di.getContactInfo(false)
	s.NoError(err)
	err = di.updateLastContactRequest()
	s.NoError(err)

	_, lastContactRequest2, err := di.getContactInfo(false)
	s.NoError(err)

	s.True(lastContactRequest2 > lastContactRequest)
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
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.insert(nil)
	s.NoError(err)
	di, err = getDeviceInfo(s.db, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.NotNil(di)

	di.cleanup()

	di = nil

	di, err = getDeviceInfo(s.db, s.aws, s.testClientId, s.testClientContext, s.testDeviceId, s.sessionId, s.logger)
	s.NoError(err)
	s.Nil(di)
}

func (s *deviceInfoTester) TestRegisterAWS() {
	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppNumber,
		s.sessionId,
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)

	err = di.insert(nil)
	s.NoError(err)

	fmt.Println(di.AWSEndpointArn)
	s.Equal("", di.AWSEndpointArn)

	//s.aws.SetRegisteredEndpoint("foo12345")
	testArn := "arn:aws:sns:foo12345"
	s.aws.SetReturnRegisteredEndpoint("", fmt.Errorf("Endpoint %s already exists.", testArn))
	s.aws.SetReturnGetAttributes(make(map[string]string), nil)
	s.aws.SetReturnSetAttributes(nil)
	err = di.registerAws()
	s.NoError(err)
	s.Equal(testArn, di.AWSEndpointArn)
}

func (s *deviceInfoTester) TestSendToAll() {
	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppNumber,
		"1",
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)
	di.AWSEndpointArn = "12345"
	di.insert(nil)

	di, err = newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppNumber,
		"2",
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)
	di.AWSEndpointArn = "12345"
	di.insert(nil)

	deviceList, err := getAllMyDeviceInfo(s.db, s.aws, s.logger)
	s.Equal(2, len(deviceList))

	n := alertAllDevices(s.dbm, s.aws, s.logger)
	s.Equal(1, n)
}

func (s *deviceInfoTester) TestPingerStealing() {
	di, err := newDeviceInfo(
		s.testClientId,
		s.testClientContext,
		s.testDeviceId,
		s.testPushToken,
		s.testPushService,
		s.testPlatform,
		s.testOSVersion,
		s.testAppVersion,
		s.testAppNumber,
		"1",
		s.aws,
		s.db,
		s.logger)
	s.NoError(err)
	require.NotNil(s.T(), di)
	di.AWSEndpointArn = "12345"
	di.insert(nil)
	s.Equal(pingerHostId, di.Pinger)

	pingerHostId = "12345"

	d1, err := getDeviceInfo(s.db, s.aws, di.ClientId, di.ClientContext, di.DeviceId, di.SessionId, s.logger)
	s.NoError(err)
	require.NotNil(s.T(), d1)

	s.Equal(pingerHostId, d1.Pinger)
	s.NotEqual(pingerHostId, di.Pinger)
}
