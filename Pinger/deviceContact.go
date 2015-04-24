package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"reflect"
	"time"
)

type DeviceContactDbHandler interface {
	insert(dc *deviceContact) error
	update(dc *deviceContact) (int64, error)
	delete(dc *deviceContact) (int64, error)
	get(keys []AWS.DBKeyValue) (*deviceContact, error)
	findByPingerId(pingerId string) ([]*deviceContact, error)
}

type deviceContact struct {
	Id                 int64  `db:"id" dynamo:"id"`
	Created            int64  `db:"created" dynamo:"created"`
	Updated            int64  `db:"updated" dynamo:"updated"`
	LastContact        int64  `db:"last_contact" dynamo:"last_contact"`
	LastContactRequest int64  `db:"last_contact_request" dynamo:"last_contact_request"`
	ClientId           string `db:"client_id" dynamo:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext      string `db:"client_context" dynamo:"client_context"`
	DeviceId           string `db:"device_id" dynamo:"device_id"` // NCHO348348384384.....
	PushToken          string `db:"push_token" dynamo:"push_token"`
	PushService        string `db:"push_service" dynamo:"push_service"` // APNS, GCM, ...
	Pinger             string `db:"pinger" dynamo:"pinger"`

	db DeviceContactDbHandler `db:"-" dynamo:"-"`
}

var dcIdField, dcCreatedField, dcUpdatedField, dcLastContactField, dcLastContactRequestField, dcPingerField, dcClientIdField, dcClientContextField, dcDeviceIdField, dcPushServiceField, dcPushTokenField reflect.StructField

func init() {
	var ok bool
	deviceContactReflection := reflect.TypeOf(deviceContact{})
	dcIdField, ok = deviceContactReflection.FieldByName("Id")
	if ok == false {
		panic("Could not get Id Field information")
	}
	dcPingerField, ok = deviceContactReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	dcCreatedField, ok = deviceContactReflection.FieldByName("Created")
	if ok == false {
		panic("Could not get Created Field information")
	}
	dcUpdatedField, ok = deviceContactReflection.FieldByName("Updated")
	if ok == false {
		panic("Could not get Updated Field information")
	}
	dcLastContactField, ok = deviceContactReflection.FieldByName("LastContact")
	if ok == false {
		panic("Could not get LastContact Field information")
	}
	dcLastContactRequestField, ok = deviceContactReflection.FieldByName("LastContactRequest")
	if ok == false {
		panic("Could not get LastContactRequest Field information")
	}
	dcClientIdField, ok = deviceContactReflection.FieldByName("ClientId")
	if ok == false {
		panic("Could not get ClientId Field information")
	}
	dcClientContextField, ok = deviceContactReflection.FieldByName("ClientContext")
	if ok == false {
		panic("Could not get ClientContext Field information")
	}
	dcDeviceIdField, ok = deviceContactReflection.FieldByName("DeviceId")
	if ok == false {
		panic("Could not get DeviceId Field information")
	}
	dcPushServiceField, ok = deviceContactReflection.FieldByName("PushService")
	if ok == false {
		panic("Could not get PushService Field information")
	}
	dcPushTokenField, ok = deviceContactReflection.FieldByName("PushToken")
	if ok == false {
		panic("Could not get PushToken Field information")
	}
}

func deviceContactGet(db DeviceContactDbHandler, clientId, clientContext, deviceId string) (*deviceContact, error) {
	keys := []AWS.DBKeyValue{
		// TODO Need to look into the struct for the db tags to get the column names
		// Note these are really only relevant to the dynamoDB sql handler. for gorp,
		// the keys should be in order they are in the struct, so we need to make sure
		// the order is correct here, as well as the values, but don't care about the column name.
		AWS.DBKeyValue{Key: "ClientId", Value: clientId, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "ClientContext", Value: clientContext, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "DeviceId", Value: deviceId, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: "Pinger", Value: pingerHostId, Comparison: AWS.KeyComparisonEq},
	}
	dc, err := db.get(keys)
	if err != nil {
		return nil, err
	}
	return dc, nil
}

func (dc *deviceContact) validate() error {
	vReflect := reflect.Indirect(reflect.ValueOf(dc))
	t := vReflect.Type()
	emptyFields := make([]string, 0, 0)
	for i := 0; i < vReflect.NumField(); i++ {
		k := t.Field(i).Name
		switch k {
		case "LastContactRequest":
			continue
			
		case "db":
			continue
			
		case "Id":
			continue
		}
		
		switch v := vReflect.Field(i).Interface().(type) {
		case string:
			if v == "" {
				emptyFields = append(emptyFields, k)
			}
			
		case int:
			if v == 0 {
				emptyFields = append(emptyFields, k)
			}

		case int64:
			if v == 0 {
				emptyFields = append(emptyFields, k)
			}
		}
	}
	if len(emptyFields) > 0 {
		return fmt.Errorf(fmt.Sprintf("Empty fields: %v", emptyFields))
	}
	return nil
}
func newDeviceContact(db DeviceContactDbHandler, clientId, clientContext, deviceId, pushService, pushToken string) *deviceContact {
	dc := deviceContact{
		ClientId:      clientId,
		ClientContext: clientContext,
		DeviceId:      deviceId,
		PushService:   pushService,
		PushToken:     pushToken,
	}
	dc.db = db
	return &dc
}

func (dc *deviceContact) insert() error {
	dc.Created = time.Now().UTC().Unix()
	dc.Updated = dc.Created
	dc.LastContact = dc.Created
	dc.Pinger = pingerHostId
	
	err := dc.validate()
	if err != nil {
		return err
	}
	return dc.db.insert(dc)
}

func (dc *deviceContact) update() (int64, error) {
	dc.Updated = time.Now().UTC().Unix()
	dc.Pinger = pingerHostId
	
	err := dc.validate()
	if err != nil {
		return 0, err
	}
	return dc.db.update(dc)
}

func (dc *deviceContact) delete() error {
	n, err := dc.db.delete(dc)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("Expected to delete entry, but none was")
	}
	return nil
}


func (dc *deviceContact) updateLastContact() error {
	dc.LastContact = time.Now().UnixNano()
	_, err := dc.update()
	if err != nil {
		return err
	}
	return nil
}

func (dc *deviceContact) updateLastContactRequest() error {
	dc.LastContactRequest = time.Now().UnixNano()
	_, err := dc.update()
	if err != nil {
		return err
	}
	return nil
}
