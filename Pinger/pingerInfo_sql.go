package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
)

type PingerInfoDbHandleSql struct {
	db *DBHandleSql
}

func newPingerInfoDbHandleSql(db DBHandler) PingerInfoDbHandler {
	return &PingerInfoDbHandleSql{db.(*DBHandleSql)}
}

func (h *PingerInfoDbHandleSql) createTable() error {
	return nil
}

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

var getPingerSql string

func init() {
	getPingerSql = fmt.Sprintf("select * from %s where %s=?", PingerTableName, piPingerField.Tag.Get("db"))
}

func (h *PingerInfoDbHandleSql) update(pinger *PingerInfo) (int64, error) {
	return h.db.update(pinger, "")
}

func (h *PingerInfoDbHandleSql) insert(pinger *PingerInfo) error {
	return h.db.insert(pinger, "")
}

func (h *PingerInfoDbHandleSql) delete(pinger *PingerInfo) (int64, error) {
	return h.db.delete(pinger, "", nil)
}

func (h *PingerInfoDbHandleSql) get(keys []AWS.DBKeyValue) (*PingerInfo, error) {
	obj, err := h.db.get(&PingerInfo{}, "", keys)
	if err != nil {
		return nil, err
	}
	var pinger *PingerInfo
	if obj != nil {
		pinger = obj.(*PingerInfo)
		pinger.dbHandler = h
	}
	return pinger, nil
}
