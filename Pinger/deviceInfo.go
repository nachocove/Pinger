package Pinger

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"reflect"
	"regexp"
	"strings"
	"time"
)

type DeviceInfo struct {
	Id              int64  `db:"id"`
	Created         int64  `db:"created"`
	Updated         int64  `db:"updated"`
	ClientId        string `db:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext   string `db:"client_context"`
	DeviceId        string `db:"device_id"`       // NCHO348348384384.....
	Platform        string `db:"device_platform"` // "ios", "android", etc..
	PushToken       string `db:"push_token"`
	PushService     string `db:"push_service"` // APNS, GCM, ...
	OSVersion       string `db:"os_version"`
	AppBuildVersion string `db:"build_version"`
	AppBuildNumber  string `db:"build_number"`
	AWSEndpointArn  string `db:"aws_endpoint_arn"`
	Pinger          string `db:"pinger"`

	dbm       *gorp.DbMap     `db:"-"`
	logger    *Logging.Logger `db:"-"`
	logPrefix string          `db:"-"`
	aws       AWS.AWSHandler  `db:"-"`
	sessionId string          `db:"-"`
}

type deviceContact struct {
	Id                 int64  `db:"id"`
	Created            int64  `db:"created"`
	Updated            int64  `db:"updated"`
	LastContact        int64  `db:"last_contact"`
	LastContactRequest int64  `db:"last_contact_request"`
	ClientId           string `db:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext      string `db:"client_context"`
	DeviceId           string `db:"device_id"` // NCHO348348384384.....
}

const (
	deviceTableName        string = "device_info"
	deviceContactTableName string = "device_contact"
)

func addDeviceInfoTable(dbmap *gorp.DbMap) {
	tMap := dbmap.AddTableWithName(DeviceInfo{}, deviceTableName)
	if tMap.SetKeys(false, "ClientId", "ClientContext", "DeviceId") == nil {
		panic(fmt.Sprintf("Could not create key on %s:ID", deviceTableName))
	}
	tMap.SetVersionCol("Id")

	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientId")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientContext")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("DeviceId")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Platform")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("PushToken")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("PushService")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("OSVersion")
	cMap.SetNotNull(false)

	cMap = tMap.ColMap("AppBuildNumber")
	cMap.SetNotNull(false)

	cMap = tMap.ColMap("AppBuildVersion")
	cMap.SetNotNull(false)

	cMap = tMap.ColMap("Pinger")
	cMap.SetNotNull(true)
}

