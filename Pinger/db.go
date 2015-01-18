package Pinger

import (
	"database/sql"
	"fmt"
	"log"

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

type deviceInfo struct {
	Id                 int64
	ClientId           string // us-east-1a-XXXXXXXX
	DeviceId           string // "NchoXXXXXX"
	AWSPushToken       string
	DevicePlatform     string // "ios", "android", etc..
	DevicePlatformInfo string // free-form attr/value field for platform-push specific values, i.e. APNS Token, etc..
	MailClientType     string
}

type iOSAPNSInfo struct {
	Id          int64
	Topic       string
	Certificate string
	Key         string
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

	dbmap.AddTable(deviceInfo{}).SetKeys(true, "Id")
	dbmap.AddTable(iOSAPNSInfo{}).SetKeys(true, "Id")

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
