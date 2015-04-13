package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
)

type DeviceInfoDynamoHandler struct {
	DbHandler
	dynamo AWS.DynamoDbHandler
	tableName string
}

const (
	dynamoDeviceInfoTableName = "alpha.pinger.device_info"
)

func newDeviceInfoDynamoHandler(aws AWS.AWSHandler, tableName string) *DeviceInfoDynamoHandler {
	return &DeviceInfoDynamoHandler{
		dynamo: aws.GetDynamoDbSession(),
		tableName: tableName,
	}
}

func (h *DeviceInfoDynamoHandler) Get([]DBKeyValue) (map[string]interface{}, error) {
	h.dynamo.Get(dynamoDeviceInfoTableName, )
	err := fmt.Errorf("Not implemented")
	return nil, err
}

func (h *DeviceInfoDynamoHandler) Search(keys []DBKeyValue) ([]map[string]interface{}, error) {
	return nil, nil
}

func (h *DeviceInfoDynamoHandler) Insert(args ...interface{}) error {
	err := fmt.Errorf("Not implemented")
	return err
}

func (h *DeviceInfoDynamoHandler) Update(args ...interface{}) (int64, error) {
	err := fmt.Errorf("Not implemented")
	return 0, err
}

func (h *DeviceInfoDynamoHandler) Delete(args ...interface{}) error {
	err := fmt.Errorf("Not implemented")
	return err
	
}
