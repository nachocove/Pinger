package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
	"time"
)

type DeviceInfoSqlHandler struct {
	DeviceInfoDbHandler
	dbm *gorp.DbMap
}

func newDeviceInfoSqlHandler(dbm *gorp.DbMap) *DeviceInfoSqlHandler {
	return &DeviceInfoSqlHandler{dbm: dbm}
}
func (h *DeviceInfoSqlHandler) insert(di *DeviceInfo) error {
	return h.dbm.Insert(di)
}

func (h *DeviceInfoSqlHandler) update(di *DeviceInfo) (int64, error) {
	n, err := h.dbm.Update(di)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (h *DeviceInfoSqlHandler) delete(di *DeviceInfo) (int64, error) {
	return h.dbm.Delete(di)
}

func (h *DeviceInfoSqlHandler) get(keys []AWS.DBKeyValue) (*DeviceInfo, error) {
	args := make([]interface{}, 0, len(keys))
	for _, a := range keys {
		if a.Comparison != AWS.KeyComparisonEq {
			panic("Can only use KeyComparisonEq for get")
		}
		args = append(args, a.Value)
	}
	obj, err := h.dbm.Get(&DeviceInfo{}, args...)
	if err != nil {
		return nil, err
	}
	var di *DeviceInfo
	if obj != nil {
		di = obj.(*DeviceInfo)
		di.db = h
	}
	return di, nil
}

func (h *DeviceInfoSqlHandler) findByPingerId(pingerId string) ([]*DeviceInfo, error) {
	var devices []*DeviceInfo
	var err error
	_, err = h.dbm.Select(&devices, getAllMyDeviceInfoSql, pingerHostId)
	if err != nil {
		return nil, err
	}
	for k := range devices {
		devices[k].db = h
	}
	return devices, nil
}

const (
	deviceTableName string = "device_info"
)

func addDeviceInfoTable(dbmap *gorp.DbMap) {
	tMap := dbmap.AddTableWithName(DeviceInfo{}, deviceTableName)
	if tMap.SetKeys(false, "UserId", "ClientContext", "DeviceId", "SessionId") == nil {
		panic(fmt.Sprintf("Could not create key on %s:ID", deviceTableName))
	}
	tMap.SetVersionCol("Id")

	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("UserId")
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

var getAllMyDeviceInfoSql string
var distinctPushServiceTokenSql string
var clientContextsSql string

func init() {
	var ok bool
	deviceInfoReflection := reflect.TypeOf(DeviceInfo{})
	pingerField, ok := deviceInfoReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	pushServiceField, ok := deviceInfoReflection.FieldByName("PushService")
	if ok == false {
		panic("Could not get PushService Field information")
	}
	pushTokenField, ok := deviceInfoReflection.FieldByName("PushToken")
	if ok == false {
		panic("Could not get PushToken Field information")
	}
	platformField, ok := deviceInfoReflection.FieldByName("Platform")
	if ok == false {
		panic("Could not get Platform Field information")
	}
	OSVersionField, ok := deviceInfoReflection.FieldByName("OSVersion")
	if ok == false {
		panic("Could not get OsVersion Field information")
	}
	awsEndpointField, ok := deviceInfoReflection.FieldByName("AWSEndpointArn")
	if ok == false {
		panic("Could not get AWSEndpointArn Field information")
	}
	clientContextField, ok := deviceInfoReflection.FieldByName("ClientContext")
	if ok == false {
		panic("Could not get ClientContext Field information")
	}
	getAllMyDeviceInfoSql = fmt.Sprintf("select * from %s where %s=?",
		deviceTableName,
		pingerField.Tag.Get("db"))
	distinctPushServiceTokenSql = fmt.Sprintf("select distinct %s, %s, %s, %s, %s from %s where %s=?",
		pushServiceField.Tag.Get("db"), pushTokenField.Tag.Get("db"), OSVersionField.Tag.Get("db"), platformField.Tag.Get("db"), awsEndpointField.Tag.Get("db"),
		deviceTableName,
		pingerField.Tag.Get("db"),
	)
	clientContextsSql = fmt.Sprintf("select distinct %s from %s where %s=? and %s=?",
		clientContextField.Tag.Get("db"), deviceTableName, pushServiceField.Tag.Get("db"), pushTokenField.Tag.Get("db"))
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
