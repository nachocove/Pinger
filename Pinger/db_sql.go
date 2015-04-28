package Pinger

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/Go-SQL-Driver/MySQL" // blank import to get the mysql mappings for gorp
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3" // blank import to get the mysql mappings for gorp
	"github.com/nachocove/Pinger/Utils/AWS"
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
	DebugSql    bool
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

func (dbconfig *DBConfiguration) initDB(init bool, logger *Logging.Logger) (*gorp.DbMap, error) {
	var dbmap *gorp.DbMap
	err := dbconfig.Validate()
	if err != nil {
		return nil, err
	}

	switch {
	case dbconfig.Type == "mysql":
		dbmap, err = dbconfig.initDbMySql()

	case dbconfig.Type == "sqlite" || dbconfig.Type == "sqlite3":
		dbmap, err = dbconfig.initDbSqlite()

	default:
		return nil, fmt.Errorf("Unknown db type %s", dbconfig.Type)
	}
	if err != nil {
		return nil, err
	}
	if dbmap == nil {
		return nil, errors.New("Could not get dbmap")
	}

	if dbconfig.DebugSql {
		l := &DBLogger{logger: logger.Copy()}
		l.logger.SetCallDepth(6)
		dbmap.TraceOn("[gorp]", l)
	}

	///////////////
	// map tables
	///////////////
	addDeviceInfoTable(dbmap)
	addPingerInfoTable(dbmap)

	if init {
		// create the tables. in a production system you'd generally
		// use a migration tool, or create the tables via scripts
		err := dbmap.CreateTablesIfNotExists()
		if err != nil {
			panic(err)
		}
	}

	return dbmap, nil
}

func (dbconfig *DBConfiguration) initDbSqlite() (*gorp.DbMap, error) {
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

func (dbConfig *DBConfiguration) initDbMySql() (*gorp.DbMap, error) {
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

type DBHandleSql struct {
	DBHandler
	dbm *gorp.DbMap
}

func newDBHandleSql(dbm *gorp.DbMap) DBHandler {
	return &DBHandleSql{dbm: dbm}
}

func (h *DBHandleSql) insert(i interface{}, tableName string) error {
	return h.dbm.Insert(i)
}

func (h *DBHandleSql) update(i interface{}, tableName string) (int64, error) {
	n, err := h.dbm.Update(i)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (h *DBHandleSql) delete(i interface{}, tableName string, keys []AWS.DBKeyValue) (int64, error) {
	return h.dbm.Delete(i)
}

func (h *DBHandleSql) get(i interface{}, tableName string, keys []AWS.DBKeyValue) (interface{}, error) {
	args := make([]interface{}, 0, len(keys))
	for _, a := range keys {
		if a.Comparison != AWS.KeyComparisonEq {
			panic("Can only use KeyComparisonEq for get")
		}
		args = append(args, a.Value)
	}
	return h.dbm.Get(i, args...)
}

func (h *DBHandleSql) search(i interface{}, tableName, indexName string, keys []AWS.DBKeyValue) ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *DBHandleSql) selectItems(i interface{}, sql string, args ...interface{}) ([]interface{}, error) {
	return h.dbm.Select(i, sql, args...)
}

func (h *DBHandleSql) initDb() error {
	return nil
}
