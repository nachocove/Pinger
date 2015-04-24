package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
)

type PingerInfoDynamoDbHandler struct {
	PingerInfoDbHandler
	dynamo    *AWS.DynamoDb
	tableName string
}

const (
	dynamoPingerInfoTableName = "alpha.pinger.pinger_info"
	dynamoPingerInfoUpdatedIndexName = "index-updated"
)

func newPingerInfoDynamoDbHandler(aws AWS.AWSHandler) (*PingerInfoDynamoDbHandler, error) {
	dynamo := aws.GetDynamoDbSession()
	table, err := dynamo.DescribeTable(dynamoPingerInfoTableName)
	if err != nil {
		return nil, err
	}
	if table == nil {
		createReq := dynamo.CreateTableReq(dynamoPingerInfoTableName,
			[]AWS.DBAttrDefinition{
				{Name: piPingerField.Tag.Get("dynamo"), Type: AWS.String},
				{Name: piCreatedField.Tag.Get("dynamo"), Type: AWS.Number},
				{Name: piUpdatedField.Tag.Get("dynamo"), Type: AWS.Number},
			},
			[]AWS.DBKeyType{
				{Name: piPingerField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
			},
			AWS.ThroughPut{Read: 10, Write: 10},
		)
		
		err = dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoPingerInfoUpdatedIndexName,
			[]AWS.DBKeyType{
				{Name: piUpdatedField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
			},
			AWS.ThroughPut{Read: 10, Write: 10},			
		)
		err := dynamo.CreateTable(createReq)
		if err != nil {
			return nil, err
		}
	}
	return &PingerInfoDynamoDbHandler{
		dynamo:    dynamo,
		tableName: dynamoPingerInfoTableName,
	}, nil
}

func (h *PingerInfoDynamoDbHandler) get(keys []AWS.DBKeyValue) (*PingerInfo, error) {
	dcMap, err := h.dynamo.Get(dynamoPingerInfoTableName, keys)
	if err != nil {
		return nil, err
	}
	return fromPingerInfoMap(dcMap), nil
}

func fromPingerInfoMap(dcMap *map[string]interface{}) *PingerInfo {
	pinger := PingerInfo{}
	for k, v := range *dcMap {
		switch v := v.(type) {
		case string:
			switch k {
			case piPingerField.Tag.Get("dynamo"):
				pinger.Pinger = v
			}
		case int64:
			switch k {
			case piCreatedField.Tag.Get("dynamo"):
				pinger.Created = v

			case piUpdatedField.Tag.Get("dynamo"):
				pinger.Updated = v
			}
		case int:
			switch k {
			case piCreatedField.Tag.Get("dynamo"):
				pinger.Created = int64(v)

			case piUpdatedField.Tag.Get("dynamo"):
				pinger.Updated = int64(v)
			}
		}
	}
	return &pinger
}

func (pinger *PingerInfo) toMap() map[string]interface{} {
	pingerMap := make(map[string]interface{})
	v := reflect.Indirect(reflect.ValueOf(pinger))
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		k := t.Field(i).Tag.Get("dynamo")
		if k != "" && k != "-" {
			pingerMap[k] = v.Field(i).Interface()
		}
	}
	return pingerMap
}

func (h *PingerInfoDynamoDbHandler) insert(pinger *PingerInfo) error {
	return h.dynamo.Insert(dynamoPingerInfoTableName, pinger.toMap())
}

func (h *PingerInfoDynamoDbHandler) update(pinger *PingerInfo) (int64, error) {
	err := h.dynamo.Update(dynamoPingerInfoTableName, pinger.toMap())
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (h *PingerInfoDynamoDbHandler) delete(pinger *PingerInfo) (int64, error) {
	err := fmt.Errorf("Not implemented")
	return 0, err

}
