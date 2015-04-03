package Pinger

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/HostId"
	"github.com/nachocove/Pinger/Utils/Logging"
	"reflect"
	"time"
)

type PingerInfo struct {
	Id      int64  `db:"id"`
	Pinger  string `db:"pinger"`
	Created int64  `db:"created"`
	Updated int64  `db:"updated"`

	dbm    *gorp.DbMap     `db:"-"`
	logger *Logging.Logger `db:"-"`
}

const (
	PingerTableName string = "pinger_info"
)

var pingerHostId string
var getPingerSql string

func init() {
	pingerHostId = HostId.HostId()
	var pingerInfoReflection reflect.Type
	var pingerField reflect.StructField
	var ok bool
	pingerInfoReflection = reflect.TypeOf(PingerInfo{})
	pingerField, ok = pingerInfoReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	getPingerSql = fmt.Sprintf("select * from %s where %s=?", PingerTableName, pingerField.Tag.Get("db"))
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

func (pinger *PingerInfo) Updater(minutes int) {
	d := time.Duration(minutes) * time.Minute
	ticker := time.NewTicker(d)
	for {
		<-ticker.C
		err := pinger.UpdateEntry()
		if err != nil {
			panic("Could not update entry")
		}
	}
}

func (pinger *PingerInfo) UpdateEntry() error {
	pinger.Updated = time.Now().UnixNano()
	n, err := pinger.update()
	if err != nil {
		return err
	}
	if n <= 0 {
		return fmt.Errorf("%d rows updated. That's not right.", n)
	}
	pinger.logger.Info("%s: Pinger marked as alive", pinger.Pinger)
	return nil
}

func (pinger *PingerInfo) update() (int64, error) {
	if pinger.dbm == nil {
		panic("Can not update pinger info without having fetched it")
	}
	n, err := pinger.dbm.Update(pinger)
	if err != nil {
		panic(fmt.Sprintf("update error: %s", err.Error()))
	}
	return n, nil
}

func (pinger *PingerInfo) PreInsert(s gorp.SqlExecutor) error {
	pinger.Created = time.Now().UnixNano()
	pinger.Updated = pinger.Created
	return nil
}

func newPingerInfo(dbm *gorp.DbMap, logger *Logging.Logger) (*PingerInfo, error) {
	obj, err := dbm.Get(&PingerInfo{}, pingerHostId)
	if err != nil {
		return nil, err
	}
	var pinger *PingerInfo
	if obj != nil {
		pinger = obj.(*PingerInfo)
		pinger.dbm = dbm
		pinger.logger = logger
		err = pinger.UpdateEntry()
		if err != nil {
			return nil, err
		}
	} else {
		pinger = &PingerInfo{Pinger: pingerHostId}
		dbm.Insert(pinger)
		pinger.dbm = dbm
		pinger.logger = logger
	}
	return pinger, nil
}
