package Pinger

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"time"

	_ "github.com/Go-SQL-Driver/MySQL"
	_ "github.com/mattn/go-sqlite3"

	"github.com/coopernurse/gorp"
)

type DBConfiguration struct {
	Type        string
	Filename    string // used for sqlite3
	Name        string
	Host        string
	Port        int
	Username    string
	Password    string
	Certificate string // for SSL protected communication with the DB
}

// Tables

type DeviceInfo struct {
	Id             int64  `db:"id"`
	Created        int64  `db:"created"`
	Updated        int64  `db:"updated"`
	ClientId       string `db:"client_id"`       // us-east-1a-XXXXXXXX
	DeviceId       string `db:"device_id"`       // "NchoXXXXXX"
	Platform       string `db:"device_platform"` // "ios", "android", etc..
	PushToken      string `db:"push_token"`
	PushService    string `db:"push_service"` // AWS, APNS, GCM, ...
	MailClientType string `db:"mail_client_type"`
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
			return errors.New(fmt.Sprintf("Platform %s is not known", di.Platform))
		}
	}
	return nil
}
func NewDeviceInfo(clientID, deviceID, pushToken, pushService, platform, mailClientType string) (*DeviceInfo, error) {
	di := &DeviceInfo{
		ClientId:       clientID,
		DeviceId:       deviceID,
		PushToken:      pushToken,
		PushService:    pushService,
		Platform:       platform,
		MailClientType: mailClientType,
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

/////////////////////////////////////////////////////////////////////////////////
func InitDB(dbconfig *DBConfiguration, init bool) *gorp.DbMap {
	var dbmap *gorp.DbMap

	switch {
	case dbconfig.Type == "mysql":
		dbmap = initDbMySql(dbconfig)

	case dbconfig.Type == "sqlite" || dbconfig.Type == "sqlite3":
		dbmap = initDbSqlite(dbconfig)

	default:
		log.Fatalf("Unknown db type %s", dbconfig.Type)
	}

	if dbmap == nil {
		log.Fatalf("Could not get dbmap")
	}

	///////////////
	// map tables
	///////////////
	addDeviceInfoTable(dbmap)

	if init {
		// create the table. in a production system you'd generally
		// use a migration tool, or create the tables via scripts
		err := dbmap.CreateTablesIfNotExists()
		if err != nil {
			log.Fatalf("Create tables failed: %s", err)
		}
	}
	return dbmap
}

func initDbSqlite(dbconfig *DBConfiguration) *gorp.DbMap {
	db, err := sql.Open("sqlite3", dbconfig.Filename)
	if err != nil {
		// DO NOT LOG THE PASSWORD!
		log.Fatalf("Failed to open sqlite3 DB: %s\n%v", dbconfig.Filename, err)
	}
	return &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
}

func initDbMySql(dbConfig *DBConfiguration) *gorp.DbMap {
	//const mysqlDBInitString string = "%s:%s@tcp(%s:%d)/%s/collation=utf8_general_ci&autocommit=true"
	const mysqlDBInitString string = "%s:%s@tcp(%s:%d)/%s"
	// connect to db using standard Go database/sql API
	connectString := fmt.Sprintf(
		mysqlDBInitString,
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Name,
	)
	fmt.Println(connectString)
	db, err := sql.Open("mysql", connectString)
	if err != nil {
		// DO NOT LOG THE PASSWORD!
		log.Fatalf("Failed to open DB: %s\n", fmt.Sprintf(mysqlDBInitString, dbConfig.Username, "XXXXXXX", dbConfig.Host, dbConfig.Port, dbConfig.Name))
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("Could not ping database: %v", err)
	}
	return &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
}
