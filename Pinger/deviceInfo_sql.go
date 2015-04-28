package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
)

type DeviceInfoDbHandleSql struct {
	db *DBHandleSql
}

func newDeviceInfoSqlHandler(db DBHandler) DeviceInfoDbHandler {
	return &DeviceInfoDbHandleSql{db.(*DBHandleSql)}
}

func (h *DeviceInfoDbHandleSql) createTable() error {
	return nil
}

func (h *DeviceInfoDbHandleSql) insert(di *DeviceInfo) error {
	return h.db.insert(di, "")
}

func (h *DeviceInfoDbHandleSql) update(di *DeviceInfo) (int64, error) {
	return h.db.update(di, "")
}

func (h *DeviceInfoDbHandleSql) delete(di *DeviceInfo) (int64, error) {
	return h.db.delete(di, "", nil)
}

func (h *DeviceInfoDbHandleSql) get(keys []AWS.DBKeyValue) (*DeviceInfo, error) {
	obj, err := h.db.get(&DeviceInfo{}, "", keys)
	if err != nil {
		return nil, err
	}
	var di *DeviceInfo
	if obj != nil {
		di = obj.(*DeviceInfo)
		di.dbHandler = h
	}
	return di, nil
}

func (h *DeviceInfoDbHandleSql) distinctPushServiceTokens(pingerHostId string) ([]DeviceInfo, error) {
	servicesAndTokens := make([]DeviceInfo, 0, 100)
	_, err := h.db.dbm.Select(&servicesAndTokens, distinctPushServiceTokenSql, pingerHostId)
	if err != nil {
		return nil, err
	}
	return servicesAndTokens, nil
}

func (h *DeviceInfoDbHandleSql) clientContexts(pushservice, pushToken string) ([]string, error) {
	contexts := make([]string, 0, 5)
	_, err := h.db.dbm.Select(&contexts, clientContextsSql, pushservice, pushToken)
	if err != nil {
		return nil, err
	}
	return contexts, nil
}

func (h *DeviceInfoDbHandleSql) getAllMyDeviceInfo(pingerHostId string) ([]DeviceInfo, error) {
	deviceList := make([]DeviceInfo, 0, 100)
	_, err := h.db.dbm.Select(&deviceList, getAllMyDeviceInfoSql, pingerHostId)
	if err != nil {
		return nil, err
	}

	return deviceList, nil
}

const (
	deviceTableName string = "device_info"
)

func addDeviceInfoTable(dbmap *gorp.DbMap) {
	tMap := dbmap.AddTableWithName(DeviceInfo{}, deviceTableName)
	if tMap.SetKeys(false, "ClientId", "ClientContext", "DeviceId", "SessionId") == nil {
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

	cMap = tMap.ColMap("PushToken")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("PushService")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Pinger")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("SessionId")
	cMap.SetNotNull(true)

	createDeviceInfoSqlStatements(dbmap.Dialect)
}

var getAllMyDeviceInfoSql string
var distinctPushServiceTokenSql string
var clientContextsSql string

func createDeviceInfoSqlStatements(dialect gorp.Dialect) {
	_, isSqlite := dialect.(gorp.SqliteDialect)
	_, isMysql := dialect.(gorp.MySQLDialect)
	_, isPostgres := dialect.(gorp.PostgresDialect)
	switch {
	case isSqlite || isMysql:
		getAllMyDeviceInfoSql = fmt.Sprintf("select * from %s where %s=$1",
			deviceTableName,
			diPingerField.Tag.Get("db"))
		distinctPushServiceTokenSql = fmt.Sprintf("select distinct %s, %s from %s where %s=$1",
			diPushServiceField.Tag.Get("db"), diPushTokenField.Tag.Get("db"),
			deviceTableName,
			diPingerField.Tag.Get("db"),
		)
		clientContextsSql = fmt.Sprintf("select distinct %s from %s where %s=$1 and %s=$2",
			diClientContextField.Tag.Get("db"), deviceTableName, diPushServiceField.Tag.Get("db"), diPushTokenField.Tag.Get("db"))

	case isPostgres:
		getAllMyDeviceInfoSql = fmt.Sprintf("select * from %s where %s=$1",
			deviceTableName,
			diPingerField.Tag.Get("db"))
		distinctPushServiceTokenSql = fmt.Sprintf("select distinct %s, %s from %s where %s=$1",
			diPushServiceField.Tag.Get("db"), diPushTokenField.Tag.Get("db"),
			deviceTableName,
			diPingerField.Tag.Get("db"),
		)
		clientContextsSql = fmt.Sprintf("select distinct %s from %s where %s=$1 and %s=$2",
			diClientContextField.Tag.Get("db"), deviceTableName, diPushServiceField.Tag.Get("db"), diPushTokenField.Tag.Get("db"))

	default:
		panic("Unknown db dialect")
	}
}
