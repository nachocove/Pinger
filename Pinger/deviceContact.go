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
	Id                 int64  `db:"id", dynamo:"id"`
	Created            int64  `db:"created", dynamo:"created"`
	Updated            int64  `db:"updated", dynamo:"updated"`
	LastContact        int64  `db:"last_contact", dynamo:"last_contact"`
	LastContactRequest int64  `db:"last_contact_request", dynamo:"last_contact_request"`
	ClientId           string `db:"client_id", dynamo:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext      string `db:"client_context", dynamo:"client_context"`
	DeviceId           string `db:"device_id", dynamo:"device_id"` // NCHO348348384384.....
	PushToken          string `db:"push_token", dynamo:"push_token"`
	PushService        string `db:"push_service", dynamo:"push_service"` // APNS, GCM, ...
	Pinger             string `db:"pinger"`

	db DeviceContactDbHandler `db:"-"`
}

var clientIdField, clientContextField, deviceIdField, pushServiceField, pushTokenField reflect.StructField

func init() {
	var ok bool
	deviceContactReflection := reflect.TypeOf(deviceContact{})
	clientIdField, ok = deviceContactReflection.FieldByName("ClientId")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	clientContextField, ok = deviceContactReflection.FieldByName("ClientContext")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	deviceIdField, ok = deviceContactReflection.FieldByName("DeviceId")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	pushServiceField, ok = deviceContactReflection.FieldByName("PushService")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	pushTokenField, ok = deviceContactReflection.FieldByName("PushToken")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
}

func deviceContactGet(db DeviceContactDbHandler, clientId, clientContext, deviceId string) (*deviceContact, error) {
	keys := []AWS.DBKeyValue{
		// TODO Need to look into the struct for the db tags to get the column names
		// Note these are really only relevant to the dynamoDB sql handler. for gorp,
		// the keys should be in order they are in the struct, so we need to make sure
		// the order is correct here, as well as the values, but don't care about the column name.
		AWS.DBKeyValue{Key: clientIdField.Tag.Get("db"), Value: clientId, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: clientContextField.Tag.Get("db"), Value: clientContext, Comparison: AWS.KeyComparisonEq},
		AWS.DBKeyValue{Key: deviceIdField.Tag.Get("db"), Value: deviceId, Comparison: AWS.KeyComparisonEq},
	}
	dc, err := db.get(keys)
	if err != nil {
		return nil, err
	}
	return dc, nil
}

func newDeviceContact(db DeviceContactDbHandler, clientId, clientContext, deviceId string) *deviceContact {
	dc := deviceContact{
		ClientId:      clientId,
		ClientContext: clientContext,
		DeviceId:      deviceId,
	}
	dc.db = db
	return &dc
}

func (dc *deviceContact) insert() error {
	return dc.db.insert(dc)
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
	_, err := dc.db.update(dc)
	if err != nil {
		return err
	}
	return nil
}

func (dc *deviceContact) updateLastContactRequest() error {
	dc.LastContactRequest = time.Now().UnixNano()
	_, err := dc.db.update(dc)
	if err != nil {
		return err
	}
	return nil
}
