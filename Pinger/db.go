package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"strings"
)

type DBHandlerType int

const (
	DBHandlerSql    DBHandlerType = iota
	DBHandlerDynamo DBHandlerType = iota
)

func (d *DBHandlerType) String() string {
	switch *d {
	case DBHandlerSql:
		return "sql"

	case DBHandlerDynamo:
		return "dynamo"

	default:
		panic("Unknown DBHandlerType")
	}
}
func (d *DBHandlerType) UnmarshalText(text []byte) error {
	switch {
	case strings.EqualFold(strings.ToLower(string(text)), "sql"):
		*d = DBHandlerSql
		return nil
	case strings.EqualFold(strings.ToLower(string(text)), "dynamo"):
		*d = DBHandlerDynamo
		return nil

	default:
		return fmt.Errorf("Unknown DBHandlerType")
	}
}

type DBHandler interface {
	insert(i interface{}, tableName string) error
	update(i interface{}, tableName string) (int64, error)
	delete(i interface{}, tableName string, keys []AWS.DBKeyValue) (int64, error)
	get(i interface{}, tableName string, keys []AWS.DBKeyValue) (interface{}, error)
}

func newDbHandler(db DBHandlerType, dbm *gorp.DbMap, aws AWS.AWSHandler) DBHandler {
	switch db {
	case DBHandlerSql:
		if dbm == nil {
			panic(fmt.Errorf("dbm can not be nil"))
		}
		return newDBHandleSql(dbm)

	case DBHandlerDynamo:
		if aws == nil {
			panic(fmt.Errorf("aws can not be nil"))
		}
		return newDBHandleDynamo(aws.GetDynamoDbSession())

	default:
		panic("Unknown interface type")
	}
}
