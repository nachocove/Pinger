package Telemetry

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	"reflect"
	"time"
)

const telemetryTableName string = "log"

func (writer *TelemetryWriter) initDb() error {
	dbFile := fmt.Sprintf("%s/%s", writer.fileLocationPrefix, "telemetry.db")
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to open sqlite3 DB: %s\n%v", dbFile, err))
	}
	err = db.Ping()
	if err != nil {
		panic(fmt.Sprintf("Could not ping database: %v", err))
	}
	writer.dbmap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}

	tMap := writer.dbmap.AddTableWithName(telemetryMsg{}, telemetryTableName)
	if tMap.SetKeys(false, "Id") == nil {
		panic(fmt.Sprintf("Could not create key on %s:ID", telemetryTableName))
	}
	cMap := tMap.ColMap("EventType")
	cMap.SetNotNull(true)
	
	err = writer.dbmap.CreateTablesIfNotExists()
	if err != nil {
		panic(fmt.Sprintf("Create tables failed: %s", err))
	}
	return nil
}

func (t *telemetryEventType) Scan(value interface{}) error {
	*t = telemetryEventType(string(value.([]uint8)))
	return nil
}
func (t telemetryEventType) Value() (driver.Value, error) {
	return string(t), nil
}

var getAllMessagesSQLwithType string
var getAllMessagesSQL string

func init() {
	var telemetryMsgReflection reflect.Type
	var timestampField reflect.StructField
	var eventTypeField reflect.StructField
	var ok bool
	telemetryMsgReflection = reflect.TypeOf(telemetryMsg{})
	timestampField, ok = telemetryMsgReflection.FieldByName("Timestamp")
	if ok == false {
		panic("Could not get Timestamp Field information")
	}
	eventTypeField, ok = telemetryMsgReflection.FieldByName("EventType")
	if ok == false {
		panic("Could not get EventType Field information")
	}
	getAllMessagesSQLwithType = fmt.Sprintf("select * from %s where %s=? and %s>?", telemetryTableName, eventTypeField.Tag.Get("db"), timestampField.Tag.Get("db"))
	getAllMessagesSQL = fmt.Sprintf("select * from %s where %s>?", telemetryTableName, timestampField.Tag.Get("db"))
}

func (writer *TelemetryWriter) getAllMessagesSince(t time.Time, eventType telemetryEventType) ([]telemetryMsg, error) {
	var messages []telemetryMsg
	var err error
	if eventType == telemetryEventAll {
		_, err = writer.dbmap.Select(&messages, getAllMessagesSQL, t)
	} else {
		_, err = writer.dbmap.Select(&messages, getAllMessagesSQLwithType, eventType, t)
	}
	if err != nil {
		return nil, err
	}
	return messages, nil
}
