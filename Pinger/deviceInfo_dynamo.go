package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
)

type DeviceInfoDbHandleDynamo struct {
	db *DBHandleDynamo
}

const (
	dynamoDeviceInfoTableName             = "alpha.pinger.device_info"
	dynamoDeviceInfoPingerClientIndexName = "index.pinger-device"
	dynamoDeviceInfoServiceTokenIndexName = "index.service-token"
)

func newDeviceInfoDynamoDbHandler(db DBHandler) DeviceInfoDbHandler {
	return &DeviceInfoDbHandleDynamo{db.(*DBHandleDynamo)}
}

func (h *DeviceInfoDbHandleDynamo) createDeviceInfoTable() error {
	createReq := h.db.dynamo.CreateTableReq(dynamoDeviceInfoTableName,
		[]AWS.DBAttrDefinition{
			{Name: diIdField.Tag.Get("dynamo"), Type: AWS.Number},
			{Name: diPingerField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: diClientIdField.Tag.Get("dynamo"), Type: AWS.String},
		},
		[]AWS.DBKeyType{
			{Name: diIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)

	err := h.db.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoDeviceInfoPingerClientIndexName,
		[]AWS.DBKeyType{
			{Name: diPingerField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
			{Name: diClientIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)
	if err != nil {
		return err
	}

	err = h.db.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoDeviceInfoServiceTokenIndexName,
		[]AWS.DBKeyType{
			{Name: diPushTokenField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
			{Name: diPushServiceField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)
	if err != nil {
		return err
	}

	err = h.db.dynamo.CreateTable(createReq)
	if err != nil {
		return err
	}
	return nil
}

var DeviceInfoReflection reflect.Type

func init() {
	DeviceInfoReflection = reflect.TypeOf(DeviceInfo{})
}

func (h *DeviceInfoDbHandleDynamo) insert(di *DeviceInfo) error {
	return h.db.insert(di, dynamoDeviceInfoTableName)
}

func (h *DeviceInfoDbHandleDynamo) update(di *DeviceInfo) (int64, error) {
	return h.db.update(di, dynamoDeviceInfoTableName)
}

func (h *DeviceInfoDbHandleDynamo) delete(di *DeviceInfo) (int64, error) {
	keys := []AWS.DBKeyValue{
		AWS.DBKeyValue{Key: "Id", Value: di.Id, Comparison: AWS.KeyComparisonEq},
	}
	return h.db.delete(DeviceInfo{}, dynamoDeviceInfoTableName, keys)
}

func (h *DeviceInfoDbHandleDynamo) get(keys []AWS.DBKeyValue) (*DeviceInfo, error) {
	obj, err := h.db.get(DeviceInfo{}, dynamoDeviceInfoTableName, keys)
	if err != nil {
		return nil, err
	}
	var di *DeviceInfo
	if obj != nil {
		di = obj.(*DeviceInfo)
		di.dbHandler = h
	}
	return di, nil
}

func (h *DeviceInfoDbHandleDynamo) distinctPushServiceTokens(pingerHostId string) ([]DeviceInfo, error) {
	keys := []AWS.DBKeyValue{
		AWS.DBKeyValue{Key: "Pinger", Value: pingerHostId, Comparison: AWS.KeyComparisonEq},
	}
	objs, err := h.db.search(DeviceInfo{}, dynamoDeviceInfoTableName, dynamoDeviceInfoPingerClientIndexName, keys)
	if err != nil {
		return nil, err
	}
	servicesAndTokens := make([]DeviceInfo, 0, len(objs))
	for _, obj := range objs {
		di := obj.(*DeviceInfo)
		di.dbHandler = h
		servicesAndTokens = append(servicesAndTokens, *di)
	}
	return servicesAndTokens, nil
}

func (h *DeviceInfoDbHandleDynamo) clientContexts(pushservice, pushToken string) ([]string, error) {
	keys := []AWS.DBKeyValue{
		AWS.DBKeyValue{Key: "PushService", Value: pushservice, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "PushToken", Value: pushToken, Comparison: AWS.KeyComparisonEq},
	}
	objs, err := h.db.search(DeviceInfo{}, dynamoDeviceInfoTableName, dynamoDeviceInfoPingerClientIndexName, keys)
	if err != nil {
		return nil, err
	}
	contexts := make([]string, 0, len(objs))
	for _, obj := range objs {
		di := obj.(*DeviceInfo)
		contexts = append(contexts, di.ClientContext)
	}
	return contexts, nil
}

func (h *DeviceInfoDbHandleDynamo) getAllMyDeviceInfo(pingerHostId string) ([]DeviceInfo, error) {
	deviceList := make([]DeviceInfo, 0, 100)
	return deviceList, nil
}
