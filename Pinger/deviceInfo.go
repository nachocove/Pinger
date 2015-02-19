package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"regexp"
	"time"

	"github.com/coopernurse/gorp"
)

const (
	PushServiceAPNS = "APNS"
)

type DeviceInfo struct {
	Id                 int64 `db:"id"`
	Created            int64 `db:"created"`
	Updated            int64 `db:"updated"`
	LastContact        int64 `db:"last_contact"`
	LastContactRequest int64 `db:"last_contact_request"`

	ClientId       string `db:"client_id"`       // us-east-1a-XXXXXXXX
	Platform       string `db:"device_platform"` // "ios", "android", etc..
	PushToken      string `db:"push_token"`
	PushService    string `db:"push_service"` // APNS, GCM, ...
	AWSEndpointArn string `db:"aws_endpoint_arn"`
	Enabled        bool   `db:"enabled"`
	Pinger         string `db:"pinger"`

	dbm *gorp.DbMap `db:"-"`
}

const (
	DeviceTableName string = "DeviceInfo"
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
	cMap.SetUnique(true)
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Platform")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("PushToken")
	cMap.SetUnique(true)
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("PushService")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Pinger")
	cMap.SetNotNull(true)

	return nil
}

func (di *DeviceInfo) validate() error {
	if di.ClientId == "" {
		return errors.New("ClientID can not be empty")
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
func newDeviceInfo(clientID, pushToken, pushService, platform string) (*DeviceInfo, error) {
	di := &DeviceInfo{
		ClientId:    clientID,
		PushToken:   pushToken,
		PushService: pushService,
		Platform:    platform,
		Enabled:     false,
	}
	err := di.validate()
	if err != nil {
		return nil, err
	}
	return di, nil
}

var pingerHostId string

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
}

func getDeviceInfo(dbm *gorp.DbMap, clientId string) (*DeviceInfo, error) {
	s := reflect.TypeOf(DeviceInfo{})
	clientIdField, ok := s.FieldByName("ClientId")
	if ok == false {
		return nil, errors.New("Could not get ClientId Field information")
	}
	pingerField, ok := s.FieldByName("Pinger")
	if ok == false {
		return nil, errors.New("Could not get Pinger Field information")
	}
	var devices []DeviceInfo
	var err error
	_, err = dbm.Select(
		&devices,
		fmt.Sprintf("select * from %s where %s=? and %s=?", s.Name(),
			clientIdField.Tag.Get("db"), pingerField.Tag.Get("db")),
		clientId, pingerHostId)
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
		return device, nil

	default:
		return nil, fmt.Errorf("Bad number of rows returned: %d", len(devices))
	}
}

func getAllMyDeviceInfo(dbm *gorp.DbMap) ([]DeviceInfo, error) {
	s := reflect.TypeOf(DeviceInfo{})
	pingerField, ok := s.FieldByName("Pinger")
	if ok == false {
		return nil, errors.New("Could not get Pinger Field information")
	}
	var devices []DeviceInfo
	var err error
	_, err = dbm.Select(
		&devices,
		fmt.Sprintf("select * from %s where %s=?", s.Name(), pingerField.Tag.Get("db")),
		pingerHostId)
	if err != nil {
		return nil, err
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

func (di *DeviceInfo) setDbm(dbm *gorp.DbMap) {
	di.dbm = dbm
}

func updateLastContact(dbm *gorp.DbMap, clientId string) error {
	di, err := getDeviceInfo(dbm, clientId)
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

func newDeviceInfoPI(dbm *gorp.DbMap, pi *MailPingInformation) error {
	var err error
	di, err := getDeviceInfo(dbm, pi.ClientId)
	if err != nil {
		return err
	}
	if di == nil {
		di, err = newDeviceInfo(
			pi.ClientId,
			pi.PushToken,
			pi.PushService,
			pi.Platform)
		if err != nil {
			return err
		}
		if di == nil {
			return errors.New("Could not create DeviceInfo")
		}
		di.dbm = dbm
		err = di.insert()
		if err != nil {
			return err
		}
	} else {
		_, err := di.updateDeviceInfo(pi)
		if err != nil {
			return err
		}
	}
	return nil
}

func (di *DeviceInfo) updateDeviceInfo(pi *MailPingInformation) (bool, error) {
	changed := false
	if di.ClientId != pi.ClientId {
		panic("Can not have a different ClientID")
	}
	if di.Platform != pi.Platform {
		di.Platform = pi.Platform
		changed = true
	}
	if di.PushService != pi.PushService {
		di.PushService = pi.PushService
		changed = true
	}
	if di.PushToken != pi.PushToken {
		di.PushToken = pi.PushToken
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
	return di.dbm.Update(di)
}

func (di *DeviceInfo) insert() error {
	if di.dbm == nil {
		panic("Can not insert device info without having fetched it")
	}
	return di.dbm.Insert(di)
}

func (di *DeviceInfo) push(message string) error {
	var err error
	switch {
	case di.AWSEndpointArn != "":
		err = DefaultPollingContext.config.Aws.sendPushNotification(di.AWSEndpointArn, message)

	case di.Enabled == false:
		err = errors.New("Endpoint is disabled")

	default:
		err = fmt.Errorf("Unsupported push service: %s", di.PushService)
	}
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
	if di.PushService != "APNS" {
		return fmt.Errorf("Unsupported push service %s", di.PushService)
	}
	arn, err := DefaultPollingContext.config.Aws.registerEndpointArn(di.PushService, di.PushToken, di.ClientId)
	if err != nil {
		return err
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
	enabled, ok := attributes["Enabled"]
	if !ok || enabled != "true" {
		if enabled != "true" {
			// Only disable this if we get an actual indication thereof
			di.Enabled = false
			di.update()
		}
		return errors.New("Endpoint is not enabled")
	}
	if di.Enabled == false {
		// re-enable the endpoint
		di.Enabled = true
		di.update()
	}
	return nil
}
