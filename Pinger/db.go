package Pinger

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/Go-SQL-Driver/MySQL"
	_ "github.com/mattn/go-sqlite3"

	"github.com/coopernurse/gorp"
	"github.com/op/go-logging"
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

func (dbconfig *DBConfiguration) Validate() error {
	switch {
	case dbconfig.Type == "mysql":
		if dbconfig.Username == "" || dbconfig.Password == "" || dbconfig.Host == "" ||
			dbconfig.Port == 0 || dbconfig.Name == "" {
			return errors.New("Missing parameters for mysql. All are required: Username, Password, Host, Port, Name")
		}
	case dbconfig.Type == "sqlite" || dbconfig.Type == "sqlite3":
		if dbconfig.Filename == "" {
			return errors.New("Empty filename with sqlite")
		}
		break

	default:
		return errors.New(fmt.Sprintf("Unknown/Unsupported db type %s", dbconfig.Type))
	}
	return nil
}

type DBLogger struct {
	logger *logging.Logger
}

func (dbl DBLogger) Printf(format string, v ...interface{}) {
	dbl.logger.Debug(format, v...)
}

func initDB(dbconfig *DBConfiguration, init, debug bool, logger *logging.Logger) (*gorp.DbMap, error) {
	var dbmap *gorp.DbMap
	err := dbconfig.Validate()
	if err != nil {
		return nil, err
	}

	switch {
	case dbconfig.Type == "mysql":
		dbmap = initDbMySql(dbconfig)

	case dbconfig.Type == "sqlite" || dbconfig.Type == "sqlite3":
		dbmap = initDbSqlite(dbconfig)

	default:
		return nil, errors.New(fmt.Sprintf("Unknown db type %s", dbconfig.Type))
	}

	if dbmap == nil {
		return nil, errors.New("Could not get dbmap")
	}

	if debug {
		l := &DBLogger{logger: logger}
		dbmap.TraceOn("[gorp]", l)
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
			return nil, errors.New(fmt.Sprintf("Create tables failed: %s", err))
		}
	}
	return dbmap, nil
}

func initDbSqlite(dbconfig *DBConfiguration) *gorp.DbMap {
	db, err := sql.Open("sqlite3", dbconfig.Filename)
	if err != nil {
		// DO NOT LOG THE PASSWORD!
		log.Fatalf("Failed to open sqlite3 DB: %s\n%v", dbconfig.Filename, err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("Could not ping database: %v", err)
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
