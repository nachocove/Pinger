package Pinger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/coopernurse/gorp"
	"net"
	"os"
	"reflect"
	"time"
)

type PingerInfo struct {
	Id      int64  `db:"id"`
	Pinger  string `db:"pinger"`
	Created int64  `db:"created"`
	Updated int64  `db:"updated"`

	dbm *gorp.DbMap `db:"-"`
}

const (
	PingerTableName string = "pinger_info"
)

var pingerHostId string
var getPingerSql string

func init() {
	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		if inter.Name[0:2] == "lo" {
			continue
		}
		if inter.HardwareAddr.String() == "" {
			continue
		}
		hash := sha256.New()
		hash.Write(inter.HardwareAddr)
		md := hash.Sum(nil)
		pingerHostId = hex.EncodeToString(md)
		break
	}
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
	tMap.SetVersionCol("Id")
	cMap := tMap.ColMap("Created")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Updated")
	cMap.SetNotNull(true)

	cMap = tMap.ColMap("Pinger")
	cMap.SetNotNull(true)
	cMap.SetUnique(true)
}

func getPingerInfo() *PingerInfo {
	return nil
}
func (pinger *PingerInfo) Updater() {
	pinger.UpdateEntry() // update now, then every 10 minutes
	ticker := time.NewTicker(time.Duration(10) * time.Minute)
	for {
		<-ticker.C
		pinger.UpdateEntry()
	}
}

func (pinger *PingerInfo) UpdateEntry() {
	pinger.Updated = time.Now().UnixNano()
	_, err := pinger.update()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not update pinger info")
	}
}

func (pinger *PingerInfo) update() (int64, error) {
	if pinger.dbm == nil {
		panic("Can not update device info without having fetched it")
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

func newPingerInfo(dbm *gorp.DbMap) (*PingerInfo, error) {
	obj, err := dbm.Get(&PingerInfo{}, pingerHostId)
	if err != nil {
		return nil, err
	}
	var pinger *PingerInfo
	if obj != nil {
		pinger = obj.(*PingerInfo)
	} else {
		pinger = &PingerInfo{Pinger: pingerHostId}
		dbm.Insert(pinger)
	}
	pinger.dbm = dbm
	return pinger, nil
}
