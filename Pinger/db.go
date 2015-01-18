package Pinger

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
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
	ClientId       string `db:"client_id"` // us-east-1a-XXXXXXXX
	DeviceId       string `db:"device_id"` // "NchoXXXXXX"
	AWSPushToken   string `db:"aws_push_token"`
	Platform       string `db:"device_platform"` // "ios", "android", etc..
	MailClientType string `db:"mail_client_type"`
	Info           string `db:"info"` // free-form attr/value field for platform-push specific values, i.e. APNS Token, Topic, etc..
}

const (
	DeviceTableName string = "DeviceInfo"
)

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
	if di.Info == "" {
		di.Info = "{}"
	}
	return nil
}
func NewDeviceInfo(clientID, deviceID, pushToken, platform, mailClientType string, info map[string]string) (*DeviceInfo, error) {
	infoString, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	di := &DeviceInfo{
		Created:        time.Now().Unix(),
		Updated:        time.Now().Unix(),
		ClientId:       clientID,
		DeviceId:       deviceID,
		AWSPushToken:   pushToken,
		Platform:       platform,
		Info:           string(infoString),
		MailClientType: mailClientType,
	}
	err = di.Validate()
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

func (di *DeviceInfo) Update(dbm *gorp.DbMap) (int64, error) {
	err := di.Validate()
	if err != nil {
		return 0, err
	}
	di.Updated = time.Now().Unix()
	return dbm.Update(di)
}

func (di *DeviceInfo) Insert(dbm *gorp.DbMap) error {
	err := di.Validate()
	if err != nil {
		return err
	}
	return dbm.Insert(di)
}

func IOSDeviceInfoMap(topic, pushToken, resetToken string) map[string]string {
	info := make(map[string]string)
	info["Topic"] = topic
	info["PushToken"] = pushToken
	info["ResetToken"] = resetToken
	return info
}

func NewDeviceInfoIOS(clientId, deviceID, pushToken, topic, resetToken, mailClientType string) (*DeviceInfo, error) {
	return NewDeviceInfo(
		clientId,
		deviceID,
		pushToken,
		"ios",
		mailClientType,
		IOSDeviceInfoMap(topic, pushToken, resetToken))
}

type EncryptedData []byte

func (enc EncryptedData) Decrypt(encryptionKey []byte) string {
	// TODO Need to decrypt this.
	s := strings.Split(string(enc), ":")
	return string(s[1])
}

func NewEncryptedData(data string, encryptionKey []byte) (EncryptedData, error) {
	// TODO Need to encrypt!
	return EncryptedData(fmt.Sprintf("enc:%s", data)), nil
}

type IOSAPNSInfo struct {
	Id          int64         `db:"id"`
	Created     int64         `db:"created"`
	Updated     int64         `db:"updated"`
	Topic       string        `db:"topic"`
	Certificate string        `db:"certificate"`
	Key         EncryptedData `db:"key"`
}

func InitDB(dbconfig *DBConfiguration, init bool) *gorp.DbMap {
	var dbmap *gorp.DbMap

	switch {
	case dbconfig.Type == "mysql":
		dbmap = initDbMySql(dbconfig)

	case dbconfig.Type == "sqlite":
		//	case dbconfig.Type == "sqlite3":
		dbmap = initDbSqlite(dbconfig)

	default:
		log.Fatalf("Unknown db type %s", dbconfig.Type)
	}

	if dbmap == nil {
		log.Fatalf("Could not get dbmap")
	}

	tMap := dbmap.AddTableWithName(DeviceInfo{}, DeviceTableName)
	if tMap.SetKeys(true, "Id") == nil {
		log.Fatalf("Could not create key on DeviceInfo:ID")
	}

	tMap = dbmap.AddTable(IOSAPNSInfo{})
	if tMap.SetKeys(true, "Id") == nil {
		log.Fatalf("Could not create key on IOSAPNSInfo:ID")
	}
	log.Printf("Finished setting Keys")

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
	//err := db.Ping()
	//if err != nil {
	//	log.Fatalf("Could not ping database: %v", err)
	//}
	return &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
}

const (
	mysqlDBInitString string = "%s:%s@%s:%d/%s"
)

func initDbMySql(dbConfig *DBConfiguration) *gorp.DbMap {
	// connect to db using standard Go database/sql API
	db, err := sql.Open("mysql", fmt.Sprintf(mysqlDBInitString, dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name))
	if err != nil {
		// DO NOT LOG THE PASSWORD!
		log.Fatalf("Failed to open DB: %s\n", fmt.Sprintf(mysqlDBInitString, dbConfig.Username, "XXXXXXX", dbConfig.Host, dbConfig.Port, dbConfig.Name))
	}
	return &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
}

//func init() {
//	// initialize the DbMap
//	dbmap := initDbMySql()
//	defer dbmap.Db.Close()
//}
