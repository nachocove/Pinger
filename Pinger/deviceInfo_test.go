package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var dbmap *gorp.DbMap

func TestMain(m *testing.M) {
	var err error
	logger := logging.MustGetLogger("unittest")
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

func TestDeviceInfoCreate(t *testing.T) {
	var err error

	assert := assert.New(t)

	testClientID := "clientID"
	testClientContext := "clientContext"
	testPushToken := "pushToken"
	testPushService := "pushService"
	testPlatform := "ios"

	deviceList, err := getAllMyDeviceInfo(dbmap)
	assert.Equal(len(deviceList), 0)

	di, err := newDeviceInfo(testClientID, testClientContext, testPushToken, testPushService, testPlatform)
	assert.NoError(err)
	assert.NotNil(di)

	assert.Equal(testClientID, di.ClientId)
	assert.Equal(testClientContext, di.ClientContext)
	assert.Equal(testPushToken, di.PushToken)
	assert.Equal(testPushService, di.PushService)
	assert.Equal(testPlatform, di.Platform)

	assert.Equal(0, di.Id)
	assert.Empty(di.Pinger)
	assert.Equal(0, di.Created)
	assert.Equal(0, di.Updated)
	assert.Equal(0, di.LastContact)
	assert.Equal(0, di.LastContactRequest)
	assert.Empty(di.AWSEndpointArn)

	deviceList, err = getAllMyDeviceInfo(dbmap)
	assert.Equal(0, len(deviceList))

	diInDb, err := getDeviceInfo(dbmap, testClientID)
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

	diInDb, err = getDeviceInfo(dbmap, testClientID)
	assert.NoError(err)
	assert.NotNil(diInDb)
	assert.Equal(di.Id, diInDb.Id)

	deviceList, err = getAllMyDeviceInfo(dbmap)
	assert.NoError(err)
	assert.Equal(1, len(deviceList))
}

func TestDeviceInfoUpdate(t *testing.T) {
	assert := assert.New(t)

	deviceList, err := getAllMyDeviceInfo(dbmap)
	assert.NoError(err)
	assert.Equal(1, len(deviceList))
	di := deviceList[0]
	assert.NotNil(di.dbm)
	fmt.Printf("di.dbm is%+v\n", di.dbm)

	di.AWSEndpointArn = "some endpoint"
	_, err = di.update()
	assert.NoError(err)
	assert.NotEmpty(di.AWSEndpointArn)

	changed, err := di.updateDeviceInfo(di.PushService, di.PushToken, di.Platform)
	assert.NoError(err)
	assert.False(changed)
	assert.NotEmpty(di.AWSEndpointArn)
	
	newToken := "some updated token"
	changed, err = di.updateDeviceInfo(di.PushService, newToken, di.Platform)
	assert.NoError(err)
	assert.True(changed)
	assert.Equal(newToken, di.PushToken)
	assert.Empty(di.AWSEndpointArn)
}
