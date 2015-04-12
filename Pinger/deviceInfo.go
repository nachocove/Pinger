package Pinger

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"regexp"
	"strings"
)

type DbHandler interface {
	insert(i interface{}) error
	update(i interface{}) (int64, error)
	delete(i interface{}) error
	get(args ...interface{}) (interface{}, error)
}
type DeviceInfoDbHandler interface {
	DbHandler
	findByPingerId(pingerId string) ([]DeviceInfo, error)
}

type DeviceInfo struct {
	Id              int64  `db:"id"`
	Created         int64  `db:"created"`
	Updated         int64  `db:"updated"`
	ClientId        string `db:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext   string `db:"client_context"`
	DeviceId        string `db:"device_id"` // NCHO348348384384.....
	SessionId       string `db:"session_id"`
	Platform        string `db:"device_platform"` // "ios", "android", etc..
	PushToken       string `db:"push_token"`
	PushService     string `db:"push_service"` // APNS, GCM, ...
	OSVersion       string `db:"os_version"`
	AppBuildVersion string `db:"build_version"`
	AppBuildNumber  string `db:"build_number"`
	AWSEndpointArn  string `db:"aws_endpoint_arn"`
	Pinger          string `db:"pinger"`

	db DeviceInfoDbHandler    `db:"-"`
	logger    *Logging.Logger `db:"-"`
	logPrefix string          `db:"-"`
	aws       AWS.AWSHandler  `db:"-"`
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
	if di.Platform == "" {
		return errors.New("Platform can not be empty")
	} else {
		matched, err := regexp.MatchString("(ios|android)", di.Platform)
		if err != nil {
			return err
		}
		if matched == false {
			return fmt.Errorf("Platform %s is not known", di.Platform)
		}
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
	db DeviceInfoDbHandler,
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
		db: db,
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

func (di *DeviceInfo) delete() error {
	return di.db.delete(di)
}

func (di *DeviceInfo) cleanup() {
	di.Debug("Cleaning up DeviceInfo")
	err := di.db.delete(di)
	if err != nil {
		di.Error("Not deleted from DB: %s", err)
	}
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

func getDeviceInfo(db DeviceInfoDbHandler, aws AWS.AWSHandler, clientId, clientContext, deviceId, sessionId string, logger *Logging.Logger) (*DeviceInfo, error) {
	obj, err := db.get(clientId, clientContext, deviceId, sessionId)
	if err != nil {
		return nil, err
	}
	var di *DeviceInfo
	if obj != nil {
		di = obj.(*DeviceInfo)
		if di == nil {
			panic("di can not be nil")
		}
		di.db = db
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

func getAllMyDeviceInfo(db DeviceInfoDbHandler, aws AWS.AWSHandler, logger *Logging.Logger) ([]DeviceInfo, error) {
	devices, err := db.findByPingerId(pingerHostId)
	if err != nil {
		return nil, err
	}
	for k := range devices {
		devices[k].aws = aws
		devices[k].SetLogger(logger)
	}
	return devices, nil
}

func (di *DeviceInfo) updateDeviceInfo(
	clientContext, deviceId,
	pushService, pushToken,
	platform, osVersion,
	appBuildVersion, appBuildNumber string) (bool, error) {
	changed := false
	if di.ClientContext != clientContext {
		di.ClientContext = clientContext
		changed = true
	}
	if di.DeviceId != deviceId {
		di.DeviceId = deviceId
		changed = true
	}
	if di.OSVersion != osVersion {
		di.OSVersion = osVersion
		changed = true
	}
	if di.AppBuildVersion != appBuildVersion {
		di.AppBuildVersion = appBuildVersion
		changed = true
	}
	if di.AppBuildNumber != appBuildNumber {
		di.AppBuildNumber = appBuildNumber
		changed = true
	}
	if di.Platform != platform {
		di.Platform = platform
		changed = true
	}
	// TODO if the push token or service change, then the AWS endpoint is no longer valid: We should send a delete to AWS for the endpoint
	if di.PushService != pushService {
		di.Warning("Resetting Token ('%s') and AWSEndpointArn ('%s')", di.PushToken, di.AWSEndpointArn)
		di.PushService = pushService
		di.PushToken = ""
		di.AWSEndpointArn = ""
		changed = true
	}
	if di.PushToken != pushToken {
		di.Warning("Resetting AWSEndpointArn ('%s')", di.AWSEndpointArn)
		di.PushToken = pushToken
		di.AWSEndpointArn = ""
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
	if di.db == nil {
		panic("Can not update device info without having fetched it")
	}
	n, err := di.db.update(di)
	if err != nil {
		panic(fmt.Sprintf("%s: update error: %s", di.getLogPrefix(), err.Error()))
	}
	return n, nil
}

func (di *DeviceInfo) insert(db DeviceInfoDbHandler) error {
	if db == nil {
		db = di.db
	}
	if db == nil {
		panic("Can not insert device info without db information")
	}
	if di.db == nil {
		di.db = db
	}
	err := db.insert(di)
	if err != nil {
		panic(fmt.Sprintf("%s: insert error: %s", di.getLogPrefix(), err.Error()))
	}
	dc, err := di.getContactInfoObj(true)
	if err != nil {
		panic(fmt.Sprintf("%s: insert error(1): %s", di.getLogPrefix(), err.Error()))
	}
	if dc == nil {
		panic(fmt.Sprintf("%s: insert error(2)", di.getLogPrefix()))
	}
	return nil
}

type PingerNotification string

const (
	PingerNotificationRegister PingerNotification = "register"
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
	contextIds := []string{di.ClientContext}
	pingerMap := pingerPushMessageMap(message, contextIds, di.SessionId)
	ttl := globals.config.APNSExpirationSeconds
	err := Push(di.aws, di.Platform, di.PushService, di.PushToken, di.AWSEndpointArn, alert, sound, contentAvailable, ttl, pingerMap, di.logger)
	if err == nil {
		err = di.updateLastContactRequest()
	}
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

var alreadyRegisted *regexp.Regexp

func init() {
	alreadyRegisted = regexp.MustCompile("^.*Endpoint (?P<arn>arn:aws:sns:[^ ]+) already exists.*$")
}

func (di *DeviceInfo) registerAws() error {
	var pushToken string
	var err error
	var attributes map[string]string
	need_di_update := false
	need_attr_update := false

	if di.AWSEndpointArn == "" {
		// Need to register first
		switch {
		case di.PushService == PushServiceAPNS:
			pushToken, err = AWS.DecodeAPNSPushToken(di.PushToken)
			if err != nil {
				return err
			}
			if len(pushToken) != 64 {
				return fmt.Errorf("APNS token length wrong. %d ('%s')", len(pushToken), string(pushToken))
			}

		default:
			return fmt.Errorf("Unsupported push service %s:%s", di.PushService, di.PushToken)
		}

		di.Debug("Registering %s:%s with AWS.", di.PushService, di.PushToken)
		arn, registerErr := di.aws.RegisterEndpointArn(di.PushService, pushToken, di.customerData())
		if registerErr != nil {
			if alreadyRegisted.MatchString(registerErr.Error()) == true {
				replaceString := fmt.Sprintf("${%s}", alreadyRegisted.SubexpNames()[1])
				arn = alreadyRegisted.ReplaceAllString(registerErr.Error(), replaceString)
				di.Warning("Previously registered as %s. Updating.", arn)
			} else {
				return registerErr
			}
		} else {
			di.Debug("endpoint created %s", arn)
		}
		di.AWSEndpointArn = arn
		need_di_update = true
	}

	// fetch the attributes
	di.Debug("fetching attributes for %s.", di.AWSEndpointArn)
	attributes, err = di.aws.GetEndpointAttributes(di.AWSEndpointArn)
	if err != nil {
		return err
	}
	if attributes == nil {
		panic("attributes should not be nil")
	}
	enabled, ok := attributes["Enabled"]
	if !ok || enabled != "true" {
		if enabled != "true" {
			di.Warning("AWS has endpoint disabled. Reenabling it")
			attributes["Enabled"] = "true"
			need_attr_update = true
		}
	}

	if attributes["Token"] == "" || (pushToken != "" && pushToken != attributes["Token"]) {
		// need to update the token with aws
		attributes["Token"] = pushToken
		need_attr_update = true
	}

	cd := di.customerData()
	if cd != attributes["CustomUserData"] {
		attributes["CustomUserData"] = cd
		need_attr_update = true
	}
	if need_attr_update {
		di.Debug("Setting new attributes for %s: %+v", di.AWSEndpointArn, attributes)
		err := di.aws.SetEndpointAttributes(di.AWSEndpointArn, attributes)
		if err != nil {
			di.Debug("Could not set attributes")
			return err
		}
	}
	if need_di_update {
		di.update()
	}
	return nil
}

func (di *DeviceInfo) validateClient() error {
	if strings.EqualFold(di.PushService, PushServiceAPNS) == false || globals.config.APNSCertFile == "" || globals.config.APNSKeyFile == "" {
		// TODO Can we cache the validation results here? Can they change once a client ID has been invalidated? How do we even invalidate one?
		err := di.registerAws()
		if err != nil {
			if di.aws.IgnorePushFailures() == false {
				return err
			} else {
				di.Warning("Registering %s:%s error (ignored): %s", di.PushService, di.PushToken, err.Error())
			}
		}
	}
	return nil
}

