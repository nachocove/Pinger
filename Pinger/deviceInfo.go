package Pinger

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"time"

	"github.com/coopernurse/gorp"
)

type DeviceInfo struct {
	Id          int64  `db:"id"`
	Created     int64  `db:"created"`
	Updated     int64  `db:"updated"`
	ClientId    string `db:"client_id"`       // us-east-1a-XXXXXXXX
	Platform    string `db:"device_platform"` // "ios", "android", etc..
	PushToken   string `db:"push_token"`
	PushService string `db:"push_service"` // AWS, APNS, GCM, ...
}

const (
	DeviceTableName string = "DeviceInfo"
)

func addDeviceInfoTable(dbmap *gorp.DbMap) error {
	tMap := dbmap.AddTableWithName(DeviceInfo{}, DeviceTableName)
	if tMap.SetKeys(true, "Id") == nil {
		log.Fatalf("Could not create key on DeviceInfo:ID")
	}
	cMap := tMap.ColMap("ClientId")
	cMap.SetUnique(true)
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("PushToken")
	cMap.SetUnique(true)
	cMap.SetNotNull(true)

	return nil
}

func (di *DeviceInfo) Validate() error {
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
func NewDeviceInfo(clientID, pushToken, pushService, platform string) (*DeviceInfo, error) {
	di := &DeviceInfo{
		ClientId:    clientID,
		PushToken:   pushToken,
		PushService: pushService,
		Platform:    platform,
	}
	err := di.Validate()
	if err != nil {
		return nil, err
	}
	return di, nil
}

func GetDeviceInfo(dbm *gorp.DbMap, clientId string) (*DeviceInfo, error) {
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
		return &(devices[0]), nil

	default:
		return nil, errors.New(fmt.Sprintf("Bad number of rows returned: %d", len(devices)))
	}
}

func (di *DeviceInfo) PreUpdate(s gorp.SqlExecutor) error {
	di.Updated = time.Now().UnixNano()
	return di.Validate()
}

func (di *DeviceInfo) PreInsert(s gorp.SqlExecutor) error {
	di.Created = time.Now().UnixNano()
	di.Updated = di.Created
	return di.Validate()
}
