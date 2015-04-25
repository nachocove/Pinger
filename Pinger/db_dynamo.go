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

func (h *DBHandleDynamo) get(i interface{}, tableName string, keys []AWS.DBKeyValue) (interface{}, error) {
	m, err := h.dynamo.Get(tableName, h.withDynamoTags(i, keys))
	if err != nil {
		return nil, err
	}
	return h.toType(i, m)
}

func (h *DBHandleDynamo) search(i interface{}, tableName, indexName string, keys []AWS.DBKeyValue) ([]interface{}, error) {
	mArray, err := h.dynamo.Search(tableName, indexName, h.withDynamoTags(i, keys))
	if err != nil {
		return nil, err
	}
	iArray := make([]interface{}, 0, len(mArray))
	for _, m := range mArray {
		item, err := h.toType(i, &m)
		if err != nil {
			return nil, err
		}
		iArray = append(iArray, item)
	}
	return iArray, nil
}

func (h *DBHandleDynamo) toType(i interface{}, m *map[string]interface{}) (interface{}, error) {
	item := reflect.New(reflect.TypeOf(i))
	vReflect := reflect.ValueOf(item)
	for k, v := range *m {
		for i := 0; i < vReflect.NumField(); i++ {
			if vReflect.Type().Field(i).Tag.Get(k) != "" {
				vv := reflect.ValueOf(v)
				if vReflect.Field(i).Type() != vv.Type() {
					return nil, fmt.Errorf(fmt.Sprintf("Wrong type for field %s. Expected %s, got %s", k, vv.Type(), vReflect.Field(i).Type()))
				}
				vReflect.Field(i).Set(vv)
			}
		}
	}
	return &item, nil

}
func (h *DBHandleDynamo) toMap(i interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	vReflect := reflect.Indirect(reflect.ValueOf(i))
	t := vReflect.Type()
	for i := 0; i < vReflect.NumField(); i++ {
		k := t.Field(i).Tag.Get("dynamo")
		if k != "" && k != "-" {
			switch v := vReflect.Field(i).Interface().(type) {
			case string:
				if v != "" {
					m[k] = v
				} else {
					panic(fmt.Sprintf("Field is empty", k))
				}

			default:
				m[k] = v
			}
		}
	}
	return m
}

func (h *DBHandleDynamo) withDynamoTags(i interface{}, keys []AWS.DBKeyValue) []AWS.DBKeyValue {
	v := reflect.TypeOf(i)
	reqKeys := make([]AWS.DBKeyValue, 0, len(keys))
	for _, k := range keys {
		field, ok := v.FieldByName(k.Key)
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
