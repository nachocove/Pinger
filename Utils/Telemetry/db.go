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

const TelemetryTableName string = "log"

func (writer *TelemetryWriter) initDb() error {
	dbFile := fmt.Sprintf("%s/%s", writer.fileLocationPrefix, "telemetry.db")
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return fmt.Errorf("Failed to open sqlite3 DB: %s\n%v", dbFile, err)
	}
	err = db.Ping()
	if err != nil {
		panic(fmt.Sprintf("Could not ping database: %v", err))
	}
	writer.dbmap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}

	tMap := writer.dbmap.AddTableWithName(TelemetryMsg{}, TelemetryTableName)
	if tMap.SetKeys(false, "Id") == nil {
		panic(fmt.Sprintf("Could not create key on %s:ID", TelemetryTableName))
	}

	err = writer.dbmap.CreateTablesIfNotExists()
	if err != nil {
		panic(fmt.Sprintf("Create tables failed: %s", err))
	}
	return nil
}

func (t *TelemetryEventType) Scan(value interface{}) error {
	*t = TelemetryEventType(string(value.([]uint8)))
	return nil
}
func (t TelemetryEventType) Value() (driver.Value, error) {
	return string(t), nil
}

var getAllMessagesSQL string

func init() {
	var telemetryMsgReflection reflect.Type
	var timestampField reflect.StructField
	var eventTypeField reflect.StructField
	var ok bool
	telemetryMsgReflection = reflect.TypeOf(TelemetryMsg{})
	timestampField, ok = telemetryMsgReflection.FieldByName("Timestamp")
	if ok == false {
		panic("Could not get Timestamp Field information")
	}
	eventTypeField, ok = telemetryMsgReflection.FieldByName("EventType")
	if ok == false {
		panic("Could not get EventType Field information")
	}
	getAllMessagesSQL = fmt.Sprintf("select * from %s where %s=? and %s>?", TelemetryTableName, eventTypeField.Tag.Get("db"), timestampField.Tag.Get("db"))
}

func (writer *TelemetryWriter) getAllMessagesSince(eventType TelemetryEventType, t time.Time) ([]TelemetryMsg, error) {
	var messages []TelemetryMsg
	_, err := writer.dbmap.Select(&messages, getAllMessagesSQL, string(eventType), t)
	if err != nil {
		return nil, err
	}
	return messages, nil
}
