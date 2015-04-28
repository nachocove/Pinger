package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
)

type DBHandleDynamo struct {
	DBHandler
	dynamo *AWS.DynamoDb
}

func newDBHandleDynamo(dynamo *AWS.DynamoDb) DBHandler {
	return &DBHandleDynamo{dynamo: dynamo}
}

func (h *DBHandleDynamo) insert(i interface{}, tableName string) error {
	return h.dynamo.Insert(tableName, h.toMap(i))
}

func (h *DBHandleDynamo) update(i interface{}, tableName string) (int64, error) {
	err := h.dynamo.Update(tableName, h.toMap(i))
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (h *DBHandleDynamo) delete(i interface{}, tableName string, keys []AWS.DBKeyValue) (int64, error) {
	return h.dynamo.Delete(tableName, h.withDynamoTags(i, keys))
}

type HasToType interface {
	ToType(m *map[string]interface{}) (interface{}, error)
}

func (h *DBHandleDynamo) get(i interface{}, tableName string, keys []AWS.DBKeyValue) (interface{}, error) {
	ptrv := reflect.ValueOf(i)
	if ptrv.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("Type %#v must be pointer", i))
	}
	elem := ptrv.Elem()
	t, ok := elem.Addr().Interface().(HasToType)
	if !ok {
		panic(fmt.Sprintf("Type %#v must have ToType", i))
	}
	m, err := h.dynamo.Get(tableName, h.withDynamoTags(i, keys))
	if err != nil {
		return nil, err
	}
	if len(*m) == 0 {
		return nil, nil
	}
	return t.ToType(m)
}

func (h *DBHandleDynamo) search(i interface{}, tableName, indexName string, keys []AWS.DBKeyValue) ([]interface{}, error) {
	t, ok := i.(HasToType)
	if !ok {
		panic("Type must have ToType")
	}
	fmt.Printf("JAN searching table %s index %v keys %+v\n", tableName, indexName, h.withDynamoTags(i, keys))
	mArray, err := h.dynamo.Search(tableName, indexName, h.withDynamoTags(i, keys))
	if err != nil {
		return nil, err
	}
	iArray := make([]interface{}, 0, len(mArray))
	for _, m := range mArray {
		item, err := t.ToType(&m)
		if err != nil {
			return nil, err
		}
		iArray = append(iArray, item)
	}
	return iArray, nil
}

func (h *DBHandleDynamo) toMap(i interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	vType, vElem := h.TypeAndElem(i)
	for i := 0; i < vElem.NumField(); i++ {
		k := vType.Field(i).Tag.Get("dynamo")
		if k != "" && k != "-" {
			switch v := vElem.Field(i).Interface().(type) {
			case string:
				if v != "" {
					m[k] = v
				} else {
					panic(fmt.Sprintf("Field %s is empty", k))
				}

			default:
				m[k] = v
			}
		}
	}
	return m
}

func (h *DBHandleDynamo) TypeAndElem(ptr interface{}) (reflect.Type, reflect.Value) {
	ptrv := reflect.ValueOf(ptr)
	if ptrv.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("Dynamo passed non-pointer: %v (kind=%v)", ptr,
			ptrv.Kind()))
	}
	elem := ptrv.Elem()
	etype := reflect.TypeOf(elem.Interface())
	return etype, elem
}

func (h *DBHandleDynamo) withDynamoTags(i interface{}, keys []AWS.DBKeyValue) []AWS.DBKeyValue {
	vType, _ := h.TypeAndElem(i)
	reqKeys := make([]AWS.DBKeyValue, 0, len(keys))
	for _, k := range keys {
		field, ok := vType.FieldByName(k.Key)
		if !ok {
			panic(fmt.Sprintf("No dynamo tag for field %s", k.Key))
		}
		tag := field.Tag.Get("dynamo")
		if tag == "" {
			panic(fmt.Sprintf("Tag for field %s can not be empty (%+v)", k.Key, field.Tag))
		}
		reqKeys = append(reqKeys, AWS.DBKeyValue{Key: tag, Value: k.Value, Comparison: k.Comparison})
	}
	if len(reqKeys) == 0 {
		panic("No keys found to get")
	}
	return reqKeys
}

func (h *DBHandleDynamo) initDb() error {
	dh := newDeviceInfoDynamoDbHandler(h)
	err := dh.createTable()
	if err != nil {
		panic(err)
	}

	ph := newPingerInfoDbHandleDynamo(h)
	err = ph.createTable()
	if err != nil {
		panic(err)
	}
	return nil
}