func addDeviceContactTable(dbmap *gorp.DbMap) {
	tMap := dbmap.AddTableWithName(deviceContact{}, deviceContactTableName)
	if tMap.SetKeys(false, "ClientId", "ClientContext", "DeviceId") == nil {
		panic(fmt.Sprintf("Could not create key on %s:ID", deviceContactTableName))
	}
	tMap.SetVersionCol("Id")

	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("LastContact")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientId")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientContext")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("DeviceId")
	cMap.SetNotNull(true)
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
	logger *Logging.Logger) (*DeviceInfo, error) {
	if sessionId == "" {
		panic("session ID needs to be set")
	}
	di := &DeviceInfo{
		ClientId:        clientID,
		ClientContext:   clientContext,
		DeviceId:        deviceId,
		PushToken:       pushToken,
		PushService:     pushService,
		Platform:        platform,
		OSVersion:       osVersion,
		AppBuildVersion: appBuildVersion,
		AppBuildNumber:  appBuildNumber,
		aws:             aws,
		sessionId:       sessionId,
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
		di.logPrefix = fmt.Sprintf("%s:%s:%s:%s", di.DeviceId, di.ClientId, di.ClientContext, di.sessionId)
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

func (di *DeviceInfo) cleanup() {
	di.Debug("Cleaning up DeviceInfo")
	n, err := di.dbm.Delete(di)
	if n == 0 {
		di.Error("Not deleted from DB!")
	}
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

var getAllMyDeviceInfoSql string

func init() {
	var ok bool
	var deviceInfoReflection reflect.Type
	var pingerField reflect.StructField
	deviceInfoReflection = reflect.TypeOf(DeviceInfo{})
	pingerField, ok = deviceInfoReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	getAllMyDeviceInfoSql = fmt.Sprintf("select * from %s where %s=?",
		deviceTableName,
		pingerField.Tag.Get("db"))
}

func getDeviceInfo(dbm *gorp.DbMap, aws AWS.AWSHandler, clientId, clientContext, deviceId, sessionId string, logger *Logging.Logger) (*DeviceInfo, error) {
	var device *DeviceInfo
	obj, err := dbm.Get(&DeviceInfo{}, clientId, clientContext, deviceId)
	if err != nil {
		return nil, err
	}
	if obj != nil {
		device = obj.(*DeviceInfo)
		device.dbm = dbm
		device.aws = aws
		device.sessionId = sessionId
		device.SetLogger(logger)
		if device.Pinger != pingerHostId {
			device.Warning("device belongs to a different pinger (%s). Stealing it", device.Pinger)
			device.Pinger = pingerHostId
			device.update()
		}
	}
	return device, nil
}

func getAllMyDeviceInfo(dbm *gorp.DbMap, aws AWS.AWSHandler, logger *Logging.Logger) ([]DeviceInfo, error) {
	var devices []DeviceInfo
	var err error
	_, err = dbm.Select(&devices, getAllMyDeviceInfoSql, pingerHostId)
	if err != nil {
		return nil, err
	}
	for k := range devices {
		devices[k].dbm = dbm
		devices[k].aws = aws
		devices[k].SetLogger(logger)
	}
	return devices, nil
}

func (di *DeviceInfo) PreUpdate(s gorp.SqlExecutor) error {
	di.Updated = time.Now().UnixNano()
	if di.Pinger == "" {
		di.Pinger = pingerHostId
	}
	return di.validate()
}

func (di *DeviceInfo) PreInsert(s gorp.SqlExecutor) error {
	di.Created = time.Now().UnixNano()
	di.Updated = di.Created

	if di.Pinger == "" {
		di.Pinger = pingerHostId
	}
	return di.validate()
}

func (dc *deviceContact) PreInsert(s gorp.SqlExecutor) error {
	dc.Created = time.Now().UnixNano()
	dc.Updated = dc.Created
	dc.LastContact = dc.Created
	return nil
}

func (di *DeviceInfo) updateLastContact() error {
	dc, err := di.getContactInfoObj(false)
	if err != nil {
		return err
	}
	dc.LastContact = time.Now().UnixNano()
	_, err = di.dbm.Update(dc)
	if err != nil {
		return err
	}
	return nil
}

func (di *DeviceInfo) updateLastContactRequest() error {
	dc, err := di.getContactInfoObj(false)
	if err != nil {
		return err
	}
	dc.LastContactRequest = time.Now().UnixNano()
	_, err = di.dbm.Update(dc)
	if err != nil {
		return err
	}
	return nil
}

func (di *DeviceInfo) getContactInfoObj(insert bool) (*deviceContact, error) {
	if di.dbm == nil {
		panic("Must have fetched di first")
	}
	obj, err := di.dbm.Get(&deviceContact{}, di.ClientId, di.ClientContext, di.DeviceId)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		if insert {
			dc := &deviceContact{
				ClientId:      di.ClientId,
				ClientContext: di.ClientContext,
				DeviceId:      di.DeviceId,
			}
			err = di.dbm.Insert(dc)
			if err != nil {
				panic(err)
			}
			obj, err = di.dbm.Get(&deviceContact{}, di.ClientId, di.ClientContext, di.DeviceId)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("No object found")
		}
	}
	dc := obj.(*deviceContact)
	return dc, nil
}

func (di *DeviceInfo) getContactInfo(insert bool) (int64, int64, error) {
	dc, err := di.getContactInfoObj(insert)
	if err != nil {
		return 0, 0, err
	}
	return dc.LastContact, dc.LastContactRequest, nil
}

func newDeviceInfoPI(dbm *gorp.DbMap, aws AWS.AWSHandler, pi *MailPingInformation, logger *Logging.Logger) (*DeviceInfo, error) {
	var err error
	di, err := getDeviceInfo(dbm, aws, pi.ClientId, pi.ClientContext, pi.DeviceId, pi.SessionId, logger)
	if err != nil {
		return nil, err
	}
	if di == nil {
		di, err = newDeviceInfo(
			pi.ClientId,
			pi.ClientContext,
			pi.DeviceId,
			pi.PushToken,
			pi.PushService,
			pi.Platform,
			pi.OSVersion,
			pi.AppBuildVersion,
			pi.AppBuildNumber,
			pi.SessionId,
			aws,
			logger)
		if err != nil {
			return nil, err
		}
		if di == nil {
			return nil, fmt.Errorf("Could not create DeviceInfo")
		}
		err = di.insert(dbm)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := di.updateDeviceInfo(pi.ClientContext, pi.DeviceId, pi.PushService, pi.PushToken, pi.Platform, pi.OSVersion, pi.AppBuildVersion, pi.AppBuildNumber)
		if err != nil {
			return nil, err
		}
	}
	return di, nil
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
	if di.dbm == nil {
		panic("Can not update device info without having fetched it")
	}
	n, err := di.dbm.Update(di)
	if err != nil {
		panic(fmt.Sprintf("%s: update error: %s", di.getLogPrefix(), err.Error()))
	}
	return n, nil
}

func (di *DeviceInfo) insert(dbm *gorp.DbMap) error {
	if dbm == nil {
		dbm = di.dbm
	}
	if dbm == nil {
		panic("Can not insert device info without db information")
	}
	if di.dbm == nil {
		di.dbm = dbm
	}
	err := dbm.Insert(di)
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

func (di *DeviceInfo) pushMessage(message PingerNotification, alert string, ttl int64) (string, error) {
	if message == "" {
		return "", fmt.Errorf("Message can not be empty")
	}
	pingerMap := make(map[string]interface{})
	pingerMap[di.ClientContext] = string(message)
	pingerMap["timestamp"] = time.Now().UTC().Round(time.Millisecond).Format(Telemetry.TelemetryTimeZFormat)
	pingerMap["session"] = di.sessionId

	pingerJson, err := json.Marshal(pingerMap)
	if err != nil {
		return "", err
	}
	notificationMap := map[string]string{}
	notificationMap["default"] = string(pingerJson)

	switch {
	case di.Platform == "ios":
		APNSMap := map[string]interface{}{}
		APNSMap["pinger"] = pingerMap
		apsMap := make(map[string]interface{})
		apsMap["content-available"] = 1
		apsMap["sound"] = "silent.wav"
		if alert != "" {
			apsMap["alert"] = alert
		}
		APNSMap["aps"] = apsMap
		b, err := json.Marshal(APNSMap)
		if err != nil {
			return "", err
		}
		if len(b) > 256 {
			di.Warning("Length of push message is %d > 256", len(b))
		} else {
			di.Debug("Length of push message %d", len(b))
		}
		notificationMap["APNS"] = string(b)
		notificationMap["APNS_SANDBOX"] = string(b)

	case di.Platform == "android":
		hash := sha1.New()
		hash.Write(pingerJson)
		md := hash.Sum(nil)
		pingerMapSha := hex.EncodeToString(md)

		GCMMap := map[string]interface{}{}
		GCMMap["data"] = pingerMap
		GCMMap["collapse_key"] = string(pingerMapSha)
		GCMMap["time_to_live"] = ttl
		GCMMap["delay_while_idle"] = false

		b, err := json.Marshal(GCMMap)
		if err != nil {
			return "", err
		}
		notificationMap["GCM"] = string(b)
	}

	var notificationBytes []byte
	notificationBytes, err = json.Marshal(notificationMap)
	if err != nil {
		return "", err
	}
	if len(notificationBytes) == 0 {
		return "", fmt.Errorf("No notificationBytes created")
	}
	return string(notificationBytes), nil
}

type StringInterfaceMap map[string]interface{}
type APNSPushNotification struct {
	aps    *StringInterfaceMap
	pinger *StringInterfaceMap
}

func (di *DeviceInfo) push(message PingerNotification) error {
	var err error
	var alert string
	if message == "new" {
		alert = "Nacho says: You got mail."
	} else {
		alert = "Nacho says: Please register."
	}
	if strings.EqualFold(di.PushService, PushServiceAPNS) == false || globals.config.APNSCertFile == "" || globals.config.APNSKeyFile == "" {
		err = di.AWSpushMessage(message, alert)
	} else {
		err = di.APNSpushMessage(message, alert)
	}
	if err == nil {
		err = di.updateLastContactRequest()
	}
	return err
}

func (di *DeviceInfo) AWSpushMessage(message PingerNotification, alert string) error {
	if di.AWSEndpointArn == "" {
		return fmt.Errorf("Endpoint not registered: Token ('%s:%s')", di.PushService, di.PushToken)
	}
	var days_28 int64 = 2419200
	pushMessage, err := di.pushMessage(message, alert, days_28)
	if err != nil {
		return err
	}
	di.Debug("Sending push message to AWS: pushToken: %s/%s AWSEndpointArn:%s %s", di.PushService, di.PushToken, di.AWSEndpointArn, pushMessage)
	return di.aws.SendPushNotification(di.AWSEndpointArn, pushMessage)
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

func alertAllDevices(dbm *gorp.DbMap, aws AWS.AWSHandler, logger *Logging.Logger) error {
	devices, err := getAllMyDeviceInfo(dbm, aws, logger)
	if err != nil {
		return err
	}
	count := 0
	for _, di := range devices {
		logger.Info("%s: sending PingerNotificationRegister to device", di.getLogPrefix())
		err = di.push(PingerNotificationRegister)
		if err != nil {
			logger.Warning("%s: Could not send push: %s", di.getLogPrefix(), err.Error())
		} else {
			count++
		}
		if count >= 10 {
			count = 0
			time.Sleep(time.Duration(1) * time.Second)
		}
	}
	return nil
}
