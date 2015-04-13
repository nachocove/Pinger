package Pinger

import (
	"github.com/coopernurse/gorp"
	"time"
)

type deviceContactSqlDbHandler struct {
	DeviceContactDbHandler	
	dbm *gorp.DbMap
}

func newDeviceContactSqlDbHandler(dbm *gorp.DbMap) *deviceContactSqlDbHandler {
	return &deviceContactSqlDbHandler{
		dbm: dbm,
	}
}

func (h *deviceContactSqlDbHandler) insert(i interface{}) error {
	return h.dbm.Insert(i.(*deviceContact))
}

func (h *deviceContactSqlDbHandler) update(i interface{}) (int64, error) {
	n, err := h.dbm.Update(i.(*deviceContact))
	if err != nil {
		return n, err
	}
	return n, nil
}

func (h *deviceContactSqlDbHandler) delete(i interface{}) error {
	_, err := h.dbm.Delete(i.(*deviceContact))
	return err
}

func (h *deviceContactSqlDbHandler) get(args ...interface{}) (interface{}, error) {
	return h.dbm.Get(&deviceContact{}, args...)
}	

func (dc *deviceContact) PreInsert(s gorp.SqlExecutor) error {
	dc.Created = time.Now().UnixNano()
	dc.Updated = dc.Created
	dc.LastContact = dc.Created
	return nil
}