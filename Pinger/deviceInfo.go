package Pinger

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"reflect"
	"regexp"
	"time"
)

type DeviceInfoDbHandler interface {
	insert(di *DeviceInfo) error
	update(di *DeviceInfo) (int64, error)
	delete(di *DeviceInfo) (int64, error)
	get(keys []AWS.DBKeyValue) (*DeviceInfo, error)
	distinctPushServiceTokens(pingerHostId string) ([]DeviceInfo, error)
	clientContexts(pushService, pushToken string) ([]string, error)
	getAllMyDeviceInfo(pingerHostId string) ([]DeviceInfo, error)
	createTable() error
}

func newDeviceInfoDbHandler(db DBHandler) DeviceInfoDbHandler {
	if _, ok := db.(*DBHandleSql); ok {
		return newDeviceInfoSqlHandler(db)
	} else {
		return newDeviceInfoDynamoDbHandler(db)
	}
}

type DeviceInfo struct {
	Id            int64  `db:"id" dynamo:"id"`
	Created       int64  `db:"created" dynamo:"created"`
	Updated       int64  `db:"updated" dynamo:"updated"`
	ClientId      string `db:"client_id" dynamo:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext string `db:"client_context" dynamo:"client_context"`
	DeviceId      string `db:"device_id" dynamo:"device_id"` // NCHO348348384384.....
	PushToken     string `db:"push_token" dynamo:"push_token"`
	PushService   string `db:"push_service" dynamo:"push_service"` // APNS, GCM, ...
	Pinger        string `db:"pinger" dynamo:"pinger"`
	SessionId     string `db:"session" dynamo:"session"`

	Platform        string `db:"-" dynamo:"-"` // "ios", "android", etc..
	OSVersion       string `db:"-" dynamo:"-"`
	AppBuildVersion string `db:"-" dynamo:"-"`
	AppBuildNumber  string `db:"-" dynamo:"-"`

	dbHandler DeviceInfoDbHandler `db:"-" dynamo:"-"`
	logger    *Logging.Logger     `db:"-" dynamo:"-"`
	logPrefix string              `db:"-" dynamo:"-"`
	aws       AWS.AWSHandler      `db:"-" dynamo:"-"`
}

var deviceInfoReflection reflect.Type
var diIdField, diClientIdField, diDeviceIdField, diClientContextField, diSessionIdField, diPingerField,
	diPushServiceField, diPushTokenField reflect.StructField

func init() {
	var ok bool
	deviceInfoReflection = reflect.TypeOf(DeviceInfo{})
	diIdField, ok = deviceInfoReflection.FieldByName("Id")
	if ok == false {
		panic("Could not get Id Field information")
	}
	diClientIdField, ok = deviceInfoReflection.FieldByName("ClientId")
	if ok == false {
		panic("Could not get ClientId Field information")
	}
	diDeviceIdField, ok = deviceInfoReflection.FieldByName("DeviceId")
	if ok == false {
		panic("Could not get DeviceId Field information")
	}
	diClientContextField, ok = deviceInfoReflection.FieldByName("ClientContext")
	if ok == false {
		panic("Could not get ClientContext Field information")
	}
	diSessionIdField, ok = deviceInfoReflection.FieldByName("SessionId")
	if ok == false {
		panic("Could not get SessionId Field information")
	}
	diPingerField, ok = deviceInfoReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	diPushServiceField, ok = deviceInfoReflection.FieldByName("PushService")
	if ok == false {
		panic("Could not get PushService Field information")
	}
	diPushTokenField, ok = deviceInfoReflection.FieldByName("PushToken")
	if ok == false {
		panic("Could not get PushToken Field information")
	}
}

func (di *DeviceInfo) validate() error {
	if di.ClientId == "" {
		return errors.New("ClientID can not be empty")
	}
	if di.ClientContext == "" {
		return errors.New("ClientContext can not be empty")
	}
	if di.DeviceId == "" {
		return errors.New("DeviceId can not be empty")
	}
	if di.PushService == "" {
		return errors.New("PushService can not be empty")
	} else {
		matched, err := regexp.MatchString("(APNS|GCM)", di.PushService)
		if err != nil {
			return err
		}
		if matched == false {
			return fmt.Errorf("PushService %s is not known", di.PushService)
		}
	}
	if di.PushToken == "" {
		return errors.New("PushToken can not be empty")
	}
	return nil
}

func newDeviceInfo(
	clientID, clientContext, deviceId,
	pushToken, pushService,
	platform, osVersion,
	appBuildVersion, appBuildNumber string,
	sessionId string,
	aws AWS.AWSHandler,
	db DBHandler,
	logger *Logging.Logger) (*DeviceInfo, error) {
	if sessionId == "" {
		panic("session ID needs to be set")
	}
	di := &DeviceInfo{
		ClientId:        clientID,
		ClientContext:   clientContext,
		DeviceId:        deviceId,
		SessionId:       sessionId,
		PushToken:       pushToken,
		PushService:     pushService,
		Platform:        platform,
		OSVersion:       osVersion,
		AppBuildVersion: appBuildVersion,
		AppBuildNumber:  appBuildNumber,
		aws:             aws,
		dbHandler:       newDeviceInfoDbHandler(db),
	}
	di.SetLogger(logger)
	err := di.validate()
	if err != nil {
		return nil, err
	}
	return di, nil
}

func (di *DeviceInfo) SetLogger(logger *Logging.Logger) {
	di.logger = logger.Copy()
	di.logger.SetCallDepth(1)
}

func (di *DeviceInfo) getLogPrefix() string {
	if di.logPrefix == "" {
		di.logPrefix = fmt.Sprintf("%s:%s:%s:%s", di.DeviceId, di.ClientId, di.ClientContext, di.SessionId)
	}
	return di.logPrefix
}

func (di *DeviceInfo) Debug(format string, args ...interface{}) {
	di.logger.Debug(fmt.Sprintf("%s: %s", di.getLogPrefix(), format), args...)
}

func (di *DeviceInfo) Info(format string, args ...interface{}) {
	di.logger.Info(fmt.Sprintf("%s: %s", di.getLogPrefix(), format), args...)
}

func (di *DeviceInfo) Error(format string, args ...interface{}) {
	di.logger.Error(fmt.Sprintf("%s: %s", di.getLogPrefix(), format), args...)
}

func (di *DeviceInfo) Warning(format string, args ...interface{}) {
	di.logger.Warning(fmt.Sprintf("%s: %s", di.getLogPrefix(), format), args...)
}

func (di *DeviceInfo) delete() (int64, error) {
	return di.dbHandler.delete(di)
}

func (di *DeviceInfo) cleanup() {
	di.Debug("Cleaning up DeviceInfo")
	n, err := di.dbHandler.delete(di)
	if n == 0 {
		di.Warning("Not deleted from DB!")
	}
	if err != nil {
		di.Error("Not deleted from DB: %s", err)
	}
	// TODO investigte if there's a way to memset(0x0) these fields, instead of
	// relying on the garbage collector to clean them up (i.e. assigning "" to them
	// really just moves the pointer, orphaning the previous string, which the garbage
	// collector them frees or reuses.
	di.ClientId = ""
	di.ClientContext = ""
	di.DeviceId = ""
	di.PushToken = ""
	di.PushService = ""
	di.Platform = ""
	di.OSVersion = ""
	di.AppBuildNumber = ""
	di.AppBuildVersion = ""
	di.Id = 0
}

func getDeviceInfo(db DBHandler, aws AWS.AWSHandler, clientId, clientContext, deviceId, sessionId string, logger *Logging.Logger) (*DeviceInfo, error) {
	keys := []AWS.DBKeyValue{
		AWS.DBKeyValue{Key: "ClientId", Value: clientId, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "ClientContext", Value: clientContext, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "DeviceId", Value: deviceId, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "SessionId", Value: sessionId, Comparison: AWS.KeyComparisonEq},
	}
	h := newDeviceInfoDbHandler(db)
	di, err := h.get(keys)
	if err != nil {
		return nil, err
	}
	if di != nil {
		di.dbHandler = h
		di.aws = aws
		di.SetLogger(logger)
		if di.Pinger != pingerHostId {
			di.Warning("device belongs to a different pinger (%s). Stealing it", di.Pinger)
			di.Pinger = pingerHostId
			di.update()
		}
	}
	return di, nil
}

func (di *DeviceInfo) updateDeviceInfo(pushService, pushToken string) (bool, error) {
	changed := false

	if di.PushService != pushService {
		di.Warning("Resetting Token ('%s')", di.PushToken)
		di.PushService = pushService
		di.PushToken = ""
		changed = true
	}
	if di.PushToken != pushToken {
		di.PushToken = pushToken
		changed = true
	}
	if changed {
		n, err := di.update()
		if err != nil {
			return false, err
		}
		if n <= 0 {
			return false, errors.New("No rows updated but should have")
		}
	}
	return changed, nil
}

func (di *DeviceInfo) update() (int64, error) {
	if di.dbHandler == nil {
		panic("Can not update device info without having fetched it")
	}
	di.Updated = time.Now().UnixNano()
	if di.Pinger == "" {
		di.Pinger = pingerHostId
	}
	err := di.validate()
	if err != nil {
		return 0, err
	}
	n, err := di.dbHandler.update(di)
	if err != nil {
		panic(fmt.Sprintf("%s: update error: %s", di.getLogPrefix(), err.Error()))
	}
	return n, nil
}

func (di *DeviceInfo) insert(db DBHandler) error {
	if db != nil {
		di.dbHandler = newDeviceInfoDbHandler(db)
	}
	if di.dbHandler == nil {
		panic("Can not insert device info without db information")
	}
	di.Created = time.Now().UnixNano()
	di.Updated = di.Created

	if di.Pinger == "" {
		di.Pinger = pingerHostId
	}
	err := di.validate()
	if err != nil {
		return err
	}
	err = di.dbHandler.insert(di)
	if err != nil {
		panic(fmt.Sprintf("%s: insert error: %s", di.getLogPrefix(), err.Error()))
	}
	return nil
}

type PingerNotification string

const (
	PingerNotificationRegister PingerNotification = "reg"
	PingerNotificationNewMail  PingerNotification = "new"
)

func (di *DeviceInfo) PushRegister() error {
	var alert string
	if globals.config.APNSAlert {
		alert = "Nacho says: Reregister!"
	}
	return di.Push(PingerNotificationRegister, alert, globals.config.APNSSound, globals.config.APNSContentAvailable)
}

func (di *DeviceInfo) PushNewMail() error {
	var alert string
	if globals.config.APNSAlert {
		alert = "Nacho says: You have mail!"
	}
	return di.Push(PingerNotificationNewMail, alert, globals.config.APNSSound, globals.config.APNSContentAvailable)
}

func (di *DeviceInfo) Push(message PingerNotification, alert, sound string, contentAvailable int) error {
	pingerMap := pingerPushMessageMapV2([](*sessionContextMessage){newSessionContextMessage(message, di.ClientContext, di.SessionId)})
	ttl := globals.config.APNSExpirationSeconds
	err := Push(di.PushService, di.PushToken, alert, sound, contentAvailable, ttl, pingerMap, di.logger)
	return err
}

func (di *DeviceInfo) customerData() string {
	customMap := make(map[string]string)
	customMap["deviceId"] = di.DeviceId
	customMap["clientId"] = di.ClientId
	customJson, err := json.Marshal(customMap)
	if err != nil {
		return ""
	}
	return string(customJson)
}
