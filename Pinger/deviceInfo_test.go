package Pinger

import (
	"github.com/coopernurse/gorp"
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var dbmap *gorp.DbMap
var logger *logging.Logger = logging.MustGetLogger("unittest")
var testClientID = "clientID"
var testClientContext = "clientContext"
var testDeviceId = "NCHOXfherekgrgr"
var testPushToken = "pushToken"
var testPushService = "pushService"
var testPlatform = "ios"
var testOSVersion = "8.1"
var testAppVersion = "0.9"
var testAppnumber = "(dev) Foo"

func TestMain(m *testing.M) {
	var err error
	testDbFilename := "unittest.db"
	os.Remove(testDbFilename)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: testDbFilename}
	dbmap, err = initDB(&dbconfig, true, true, logger)
	if err != nil {
		panic("Could not create DB")
	}
	defer os.Remove(testDbFilename)
	os.Exit(m.Run())
}

func TestDeviceInfoValidate(t *testing.T) {
	var err error

	assert := assert.New(t)

	di, err := newDeviceInfo(
		testClientID,
		testClientContext,
		testDeviceId,
		testPushToken,
		testPushService,
		testPlatform,
		testOSVersion,
		testAppVersion,
		testAppnumber,
		logger)
	assert.NoError(err)
	assert.NotNil(di)
	
	err = di.validate()
	assert.NoError(err)
	
	di.ClientId = ""
	err = di.validate()
	assert.EqualError(err, "ClientID can not be empty")
	di.ClientId = testClientID
	
	di.ClientContext = ""
	err = di.validate()
	assert.EqualError(err, "ClientContext can not be empty")
	di.ClientContext = testClientContext

	di.DeviceId = ""
	err = di.validate()
	assert.EqualError(err, "DeviceId can not be empty")
	di.DeviceId = testDeviceId

	di.Platform = ""
	err = di.validate()
	assert.EqualError(err, "Platform can not be empty")
	
	di.Platform = "foo"
	err = di.validate()
	assert.EqualError(err, "Platform foo is not known")
	di.Platform = testPlatform
	
	di.cleanup()
}

func TestDeviceInfoCleanup(t *testing.T) {
	var err error

	assert := assert.New(t)

	di, err := newDeviceInfo(
		testClientID,
		testClientContext,
		testDeviceId,
		testPushToken,
		testPushService,
		testPlatform,
		testOSVersion,
		testAppVersion,
		testAppnumber,
		logger)
	assert.NoError(err)
	assert.NotNil(di)
	
	di.cleanup()
	assert.Equal("", di.ClientId)
	assert.Equal("", di.ClientContext)
	assert.Equal("", di.DeviceId)
	assert.Equal("", di.PushToken)
	assert.Equal("", di.PushService)
	assert.Equal("", di.Platform)
	assert.Equal("", di.OSVersion)
	assert.Equal("", di.AppBuildNumber)
	assert.Equal("", di.AppBuildVersion)
	assert.Equal(0, di.Id)
}

func TestDeviceInfoCreate(t *testing.T) {
	var err error

	assert := assert.New(t)

	deviceList, err := getAllMyDeviceInfo(dbmap, logger)
	assert.Equal(len(deviceList), 0)

	di, err := newDeviceInfo(
		testClientID,
		"",
		testDeviceId,
		testPushToken,
		testPushService,
		testPlatform,
		testOSVersion,
		testAppVersion,
		testAppnumber,
		logger)
	assert.Error(err, "ClientContext can not be empty")
	assert.Nil(di)

	di, err = newDeviceInfo(
		testClientID,
		testClientContext,
		testDeviceId,
		testPushToken,
		testPushService,
		testPlatform,
		testOSVersion,
		testAppVersion,
		testAppnumber,
		logger)
	assert.NoError(err)
	assert.NotNil(di)

	assert.Equal(testClientID, di.ClientId)
	assert.Equal(testClientContext, di.ClientContext)
	assert.Equal(testDeviceId, di.DeviceId)
	assert.Equal(testPushToken, di.PushToken)
	assert.Equal(testPushService, di.PushService)
	assert.Equal(testPlatform, di.Platform)
	assert.Equal(testOSVersion, di.OSVersion)
	assert.Equal(testAppVersion, di.AppBuildVersion)
	assert.Equal(testAppnumber, di.AppBuildNumber)

	assert.Equal(0, di.Id)
	assert.Empty(di.Pinger)
	assert.Equal(0, di.Created)
	assert.Equal(0, di.Updated)
	assert.Equal(0, di.LastContact)
	assert.Equal(0, di.LastContactRequest)
	assert.Empty(di.AWSEndpointArn)

	deviceList, err = getAllMyDeviceInfo(dbmap, logger)
	assert.Equal(0, len(deviceList))

	diInDb, err := getDeviceInfo(dbmap, testClientID, testClientContext, testDeviceId, logger)
	assert.NoError(err)
	assert.Nil(diInDb)

	err = di.insert(dbmap)
	assert.Equal(1, di.Id)
	assert.NoError(err)
	assert.NotEmpty(di.Pinger)
	assert.True(di.Created > 0)
	assert.True(di.Updated > 0)
	assert.True(di.LastContact > 0)
	assert.Equal(0, di.LastContactRequest)
	assert.Empty(di.AWSEndpointArn)

	assert.Equal(pingerHostId, di.Pinger)

	diInDb, err = getDeviceInfo(dbmap, testClientID, testClientContext, testDeviceId, logger)
	assert.NoError(err)
	assert.NotNil(diInDb)
	assert.Equal(di.Id, diInDb.Id)

	deviceList, err = getAllMyDeviceInfo(dbmap, logger)
	assert.NoError(err)
	assert.Equal(1, len(deviceList))
	di.cleanup()
}

