package Pinger

import (
	"database/sql"
	"errors"
	"fmt"
	// blank import to get the mysql mappings for gorp
	_ "github.com/Go-SQL-Driver/MySQL"
	// blank import to get the mysql mappings for gorp
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nachocove/Pinger/Utils/Logging"
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
		return fmt.Errorf("Unknown/Unsupported db type %s", dbconfig.Type)
	}
	return nil
}

type DBLogger struct {
	logger *Logging.Logger
}

func (dbl DBLogger) Printf(format string, v ...interface{}) {
	dbl.logger.Debug(format, v...)
}

func initDB(dbconfig *DBConfiguration, init, debug bool, logger *Logging.Logger) (*gorp.DbMap, error) {
	var dbmap *gorp.DbMap
	err := dbconfig.Validate()
	if err != nil {
		return nil, err
	}

	switch {
	case dbconfig.Type == "mysql":
		dbmap, err = initDbMySql(dbconfig)

	case dbconfig.Type == "sqlite" || dbconfig.Type == "sqlite3":
		dbmap, err = initDbSqlite(dbconfig)

	default:
		return nil, fmt.Errorf("Unknown db type %s", dbconfig.Type)
	}
	if err != nil {
		return nil, err
	}
	if dbmap == nil {
		return nil, errors.New("Could not get dbmap")
	}

	if debug {
		l := &DBLogger{logger: logger.Copy()}
		l.logger.SetCallDepth(6)
		dbmap.TraceOn("[gorp]", l)
	}

	///////////////
	// map tables
	///////////////
	addDeviceInfoTable(dbmap)
	addDeviceContactTable(dbmap)
	addPingerInfoTable(dbmap)

	if init {
		// create the tables. in a production system you'd generally
		// use a migration tool, or create the tables via scripts
		err := dbmap.CreateTablesIfNotExists()
		if err != nil {
			return nil, fmt.Errorf("Create tables failed: %s", err)
		}
	}

	// Add us (if not already there), and start the updater
	pinger, err := newPingerInfo(dbmap)
	if err != nil {
		return nil, err
	}
	go pinger.Updater()

	return dbmap, nil
}

func initDbSqlite(dbconfig *DBConfiguration) (*gorp.DbMap, error) {
	db, err := sql.Open("sqlite3", dbconfig.Filename)
	if err != nil {
		// DO NOT LOG THE PASSWORD!
		return nil, fmt.Errorf("Failed to open sqlite3 DB: %s\n%v", dbconfig.Filename, err)
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("Could not ping database: %v", err)
	}
	return &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}, nil
}

func initDbMySql(dbConfig *DBConfiguration) (*gorp.DbMap, error) {
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
		return nil, fmt.Errorf("Failed to open DB: %s\n", fmt.Sprintf(mysqlDBInitString, dbConfig.Username, "XXXXXXX", dbConfig.Host, dbConfig.Port, dbConfig.Name))
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("Could not ping database: %v", err)
	}
	return &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"}}, nil
}
