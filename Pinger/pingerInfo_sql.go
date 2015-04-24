package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
)

func addPingerInfoTable(dbmap *gorp.DbMap) {
	tMap := dbmap.AddTableWithName(PingerInfo{}, PingerTableName)
	if tMap.SetKeys(false, "Pinger") == nil {
		panic(fmt.Sprintf("Could not create key on %s:Pinger", PingerTableName))
	}
	//tMap.SetVersionCol("Id")
	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Pinger")
	cMap.SetNotNull(true)
	cMap.SetUnique(true)
}

const (
	PingerTableName string = "pinger_info"
)

type PingerInfoSqlHandler struct {
	PingerInfoDbHandler
	dbm *gorp.DbMap
}

func newPingerInfoSqlHandler(dbm *gorp.DbMap) (*PingerInfoSqlHandler, error) {
	return &PingerInfoSqlHandler{dbm: dbm}, nil
}

var getPingerSql string

func init() {
	getPingerSql = fmt.Sprintf("select * from %s where %s=?", PingerTableName, piPingerField.Tag.Get("db"))
}

func (h *PingerInfoSqlHandler) update(pinger *PingerInfo) (int64, error) {
	n, err := h.dbm.Update(pinger)
	if err != nil {
		panic(fmt.Sprintf("update error: %s", err.Error()))
	}
	return n, nil
}

func (h *PingerInfoSqlHandler) insert(pinger *PingerInfo) error {
	return h.dbm.Insert(pinger)
}

func (h *PingerInfoSqlHandler) delete(pinger *PingerInfo) (int64, error) {
	return h.dbm.Delete(pinger)
}

func (h *PingerInfoSqlHandler) get(keys []AWS.DBKeyValue) (*PingerInfo, error) {
	args := make([]interface{}, 0, len(keys))
	for _, a := range keys {
		if a.Comparison != AWS.KeyComparisonEq {
			panic("Can only use KeyComparisonEq for get")
		}
		args = append(args, a.Value)
	}
	obj, err := h.dbm.Get(&PingerInfo{}, args...)
	if err != nil {
		return nil, err
	}
	var pinger *PingerInfo
	if obj != nil {
		pinger = obj.(*PingerInfo)
		pinger.db = h
	}
	return pinger, nil

}
