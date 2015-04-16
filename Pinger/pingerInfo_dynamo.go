package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"time"
)

type PingerInfoDynamoDbHandler struct {
	PingerInfoDbHandler
	dynamo    *AWS.DynamoDb
	tableName string
}

const (
	dynamoPingerInfoTableName = "alpha.pinger.pinger_info"
)

func newPingerInfoDynamoDbHandler(aws AWS.AWSHandler) *PingerInfoDynamoDbHandler {
	return &PingerInfoDynamoDbHandler{
		dynamo:    aws.GetDynamoDbSession(),
		tableName: dynamoPingerInfoTableName,
	}
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
			case "pinger":
				pinger.Pinger = v
			}
		case int64:
			switch k {
			case "created":
				pinger.Created = v

			case "updated":
				pinger.Updated = v

			case "id":
				pinger.Id = v
			}
		case int:
			switch k {
			case "created":
				pinger.Created = int64(v)

			case "updated":
				pinger.Updated = int64(v)

			case "id":
				pinger.Id = int64(v)
			}
		}
	}
	return &pinger
}

func (pinger *PingerInfo) toMap() map[string]interface{} {
	pingerMap := make(map[string]interface{})
	pingerMap["id"] = pinger.Id
	pingerMap["pinger"] = pinger.Pinger
	pingerMap["created"] = pinger.Created
	pingerMap["updated"] = pinger.UpdateEntry
	return pingerMap
}

func (h *PingerInfoDynamoDbHandler) insert(pinger *PingerInfo) error {
	pinger.Created = time.Now().UnixNano()
	pinger.Updated = pinger.Created
	_, err := h.dynamo.Insert(dynamoPingerInfoTableName, pinger.toMap())
	return err
}

func (h *PingerInfoDynamoDbHandler) update(pinger *PingerInfo) (int64, error) {
	pinger.Updated = time.Now().UnixNano()
	err := h.dynamo.Update(dynamoPingerInfoTableName, pinger.toMap())
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (h *PingerInfoDynamoDbHandler) delete(pinger *PingerInfo) error {
	err := fmt.Errorf("Not implemented")
	return err

}
