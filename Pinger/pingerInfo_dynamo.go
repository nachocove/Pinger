package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
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

func (h *PingerInfoDbHandleDynamo) createDeviceInfoTable() error {
	table, err := h.db.dynamo.DescribeTable(dynamoPingerInfoTableName)
	if err != nil {
		return err
	}
	if table == nil {
		createReq := h.db.dynamo.CreateTableReq(dynamoPingerInfoTableName,
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

		err = h.db.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoPingerInfoUpdatedIndexName,
			[]AWS.DBKeyType{
				{Name: piUpdatedField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
			},
			AWS.ThroughPut{Read: 10, Write: 10},
		)
		err := h.db.dynamo.CreateTable(createReq)
		if err != nil {
			return err
		}
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
	obj, err := h.db.get(DeviceInfo{}, dynamoPingerInfoTableName, keys)
	if err != nil {
		return nil, err
	}
	var pinger *PingerInfo
	if obj != nil {
		pinger = obj.(*PingerInfo)
		pinger.dbHandler = h
	}
	return pinger, nil
}
