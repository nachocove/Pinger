package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"time"
	"reflect"
)

type DeviceContactDynamoDbHandler struct {
	DeviceContactDbHandler
	dynamo    *AWS.DynamoDb
	tableName string
}

const (
	dynamoDeviceContactTableName = "alpha.pinger.device_info"
)

func newDeviceContactDynamoDbHandler(aws AWS.AWSHandler) *DeviceContactDynamoDbHandler {
	return &DeviceContactDynamoDbHandler{
		dynamo:    aws.GetDynamoDbSession(),
		tableName: dynamoDeviceContactTableName,
	}
}

var deviceContactReflection reflect.Type
func init() {
	deviceContactReflection = reflect.TypeOf(deviceContact{})
}

func (h *DeviceContactDynamoDbHandler) get(keys []AWS.DBKeyValue) (*deviceContact, error) {
    // TODO Need to look at the table description and match up passed in keys to the indexes, and decide
    // whether we can get on the primary (no index needed) or one of the indexes. This may not be trivial
	reqKeys := make([]AWS.DBKeyValue, 0, 1)
	for _, k := range keys {
		field, ok := deviceContactReflection.FieldByName(k.Key)
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
	dcMap, err := h.dynamo.Get(dynamoDeviceContactTableName, reqKeys)
	if err != nil {
		return nil, err
	}
	return fromDeviceContactMap(dcMap), nil
}

func (h *DeviceContactDynamoDbHandler) insert(dc *deviceContact) error {
	if dc.Created == 0 {
		dc.Created = time.Now().UnixNano()
	}
	if dc.Id == 0 {
		dc.Id = dc.Created		
	}
	dc.Updated = dc.Created
	dc.LastContact = dc.Created
	dc.Pinger = pingerHostId
	return h.dynamo.Insert(dynamoDeviceContactTableName, dc.toMap())
}

func (h *DeviceContactDynamoDbHandler) update(dc *deviceContact) (int64, error) {
	dc.Updated = time.Now().UnixNano()
	err := h.dynamo.Update(dynamoDeviceContactTableName, dc.toMap())
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (h *DeviceContactDynamoDbHandler) delete(dc *deviceContact) (int64, error) {
    // TODO Need to look at the table description and match up passed in keys to the indexes, and decide
    // whether we can delete on the primary (no index needed) or one of the indexes. This may not be trivial
	if dc.Id == 0 {
		panic("Can not delete item without primary key")
	}
	return h.dynamo.Delete(dynamoDeviceContactTableName,
		[]AWS.DBKeyValue{
			AWS.DBKeyValue{Key: "id",  Value: dc.Id, Comparison: AWS.KeyComparisonEq},
			AWS.DBKeyValue{Key: "client_id",  Value: dc.ClientId, Comparison: AWS.KeyComparisonEq},
			AWS.DBKeyValue{Key: "pinger",  Value: dc.Pinger, Comparison: AWS.KeyComparisonEq},
			})
}

func (h *DeviceContactDynamoDbHandler) findByPingerId(pingerId string) ([]*deviceContact, error) {
	err := fmt.Errorf("Not implemented")
	return nil, err
}

func fromDeviceContactMap(dcMap *map[string]interface{}) *deviceContact {
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
	dcMap["id"] = dc.Id
	dcMap["client"] = dc.ClientId
	dcMap["context"] = dc.ClientContext
	dcMap["device"] = dc.DeviceId
	dcMap["created"] = dc.Created
	dcMap["updated"] = dc.Updated
	dcMap["last_contact"] = dc.LastContact
	dcMap["last_contact_request"] = dc.LastContactRequest
	dcMap["pinger"] = dc.Pinger
	return dcMap
}

