package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
	"strings"
	"fmt"
)

type DeviceInfoDbHandleDynamo struct {
	db *DBHandleDynamo
}

const (
	dynamoDeviceInfoTableName             = "alpha.pinger.device_info"
	dynamoDeviceInfoPingerClientIndexName = "index.pinger-client"
	dynamoDeviceInfoServiceTokenIndexName = "index.service-token"
	dynamoDeviceInfoClientDeviceIndexName = "index.client-device"
)

func newDeviceInfoDynamoDbHandler(db DBHandler) DeviceInfoDbHandler {
	return &DeviceInfoDbHandleDynamo{db.(*DBHandleDynamo)}
}

func (h *DeviceInfoDbHandleDynamo) createTable() error {
	_, err := h.db.dynamo.DescribeTable(dynamoDeviceInfoTableName)
	if err != nil {
		if !strings.Contains("Cannot do operations on a non-existent table", err.Error()) {
			return err
		}
	} else {
		return nil
	}
	
	createReq := h.db.dynamo.CreateTableReq(dynamoDeviceInfoTableName,
		[]AWS.DBAttrDefinition{
			{Name: diIdField.Tag.Get("dynamo"), Type: AWS.Number},
			{Name: diPingerField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: diClientIdField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: diDeviceIdField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: diPushTokenField.Tag.Get("dynamo"), Type: AWS.String},
			{Name: diPushServiceField.Tag.Get("dynamo"), Type: AWS.String},
		},
		[]AWS.DBKeyType{
			{Name: diIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)

	err = h.db.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoDeviceInfoPingerClientIndexName,
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

	err = h.db.dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoDeviceInfoClientDeviceIndexName,
		[]AWS.DBKeyType{
			{Name: diIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeHash},
			{Name: diClientIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
			{Name: diDeviceIdField.Tag.Get("dynamo"), Type: AWS.KeyTypeRange},
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
	return h.db.delete(&DeviceInfo{}, dynamoDeviceInfoTableName, keys)
}

func (h *DeviceInfoDbHandleDynamo) findIndexForKeys(keys []AWS.DBKeyValue) (string, []AWS.DBKeyValue, error) {
	var indexName string
	filteredKeys := make([]AWS.DBKeyValue, 0, 1)
	switch {
	case keys[0].Key == "ClientId" && keys[1].Key == "ClientContext" && keys[2].Key == "DeviceId" && keys[3].Key == "SessionId":
		indexName = dynamoDeviceInfoClientDeviceIndexName
		filteredKeys = append(filteredKeys, keys[0])
		filteredKeys = append(filteredKeys, keys[2])
		
	case keys[0].Key == "Pinger":
		indexName = dynamoDeviceInfoPingerClientIndexName
		filteredKeys = append(filteredKeys, keys[0])

	case keys[0].Key == "PushService" && keys[1].Key == "PushToken":
		indexName = dynamoDeviceInfoPingerClientIndexName
		filteredKeys = append(filteredKeys, keys[0])
		filteredKeys = append(filteredKeys, keys[1])
	}
	return indexName, filteredKeys, nil	
}

func (h *DeviceInfoDbHandleDynamo) get(keys []AWS.DBKeyValue) (*DeviceInfo, error) {
	var obj interface{}
	indexName, filtered, err := h.findIndexForKeys(keys)
	if err != nil {
		return nil, err
	}
	if indexName != "" {
		objs, err := h.db.search(&DeviceInfo{}, dynamoDeviceInfoTableName, indexName, filtered)
		if err != nil {
			return nil, err
		}
		if len(objs) > 1 {
			return nil, fmt.Errorf("Query returned more than one object (%d): %+v", len(objs), keys)
		}
		if len(objs) == 1 {
			obj = objs[0]
		}
	} else {
		obj, err = h.db.get(&DeviceInfo{}, dynamoDeviceInfoTableName, keys)
		if err != nil {
			return nil, err
		}
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
	indexName, filtered, err := h.findIndexForKeys(keys)
	if err != nil {
		return nil, err
	}
	objs, err := h.db.search(&DeviceInfo{}, dynamoDeviceInfoTableName, indexName, filtered)
	if err != nil {
		return nil, err
	}
	// TODO Need to make sure to return the DISTINCT pushservice/pushTokens
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
	indexName, filtered, err := h.findIndexForKeys(keys)
	if err != nil {
		return nil, err
	}	
	fmt.Printf("JAN: searching on %s %s\n", dynamoDeviceInfoTableName, indexName)
	objs, err := h.db.search(&DeviceInfo{}, dynamoDeviceInfoTableName, indexName, filtered)
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

func (di *DeviceInfo) ToType(m *map[string]interface{}) (interface{}, error) {
	newDi := DeviceInfo{}
	var err error
	errString := make([]string, 0, 0)
	for k, v := range *m {
		switch k {
		case piPingerField.Tag.Get("dynamo"):
			newDi.Pinger = v.(string)
			
		case piUpdatedField.Tag.Get("dynamo"):
			newDi.Updated = v.(int64)
			
		case piCreatedField.Tag.Get("dynamo"):
			newDi.Created = v.(int64)
			
		default:
			errString = append(errString, fmt.Sprintf("Unhandled key %s", k))
		}
	}
	if len(errString) > 0 {
		err = fmt.Errorf(strings.Join(errString, ", "))
	}
	return &newDi, err
}
