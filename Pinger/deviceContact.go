package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
	"time"
)

type DeviceContactDbHandler interface {
	insert(dc *deviceContact) error
	update(dc *deviceContact) (int64, error)
	delete(dc *deviceContact) (int64, error)
	get(keys []AWS.DBKeyValue) (*deviceContact, error)
}

type deviceContact struct {
	Id                 int64  `db:"id"`
	Created            int64  `db:"created"`
	Updated            int64  `db:"updated"`
	LastContact        int64  `db:"last_contact"`
	LastContactRequest int64  `db:"last_contact_request"`
	ClientId           string `db:"client_id"` // us-east-1a-XXXXXXXX
	ClientContext      string `db:"client_context"`
	DeviceId           string `db:"device_id"` // NCHO348348384384.....

	db DeviceContactDbHandler `db:"-"`
}

func deviceContactGet(db DeviceContactDbHandler, clientId, clientContext, deviceId string) (*deviceContact, error) {
	keys := []AWS.DBKeyValue{
		// TODO Need to look into the struct for the db tags to get the column names
		// Note these are really only relevant to the dynamoDB sql handler. for gorp,
		// the keys should be in order they are in the struct, so we need to make sure
		// the order is correct here, as well as the values, but don't care about the column name.
		{Key: "client_id", Value: clientId, Comparison: AWS.KeyComparisonEq},
		{Key: "client_context", Value: clientContext, Comparison: AWS.KeyComparisonEq},
		{Key: "device_id", Value: deviceId, Comparison: AWS.KeyComparisonEq},
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