func TestDeviceInfoUpdate(t *testing.T) {
	assert := assert.New(t)

	deviceList, err := getAllMyDeviceInfo(dbmap, logger)
	assert.NoError(err)
	assert.Equal(1, len(deviceList))
	di := &deviceList[0]
	assert.NotNil(di.dbm)

	di.AWSEndpointArn = "some endpoint"
	_, err = di.update()
	assert.NoError(err)
	assert.NotEmpty(di.AWSEndpointArn)

	changed, err := di.updateDeviceInfo(di.ClientContext, di.DeviceId, di.PushService, di.PushToken,
		di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	assert.NoError(err)
	assert.False(changed)
	assert.NotEmpty(di.AWSEndpointArn)

	newToken := "some updated token"
	changed, err = di.updateDeviceInfo(di.ClientContext, di.DeviceId, di.PushService, newToken,
		di.Platform, di.OSVersion, di.AppBuildVersion, di.AppBuildNumber)
	assert.NoError(err)
	assert.True(changed)
	assert.Equal(newToken, di.PushToken)
	assert.Empty(di.AWSEndpointArn)
	
	assert.True(di.LastContact > 0)
	lastContext := di.LastContact
	err = updateLastContact(dbmap, di.ClientId, di.ClientContext, di.DeviceId, logger)
	assert.NoError(err)
	
	di, err = getDeviceInfo(dbmap, di.ClientId, di.ClientContext, di.DeviceId, logger)
	assert.NoError(err)
	assert.NotNil(di)
		
	assert.True(di.LastContact > lastContext)	
	di.cleanup()
}

func TestDeviceInfoDelete(t *testing.T) {
	var err error

	assert := assert.New(t)

	di, err := newDeviceInfo(
		testClientID,
		testClientContext,
		testDeviceId,
		testPushToken,
		testPushService,
		testPlatform,
		testOSVersion,
		testAppVersion,
		testAppnumber,
		logger)
	assert.NoError(err)
	assert.NotNil(di)

	di.cleanup()
	
	di = nil
	
	di, err = getDeviceInfo(dbmap, testClientID, testClientContext, testDeviceId, logger)
	assert.NoError(err)
	assert.NotNil(di)
}

func TestDevicePushMessageCreate(t *testing.T) {
	assert := assert.New(t)
	di := DeviceInfo{Platform: "ios", PushService: PushServiceAPNS, ClientContext: "FOO"}
	var days_28 int64 = 2419200

	message, err := di.pushMessage(PingerNotificationRegister, days_28)
	assert.NoError(err)
	assert.NotEmpty(message)
	assert.Equal(
		"{\"APNS\":\"{\\\"aps\\\":{\\\"content-available\\\":1},\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}}\",\"APNS_SANDBOX\":\"{\\\"aps\\\":{\\\"content-available\\\":1},\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}}\",\"GCM\":\"{\\\"collapse_key\\\":\\\"10e23d6b0b515fbff01dff49948afebea929a763\\\",\\\"data\\\":{\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}},\\\"delay_while_idle\\\":false,\\\"time_to_live\\\":2419200}\",\"default\":\"{\\\"pinger\\\":{\\\"FOO\\\":\\\"register\\\"}}\"}",
		message)
}
