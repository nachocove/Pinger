package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/HostId"
	"github.com/nachocove/Pinger/Utils/Logging"
	"reflect"
	"time"
)

type PingerInfoDbHandler interface {
	insert(pinger *PingerInfo) error
	update(pinger *PingerInfo) (int64, error)
	delete(pinger *PingerInfo) (int64, error)
	get(keys []AWS.DBKeyValue) (*PingerInfo, error)
}

func newPingerInfoDbHandler(db DBHandler) PingerInfoDbHandler {
	if _, ok := db.(*DBHandleSql); ok {
		return newPingerInfoDbHandleSql(db)
	} else {
		return newPingerInfoDbHandleDynamo(db)
	}
}

type PingerInfo struct {
	Pinger  string `db:"pinger" dynamo:"pinger"`
	Created int64  `db:"created" dynamo:"created"`
	Updated int64  `db:"updated" dynamo:"updated"`

	dbHandler PingerInfoDbHandler `db:"-" dynamo:"-"`
	logger    *Logging.Logger     `db:"-" dynamo:"-"`
}

var pingerHostId string
var piCreatedField, piUpdatedField, piPingerField reflect.StructField

func init() {
	var ok bool
	pingerInfoReflection := reflect.TypeOf(PingerInfo{})
	piCreatedField, ok = pingerInfoReflection.FieldByName("Created")
	if ok == false {
		panic("Could not get Created Field information")
	}
	piUpdatedField, ok = pingerInfoReflection.FieldByName("Updated")
	if ok == false {
		panic("Could not get Updated Field information")
	}
	piPingerField, ok = pingerInfoReflection.FieldByName("Pinger")
	if ok == false {
		panic("Could not get Pinger Field information")
	}
	pingerHostId = HostId.HostId()
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
	if pinger.dbHandler == nil {
		panic("Can not update pinger info without having fetched it")
	}
	pinger.Updated = time.Now().UnixNano()
	n, err := pinger.dbHandler.update(pinger)
	if err != nil {
		panic(fmt.Sprintf("update error: %s", err.Error()))
	}
	return n, nil
}

func (pinger *PingerInfo) insert() error {
	if pinger.dbHandler == nil {
		panic("Can not update pinger info without db")
	}
	pinger.Created = time.Now().UnixNano()
	pinger.Updated = pinger.Created
	err := pinger.dbHandler.insert(pinger)
	if err != nil {
		panic(fmt.Sprintf("update error: %s", err.Error()))
	}
	return nil
}

func newPingerInfo(db DBHandler, logger *Logging.Logger) (*PingerInfo, error) {
	keys := []AWS.DBKeyValue{
		AWS.DBKeyValue{Key: "Pinger", Value: pingerHostId, Comparison: AWS.KeyComparisonEq},
	}
	h := newPingerInfoDbHandler(db)
	pinger, err := h.get(keys)
	if err != nil {
		return nil, err
	}
	if pinger != nil {
		pinger.logger = logger
		err = pinger.UpdateEntry()
		if err != nil {
			return nil, err
		}
	} else {
		pinger = &PingerInfo{Pinger: pingerHostId}
		pinger.dbHandler = h
		pinger.insert()
		pinger.logger = logger
	}
	return pinger, nil
}
