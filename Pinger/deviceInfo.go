package Pinger

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"regexp"
	"time"

	"github.com/coopernurse/gorp"
	"github.com/op/go-logging"
)

const (
	PushServiceAPNS = "APNS"
)

type DeviceInfo struct {
	Id                 int64  `db:"id"`
	Created            int64  `db:"created"`
	Updated            int64  `db:"updated"`
	LastContact        int64  `db:"last_contact"`
	LastContactRequest int64  `db:"last_contact_request"`
	ClientId           string `db:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext      string `db:"client_context"`
	DeviceId           string `db:"device_id"`       // NCHO348348384384.....
	Platform           string `db:"device_platform"` // "ios", "android", etc..
	PushToken          string `db:"push_token"`
	PushService        string `db:"push_service"` // APNS, GCM, ...
	OSVersion          string `db:"os_version"`
	AppBuildVersion    string `db:"build_version"`
	AppBuildNumber     string `db:"build_number"`
	AWSEndpointArn     string `db:"aws_endpoint_arn"`
	Enabled            bool   `db:"enabled"`
	Pinger             string `db:"pinger"`

	dbm       *gorp.DbMap     `db:"-"`
	logger    *logging.Logger `db:"-"`
	logPrefix string          `db:"-"`
}

const (
	DeviceTableName string = "device_info"
)

func addDeviceInfoTable(dbmap *gorp.DbMap) error {
	tMap := dbmap.AddTableWithName(DeviceInfo{}, DeviceTableName)
	if tMap.SetKeys(true, "Id") == nil {
		log.Fatalf("Could not create key on DeviceInfo:ID")
	}
	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("LastContact")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientId")
	cMap.SetNotNull(true)
	clientCol := cMap

	cMap = tMap.ColMap("ClientContext")
	cMap.SetNotNull(true)
	contextCol := cMap

	cMap = tMap.ColMap("DeviceId")
	cMap.SetNotNull(true)
	deviceCol := cMap

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

	tMap = tMap.SetUniqueTogether(clientCol.ColumnName, contextCol.ColumnName, deviceCol.ColumnName)
	return nil
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
	logger *logging.Logger) (*DeviceInfo, error) {
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
		Enabled:         false,
		logger:          logger,
	}
	err := di.validate()
	if err != nil {
		return nil, err
	}
	return di, nil
}

func (di *DeviceInfo) cleanup() {
	di.logger.Debug("%s: Cleaning up DeviceInfo", di.getLogPrefix())
	di.ClientId = ""
	di.ClientContext = ""
	di.DeviceId = ""
	di.PushToken = ""
	di.PushService = ""
	di.Platform = ""
	di.OSVersion = ""
	di.AppBuildNumber = ""
	di.AppBuildVersion = ""
	di.dbm.Delete(di.Id)
	di.Id = 0
}

var pingerHostId string
var deviceInfoReflection reflect.Type
var clientIdField reflect.StructField
var contextField reflect.StructField
var deviceidField reflect.StructField
var pingerField reflect.StructField

func init() {
	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		if inter.Name[0:2] == "lo" {
			continue
		}
		if inter.HardwareAddr.String() == "" {
			continue
		}
		hash := sha256.New()
		hash.Write(inter.HardwareAddr)
		md := hash.Sum(nil)
		pingerHostId = hex.EncodeToString(md)
		break
	}
	var ok bool
	deviceInfoReflection = reflect.TypeOf(DeviceInfo{})
	clientIdField, ok = deviceInfoReflection.FieldByName("ClientId")
	if ok == false {
		panic("Could not get ClientId Field information")
	}
	contextField, ok = deviceInfoReflection.FieldByName("ClientContext")
	if ok == false {
		panic("Could not get ClientContext Field information")
	}
	deviceidField, ok = deviceInfoReflection.FieldByName("DeviceId")
	if ok == false {
		panic("Could not get DeviceId Field information")
	}
	pingerField, ok = deviceInfoReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
}

func getDeviceInfo(dbm *gorp.DbMap, clientId, clientContext, deviceId string, logger *logging.Logger) (*DeviceInfo, error) {
	var devices []DeviceInfo
	var err error
	sql := fmt.Sprintf(
		"select * from %s where %s=? and %s=? and %s=?",
		DeviceTableName,
		clientIdField.Tag.Get("db"),
		contextField.Tag.Get("db"),
		deviceidField.Tag.Get("db"),
	)
	logger.Debug("%s:%s:%s: looking for device in DB: %s", deviceId, clientId, clientContext, sql)
	_, err = dbm.Select(&devices, sql, clientId, clientContext, deviceId)
	if err != nil {
		return nil, err
	}
	switch {
	case len(devices) > 1:
		return nil, fmt.Errorf("More than one entry from select: %d", len(devices))

	case len(devices) == 0:
		return nil, nil

	case len(devices) == 1:
		device := &(devices[0])
		device.dbm = dbm
		device.logger = logger
		if device.Pinger != pingerHostId {
			logger.Warning("%s: device belongs to a different pinger (%s). Stealing it", device.getLogPrefix(), device.Pinger)
			device.Pinger = pingerHostId
			device.update()
		}
		return device, nil

	default:
		return nil, fmt.Errorf("Bad number of rows returned: %d", len(devices))
	}
}

func getAllMyDeviceInfo(dbm *gorp.DbMap, logger *logging.Logger) ([]DeviceInfo, error) {
	var devices []DeviceInfo
	var err error
	_, err = dbm.Select(
		&devices,
		fmt.Sprintf("select * from %s where %s=?", DeviceTableName, pingerField.Tag.Get("db")),
		pingerHostId)
	if err != nil {
		return nil, err
	}
	for k := range devices {
		devices[k].dbm = dbm
		devices[k].logger = logger
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
	di.LastContact = di.Created

	if di.Pinger == "" {
		di.Pinger = pingerHostId
	}
	return di.validate()
}

func updateLastContact(dbm *gorp.DbMap, clientId, clientContext, deviceId string, logger *logging.Logger) error {
	di, err := getDeviceInfo(dbm, clientId, clientContext, deviceId, logger)
	if err != nil {
		return err
	}
	di.LastContact = time.Now().UnixNano()
	_, err = di.update()
	if err != nil {
		return err
	}
	return nil
}

func newDeviceInfoPI(dbm *gorp.DbMap, pi *MailPingInformation, logger *logging.Logger) (*DeviceInfo, error) {
	var err error
	di, err := getDeviceInfo(dbm, pi.ClientId, pi.ClientContext, pi.DeviceId, logger)
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
		di.logger.Warning("%s: Resetting Token ('%s') and AWSEndpointArn ('%s')", di.getLogPrefix(), di.PushToken, di.AWSEndpointArn)
		di.PushService = pushService
		di.PushToken = ""
		di.AWSEndpointArn = ""
		changed = true
	}
	if di.PushToken != pushToken {
		di.logger.Warning("%s: Resetting AWSEndpointArn ('%s')", di.getLogPrefix(), di.AWSEndpointArn)
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
	err := dbm.Insert(di)
	if err != nil {
		panic(fmt.Sprintf("%s: insert error: %s", di.getLogPrefix(), err.Error()))
	}
	return nil
}

type PingerNotification string

const (
	PingerNotificationRegister PingerNotification = "register"
	PingerNotificationNewMail  PingerNotification = "new"
)

func (di *DeviceInfo) pushMessage(message PingerNotification, ttl int64) (string, error) {
	if message == "" {
		return "", fmt.Errorf("Message can not be empty")
	}
	pingerMap := map[string]interface{}{}
	pingerMap["pinger"] = map[string]string{di.ClientContext: string(message)}
	pingerJson, err := json.Marshal(pingerMap)
	if err != nil {
		return "", err
	}
	hash := sha1.New()
	hash.Write(pingerJson)
	md := hash.Sum(nil)
	pingerMapSha := hex.EncodeToString(md)

	notificationMap := map[string]string{}
	notificationMap["default"] = string(pingerJson)

	APNSMap := map[string]interface{}{}
	APNSMap["pinger"] = pingerMap["pinger"]
	APNSMap["aps"] = map[string]interface{}{"content-available": 1, "sound": "silent.wav"}
	b, err := json.Marshal(APNSMap)
	if err != nil {
		return "", err
	}
	notificationMap["APNS"] = string(b)
	notificationMap["APNS_SANDBOX"] = string(b)

	GCMMap := map[string]interface{}{}
	GCMMap["data"] = pingerMap
	GCMMap["collapse_key"] = string(pingerMapSha)
	GCMMap["time_to_live"] = ttl
	GCMMap["delay_while_idle"] = false

	b, err = json.Marshal(GCMMap)
	if err != nil {
		return "", err
	}
	notificationMap["GCM"] = string(b)

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
	if di.Enabled == false {
		return errors.New("Endpoint is disabled. Can not push.")
	}
	if di.AWSEndpointArn == "" {
		return fmt.Errorf("Endpoint not registered: Token ('%s:%s')", di.PushService, di.PushToken)
	}
	var days_28 int64 = 2419200
	pushMessage, err := di.pushMessage(message, days_28)
	if err != nil {
		return err
	}
	di.logger.Debug("%s: Sending push message to AWS: %s", di.getLogPrefix(), pushMessage)
	err = DefaultPollingContext.config.Aws.sendPushNotification(di.AWSEndpointArn, pushMessage)
	if err == nil {
		di.LastContactRequest = time.Now().UnixNano()
		_, err = di.update()
	}
	return err
}

func (di *DeviceInfo) registerAws() error {
	if di.AWSEndpointArn != "" {
		panic("No need to call register again. Call validate")
	}
	var pushToken string
	var err error
	switch {
	case di.PushService == PushServiceAPNS:
		pushToken, err = decodeAPNSPushToken(di.PushToken)
		if err != nil {
			return err
		}
		if len(pushToken) != 64 {
			return fmt.Errorf("APNS token length wrong. %d ('%s')", len(pushToken), string(pushToken))
		}

	default:
		return fmt.Errorf("Unsupported push service %s:%s", di.PushService, di.PushToken)
	}

	arn, registerErr := DefaultPollingContext.config.Aws.registerEndpointArn(di.PushService, pushToken, di.ClientId)
	if registerErr != nil {
		re, err := regexp.Compile("^.*Endpoint (?P<arn>arn:aws:sns:[^ ]+) already exists.*$")
		if err != nil {
			return err
		}
		if re.MatchString(registerErr.Error()) == true {
			arn := re.ReplaceAllString(registerErr.Error(), fmt.Sprintf("${%s}", re.SubexpNames()[1]))
			di.logger.Warning("%s: Previously registered as %s. Updating.", di.getLogPrefix(), arn)
			attributes, err := DefaultPollingContext.config.Aws.getEndpointAttributes(arn)
			if err != nil {
				return err
			}
			attributes["CustomUserData"] = di.ClientId
			err = DefaultPollingContext.config.Aws.setEndpointAttributes(arn, attributes)
			if err != nil {
				return err
			}
		} else {
			return registerErr
		}
	}
	di.AWSEndpointArn = arn
	di.Enabled = true
	_, err = di.update()
	if err != nil {
		return err
	}
	return nil
}

func (di *DeviceInfo) validateAws() error {
	if di.AWSEndpointArn == "" {
		panic("Can't call validate before register")
	}
	attributes, err := DefaultPollingContext.config.Aws.validateEndpointArn(di.AWSEndpointArn)
	if err != nil {
		return err
	}
	need_update := false
	enabled, ok := attributes["Enabled"]
	if !ok || enabled != "true" {
		if enabled != "true" {
			// Only disable this if we get an actual indication thereof
			di.Enabled = false
			need_update = true
		}
		return errors.New("Endpoint is not enabled")
	}
	if di.Enabled == false {
		// re-enable the endpoint
		di.Enabled = true
		need_update = true
	}

	var pushToken string
	switch {
	case di.PushService == PushServiceAPNS:
		pushToken, err = decodeAPNSPushToken(di.PushToken)
		if err != nil {
			return err
		}
		if len(pushToken) != 64 {
			return fmt.Errorf("APNS token length wrong. %d ('%s')", len(pushToken), string(pushToken))
		}

	default:
		return fmt.Errorf("Unsupported push service %s:%s", di.PushService, di.PushToken)
	}

	if pushToken != attributes["Token"] {
		// need to update the token with aws
		attributes["Token"] = pushToken
		err := DefaultPollingContext.config.Aws.setEndpointAttributes(di.AWSEndpointArn, attributes)
		if err != nil {
			return err
		}
	}
	if need_update {
		di.update()
	}
	return nil
}

func (di *DeviceInfo) validateClient() error {
	// TODO Can we cache the validation results here? Can they change once a client ID has been invalidated? How do we even invalidate one?
	if di.AWSEndpointArn == "" {
		di.logger.Debug("%s: Registering %s:%s with AWS.", di.getLogPrefix(), di.PushService, di.PushToken)
		err := di.registerAws()
		if err != nil {
			if DefaultPollingContext.config.Global.IgnorePushFailure == false {
				return err
			} else {
				di.logger.Warning("%s: Registering %s:%s error (ignored): %s", di.getLogPrefix(), di.PushService, di.PushToken, err.Error())
			}
		} else {
			di.logger.Debug("%s: endpoint created %s", di.getLogPrefix(), di.AWSEndpointArn)
		}
		if di.AWSEndpointArn == "" {
			di.logger.Error("%s: AWSEndpointArn empty after register!", di.getLogPrefix())
		}
		// TODO We should send a test-ping here, so we don't find out the endpoint is unreachable later.
		// It's optional (we'll find out eventually), but this would speed it up.
	} else {
		// Validate this even if the device is marked as deviceInfo.Enabled=false, because this might
		// mark it as enabled again. Possibly...
		err := di.validateAws()
		if err != nil {
			if DefaultPollingContext.config.Global.IgnorePushFailure == false {
				return err
			} else {
				di.logger.Warning("%s: Validating %s:%s error (ignored): %s", di.getLogPrefix(), di.PushService, di.PushToken, err.Error())
			}
		} else {
			di.logger.Debug("%s: endpoint validated %s", di.getLogPrefix(), di.AWSEndpointArn)
		}
	}
	return nil
}

func (di *DeviceInfo) getLogPrefix() string {
	if di.logPrefix == "" {
		di.logPrefix = fmt.Sprintf("%s:%s:%s", di.DeviceId, di.ClientId, di.ClientContext)
	}
	return di.logPrefix
}
