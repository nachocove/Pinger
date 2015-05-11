package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"time"
)

type deviceContactSqlDbHandler struct {
	DeviceContactDbHandler
	dbm *gorp.DbMap
}

const (
	deviceContactTableName string = "device_contact"
)

func addDeviceContactTable(dbmap *gorp.DbMap) {
	tMap := dbmap.AddTableWithName(deviceContact{}, deviceContactTableName)
	if tMap.SetKeys(false, "ClientId", "ClientContext", "DeviceId") == nil {
		panic(fmt.Sprintf("Could not create key on %s:ID", deviceContactTableName))
	}
	tMap.SetVersionCol("Id")

	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("LastContact")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientId")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("ClientContext")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("DeviceId")
	cMap.SetNotNull(true)
}

func newDeviceContactSqlDbHandler(dbm *gorp.DbMap) *deviceContactSqlDbHandler {
	return &deviceContactSqlDbHandler{
		dbm: dbm,
	}
}

func (h *deviceContactSqlDbHandler) insert(dc *deviceContact) error {
	return h.dbm.Insert(dc)
}

func (h *deviceContactSqlDbHandler) update(dc *deviceContact) (int64, error) {
	n, err := h.dbm.Update(dc)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (h *deviceContactSqlDbHandler) delete(dc *deviceContact) (int64, error) {
	return h.dbm.Delete(dc)
}

func (h *deviceContactSqlDbHandler) get(keys []AWS.DBKeyValue) (*deviceContact, error) {
	args := make([]interface{}, 0, len(keys))
	for _, a := range keys {
		if a.Comparison != AWS.KeyComparisonEq {
			panic("Can only use KeyComparisonEq for get")
		}
		args = append(args, a.Value)
	}

	obj, err := h.dbm.Get(&deviceContact{}, args...)
	if err != nil {
		return nil, err
	}
	var dc *deviceContact
	if obj != nil {
		dc = obj.(*deviceContact)
		dc.db = h
	}
	return dc, nil
}

func (dc *deviceContact) PreInsert(s gorp.SqlExecutor) error {
	dc.Created = time.Now().UnixNano()
	dc.Updated = dc.Created
	dc.LastContact = dc.Created
	return nil
}
