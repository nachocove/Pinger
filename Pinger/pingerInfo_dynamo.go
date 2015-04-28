package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
	"strings"
	"fmt"
)

type PingerInfoDbHandleDynamo struct {
	db *DBHandleDynamo
}

func newPingerInfoDbHandleDynamo(db DBHandler) PingerInfoDbHandler {
	return &PingerInfoDbHandleDynamo{db.(*DBHandleDynamo)}
}

const (
	dynamoPingerInfoTableName        = "alpha.pinger.pinger_info"
	dynamoPingerInfoUpdatedIndexName = "index-updated"
)

func (h *PingerInfoDbHandleDynamo) createTable() error {
	_, err := h.db.dynamo.DescribeTable(dynamoPingerInfoTableName)
	if err != nil {
		if !strings.Contains("Cannot do operations on a non-existent table", err.Error()) {
			return err
		}
	} else {
		return nil
	}
	createReq := h.db.dynamo.CreateTableReq(dynamoPingerInfoTableName,
		[]AWS.DBAttrDefinition{
			{Name: piPingerField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: piUpdatedField.Tag.Get("dynamo"), Type: AWS.Number},
		},
		[]AWS.DBKeyType{
			{Name: piPingerField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)

	err = h.db.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoPingerInfoUpdatedIndexName,
		[]AWS.DBKeyType{
			{Name: piPingerField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
			{Name: piUpdatedField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)
	err = h.db.dynamo.CreateTable(createReq)
	if err != nil {
		return err
	}
	return nil
}

func (h *PingerInfoDbHandleDynamo) insert(pinger *PingerInfo) error {
	return h.db.insert(pinger, dynamoPingerInfoTableName)
}

func (h *PingerInfoDbHandleDynamo) update(pinger *PingerInfo) (int64, error) {
	return h.db.update(pinger, dynamoPingerInfoTableName)
}

func (h *PingerInfoDbHandleDynamo) delete(pinger *PingerInfo) (int64, error) {
	keys := []AWS.DBKeyValue{
		AWS.DBKeyValue{Key: "Pinger", Value: pinger.Pinger, Comparison: AWS.KeyComparisonEq},
	}
	return h.db.delete(PingerInfo{}, dynamoPingerInfoTableName, keys)
}

func (h *PingerInfoDbHandleDynamo) get(keys []AWS.DBKeyValue) (*PingerInfo, error) {
	obj, err := h.db.get(&PingerInfo{}, dynamoPingerInfoTableName, keys)
	if err != nil {
		return nil, err
	}
	fmt.Printf("JAN: obj returned as %+v\n", obj)
	var pinger *PingerInfo
	if obj != nil {
		pinger = obj.(*PingerInfo)
		pinger.dbHandler = h
	}
	return pinger, nil
}

func (pi *PingerInfo) ToType(m *map[string]interface{}) (interface{}, error) {
	newPi := PingerInfo{}
	var err error
	errString := make([]string, 0, 0)
	for k, v := range *m {
		switch k {
		case piPingerField.Tag.Get("dynamo"):
			newPi.Pinger = v.(string)
			
		case piUpdatedField.Tag.Get("dynamo"):
			newPi.Updated = v.(int64)
			
		case piCreatedField.Tag.Get("dynamo"):
			newPi.Created = v.(int64)

		default:
			errString = append(errString, fmt.Sprintf("Unhandled key %s", k))
		}
	}
	if len(errString) > 0 {
		err = fmt.Errorf(strings.Join(errString, ", "))
	}
	return &newPi, err
}
