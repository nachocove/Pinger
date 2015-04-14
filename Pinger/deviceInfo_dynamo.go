package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"time"
)

type DeviceContactDynamoDbHandler struct {
	DeviceContactDbHandler
	dynamo    *AWS.DynamoDb
	tableName string
}

const (
	dynamoDeviceInfoTableName = "alpha.pinger.device_info"
)

func newDeviceContactDynamoDbHandler(aws AWS.AWSHandler) *DeviceContactDynamoDbHandler {
	return &DeviceContactDynamoDbHandler{
		dynamo:    aws.GetDynamoDbSession(),
		tableName: dynamoDeviceInfoTableName,
	}
}

func (h *DeviceContactDynamoDbHandler) get(keys []AWS.DBKeyValue) (*deviceContact, error) {
	dcMap, err := h.dynamo.Get(dynamoDeviceInfoTableName, keys)
	if err != nil {
		return nil, err
	}
	return fromMap(dcMap), nil
}

func fromMap(dcMap *map[string]interface{}) *deviceContact {
	dc := deviceContact{}
	for k, v := range *dcMap {
		switch v := v.(type) {
		case string:
			switch k {
			case "client":
				dc.ClientId = v

			case "device":
				dc.DeviceId = v

			case "context":
				dc.ClientContext = v
			}
		case int64:
			switch k {
			case "created":
				dc.Created = v

			case "updated":
				dc.Updated = v

			case "last_contact":
				dc.LastContact = v

			case "id":
				dc.Id = v
			}
		case int:
			switch k {
			case "created":
				dc.Created = int64(v)

			case "updated":
				dc.Updated = int64(v)

			case "last_contact":
				dc.LastContact = int64(v)

			case "id":
				dc.Id = int64(v)
			}
		}
	}
	return &dc
}
func (dc *deviceContact) toMap() map[string]interface{} {
	dcMap := make(map[string]interface{})
	dcMap["client"] = dc.ClientId
	dcMap["context"] = dc.ClientContext
	dcMap["device"] = dc.DeviceId
	dcMap["created"] = time.Now().UnixNano()
	dcMap["updated"] = dcMap["created"]
	dcMap["last_contact"] = dcMap["created"]
	return dcMap
}

func (h *DeviceContactDynamoDbHandler) insert(dc *deviceContact) error {
	_, err := h.dynamo.Insert(dynamoDeviceInfoTableName, dc.toMap())
	return err
}

func (h *DeviceContactDynamoDbHandler) update(dc *deviceContact) (int64, error) {
	dcMap := make(map[string]interface{})
	dcMap["client"] = dc.ClientId
	dcMap["context"] = dc.ClientContext
	dcMap["device"] = dc.DeviceId
	dcMap["created"] = dc.Created
	dcMap["updated"] = time.Now().UnixNano()
	dcMap["last_contact"] = dc.LastContact

	err := h.dynamo.Update(dynamoDeviceInfoTableName, dcMap)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (h *DeviceContactDynamoDbHandler) delete(dc *deviceContact) error {
	err := fmt.Errorf("Not implemented")
	return err

}
