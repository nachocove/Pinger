package Pinger

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"time"

	"github.com/coopernurse/gorp"
	"github.com/nachocove/goamz/aws"
)

type DeviceInfo struct {
	Id             int64  `db:"id"`
	Created        int64  `db:"created"`
	Updated        int64  `db:"updated"`
	ClientId       string `db:"client_id"`       // us-east-1a-XXXXXXXX
	Platform       string `db:"device_platform"` // "ios", "android", etc..
	PushToken      string `db:"push_token"`
	PushService    string `db:"push_service"` // APNS, GCM, ...
	AWSEndpointArn string `db:"aws_endpoint_arn"`

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
			return errors.New(fmt.Sprintf("Platform %s is not known", di.Platform))
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
	}
	err := di.validate()
	if err != nil {
		return nil, err
	}
	return di, nil
}

func getDeviceInfo(dbm *gorp.DbMap, clientId string) (*DeviceInfo, error) {
	s := reflect.TypeOf(DeviceInfo{})
	field, ok := s.FieldByName("ClientId")
	if ok == false {
		return nil, errors.New("Could not get ClientId Field information")
	}
	var devices []DeviceInfo
	var err error
	_, err = dbm.Select(
		&devices,
		fmt.Sprintf("select * from %s where %s=?", s.Name(), field.Tag.Get("db")),
		clientId)
	if err != nil {
		return nil, err
	}
	switch {
	case len(devices) > 1:
		return nil, errors.New(fmt.Sprintf("More than one entry from select: %d", len(devices)))

	case len(devices) == 0:
		return nil, nil

	case len(devices) == 1:
		device := &(devices[0])
		device.dbm = dbm
		return device, nil

	default:
		return nil, errors.New(fmt.Sprintf("Bad number of rows returned: %d", len(devices)))
	}
}

func (di *DeviceInfo) PreUpdate(s gorp.SqlExecutor) error {
	di.Updated = time.Now().UnixNano()
	return di.validate()
}

func (di *DeviceInfo) PreInsert(s gorp.SqlExecutor) error {
	di.Created = time.Now().UnixNano()
	di.Updated = di.Created
	return di.validate()
}

func (di *DeviceInfo) setDbm(dbm *gorp.DbMap) {
	di.dbm = dbm
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
		err = sendPushNotification(di.AWSEndpointArn, message)
		if err != nil {
			awsErr := err.(*aws.Error)
			switch {
			case awsErr.Code == "EndpointDisabled":
				// TODO Need to mark the endpoint as 'off' or whatever
				break

			default:
				break
			}
		}

	default:
		return errors.New(fmt.Sprintf("Unsupported push service: %s", di.PushService))
	}
	return err
}

func (di *DeviceInfo) registerAws() error {
	if di.AWSEndpointArn == "" {
		if di.PushService != "APNS" {
			return errors.New(fmt.Sprintf("Unsupported push service %s", di.PushService))
		}
		arn, err := registerEndpointArn(di.PushService, di.PushToken, di.ClientId)
		if err != nil {
			// TODO process AWS error codes here
			return err
		}
		di.AWSEndpointArn = arn
	} else {
		// TODO get the values here, and update
	}
	return nil
}
