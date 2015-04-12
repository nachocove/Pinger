package Pinger

import (
	"time"
	"fmt"
)

type DeviceContactDbHandler interface {
	DbHandler	
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
	obj, err := db.get(clientId, clientContext, deviceId)
	if err != nil {
		return nil, err
	}
	var dc *deviceContact
	if obj != nil {
		dc = obj.(*deviceContact)
		dc.db = db
	}
	return dc, nil
}

func newDeviceContact(db DeviceContactDbHandler, clientId, clientContext, deviceId string) *deviceContact {
	dc := deviceContact{
		ClientId: clientId,
		ClientContext: clientContext,
		DeviceId: deviceId,
	}
	dc.db = db
	return &dc
}

func (dc *deviceContact) insert() error {
	return dc.db.insert(dc)
}

func (di *DeviceInfo) updateLastContact() error {
	dc, err := di.getContactInfoObj(false)
	if err != nil {
		return err
	}
	dc.LastContact = time.Now().UnixNano()
	_, err = dc.db.update(dc)
	if err != nil {
		return err
	}
	return nil
}

func (di *DeviceInfo) updateLastContactRequest() error {
	dc, err := di.getContactInfoObj(false)
	if err != nil {
		return err
	}
	dc.LastContactRequest = time.Now().UnixNano()
	_, err = dc.db.update(dc)
	if err != nil {
		return err
	}
	return nil
}

func (di *DeviceInfo) getContactInfoObj(insert bool) (*deviceContact, error) {
	if di.db == nil {
		panic("Must have fetched di first")
	}
	var db DeviceContactDbHandler
	diSql, ok := di.db.(*DeviceInfoSqlHandler)
	if ok {
		db = newDeviceContactSqlDbHandler(diSql.dbm)
	} else {
		panic("Need to create gorp dbm stuff here")
	}
	dc, err := deviceContactGet(db, di.ClientId, di.ClientContext, di.DeviceId)
	if err != nil {
		return nil, err
	}
	if dc == nil {
		if insert {
			dc = newDeviceContact(db, di.ClientId, di.ClientContext, di.DeviceId)
			err = dc.insert()
			if err != nil {
				panic(err)
			}
		} else {
			return nil, fmt.Errorf("No object found")
		}
	}
	return dc, nil
}

func (di *DeviceInfo) getContactInfo(insert bool) (int64, int64, error) {
	dc, err := di.getContactInfoObj(insert)
	if err != nil {
		return 0, 0, err
	}
	return dc.LastContact, dc.LastContactRequest, nil
}
