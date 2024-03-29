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
	return fromDeviceInfoMap(dcMap), nil
}

func fromDeviceInfoMap(dcMap *map[string]interface{}) *deviceContact {
	dc := deviceContact{}
	for k, v := range *dcMap {
		switch v := v.(type) {
		case string:
			switch k {
			case "user":
				dc.UserId = v

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
	dcMap["user"] = dc.UserId
	dcMap["context"] = dc.ClientContext
	dcMap["device"] = dc.DeviceId
	dcMap["created"] = dc.Created
	dcMap["updated"] = dc.Updated
	dcMap["last_contact"] = dc.LastContact
	dcMap["last_contact_request"] = dc.LastContactRequest
	return dcMap
}

func (h *DeviceContactDynamoDbHandler) insert(dc *deviceContact) error {
	if dc.Created == 0 {
		dc.Created = time.Now().UnixNano()
	}
	dc.Updated = dc.Created
	dc.LastContact = dc.Created
	return h.dynamo.Insert(dynamoDeviceInfoTableName, dc.toMap())
}

func (h *DeviceContactDynamoDbHandler) update(dc *deviceContact) (int64, error) {
	dc.Updated = time.Now().UnixNano()
	err := h.dynamo.Update(dynamoDeviceInfoTableName, dc.toMap())
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (h *DeviceContactDynamoDbHandler) delete(dc *deviceContact) (int64, error) {
	err := fmt.Errorf("Not implemented")
	return 0, err

}
