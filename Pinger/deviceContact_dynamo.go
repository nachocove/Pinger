package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
	"time"
)

type DeviceContactDynamoDbHandler struct {
	DeviceContactDbHandler
	dynamo    *AWS.DynamoDb
	tableName string
}

const (
	dynamoDeviceContactTableName             = "alpha.pinger.device_info"
	dynamoDeviceContactPingerClientIndexName = "index.pinger-device"
)

func newDeviceContactDynamoDbHandler(aws AWS.AWSHandler) (*DeviceContactDynamoDbHandler, error) {
	return &DeviceContactDynamoDbHandler{
		dynamo:    aws.GetDynamoDbSession(),
		tableName: dynamoDeviceContactTableName,
	}, nil
}

func (h *DeviceContactDynamoDbHandler) createDeviceContactTable() error {
	createReq := h.dynamo.CreateTableReq(dynamoDeviceContactTableName,
		[]AWS.DBAttrDefinition{
			{Name: dcIdField.Tag.Get("dynamo"), Type: AWS.Number},
			{Name: dcPingerField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: dcClientIdField.Tag.Get("dynamo"), Type: AWS.String},
		},
		[]AWS.DBKeyType{
			{Name: dcIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)

	err := h.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoDeviceContactPingerClientIndexName,
		[]AWS.DBKeyType{
			{Name: dcPingerField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
			{Name: dcClientIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)
	if err != nil {
		return err
	}

	err = h.dynamo.CreateTable(createReq)
	if err != nil {
		return err
	}
	return nil
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
	if dc.Id == 0 {
		dc.Id = time.Now().UTC().UnixNano()
	}
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
			AWS.DBKeyValue{Key: dcIdField.Tag.Get("dynamo"), Value: dc.Id, Comparison: AWS.KeyComparisonEq},
			AWS.DBKeyValue{Key: dcClientIdField.Tag.Get("dynamo"), Value: dc.ClientId, Comparison: AWS.KeyComparisonEq},
			AWS.DBKeyValue{Key: dcPingerField.Tag.Get("dynamo"), Value: dc.Pinger, Comparison: AWS.KeyComparisonEq},
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
			case dcClientIdField.Tag.Get("dynamo"):
				dc.ClientId = v

			case dcDeviceIdField.Tag.Get("dynamo"):
				dc.DeviceId = v

			case dcClientContextField.Tag.Get("dynamo"):
				dc.ClientContext = v
			}
		case int64:
			switch k {
			case dcCreatedField.Tag.Get("dynamo"):
				dc.Created = v

			case dcUpdatedField.Tag.Get("dynamo"):
				dc.Updated = v

			case dcLastContactField.Tag.Get("dynamo"):
				dc.LastContact = v

			case dcIdField.Tag.Get("dynamo"):
				dc.Id = v
			}
		case int:
			switch k {
			case dcCreatedField.Tag.Get("dynamo"):
				dc.Created = int64(v)

			case dcUpdatedField.Tag.Get("dynamo"):
				dc.Updated = int64(v)

			case dcLastContactField.Tag.Get("dynamo"):
				dc.LastContact = int64(v)

			case dcIdField.Tag.Get("dynamo"):
				dc.Id = int64(v)
			}
		}
	}
	return &dc
}
func (dc *deviceContact) toMap() map[string]interface{} {
	dcMap := make(map[string]interface{})
	vReflect := reflect.Indirect(reflect.ValueOf(dc))
	t := vReflect.Type()
	for i := 0; i < vReflect.NumField(); i++ {
		k := t.Field(i).Tag.Get("dynamo")
		if k != "" && k != "-" {
			switch v := vReflect.Field(i).Interface().(type) {
			case string:
				if v != "" {
					dcMap[k] = v
				} else {
					panic(fmt.Sprintf("Field is empty", k))
				}
				
			default:
				dcMap[k] = v
			}
		}
	}
	return dcMap
}
