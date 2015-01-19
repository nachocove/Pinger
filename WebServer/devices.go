package WebServer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/nachocove/Pinger/Pinger"
)

func init() {
	httpsRouter.HandleFunc("/register/{clientid}/{platform:ios|android}", registerDevice)
}

func decodePost(data []byte) (map[string]interface{}, error) {
	var f interface{}
	err := json.Unmarshal(data, &f)
	if err != nil {
		return nil, err
	}
	return f.(map[string]interface{}), nil
}

// TODO Need to figure out Auth
// Protocol: POST device=<json-encoded deviceInfo>
// deviceInfo is map/dict with keys:
//     DeviceID = Nacho Device ID, i.e. NchoXYZ
//     PushService = AWS|APNS|GCM|....
//     PushToken = The PushToken. For AWS: Endpoint ARN. For APNS, the PushToken
//     MailProtocol = exchange, imap, pop...
//     MailCredentials = json-encoded information. usually username and password.
func registerDevice(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	clientid := vars["clientid"]
	platform := vars["platform"]
	r.ParseForm()
	context.Logger.Debug("Received POST: %v", r.PostForm)
	devicePost := r.PostForm.Get("device")
	if len(devicePost) == 0 {
		context.Logger.Warning("No Device-info posted")
		http.Error(w, "MISSING DATA", http.StatusBadRequest)
		return
	}
	deviceInfo, err := decodePost([]byte(devicePost))
	if err != nil {
		context.Logger.Error("Could not parse json: %s %v", devicePost, err)
		http.Error(w, "Could not parse json", http.StatusBadRequest)
		return
	}
	context.Logger.Debug("deviceInfo: %v", deviceInfo)
	if len(deviceInfo) == 0 {
		context.Logger.Warning("No Device-info posted")
		http.Error(w, "MISSING DATA", http.StatusBadRequest)
		return
	}

	di, err := saveDeviceInfo(context, clientid, platform, deviceInfo)
	if err != nil {
		context.Logger.Warning("Could not save deviceInfo: %v", err)
		http.Error(w, "MISSING DATA", http.StatusBadRequest)
		return
	}
	context.Logger.Debug("created/updated device info %s", di.ClientId)
	// TODO Need to now punt this (and the un-saved mail credentials) to a go routine. Need
	// to be able to look up whether a goroutine already exists, so we don't create a new one
	// for every identical call
	return
}

////////////////////////////////////////////////////////////////////////
// Helper functions
////////////////////////////////////////////////////////////////////////

func getString(myMap map[string]interface{}, key string) string {
	x, ok := myMap[key]
	if ok {
		return x.(string)
	} else {
		return ""
	}
}
func newDeviceInfoFromMap(clientID, platform string, deviceInfo map[string]interface{}) (*Pinger.DeviceInfo, error) {
	var di *Pinger.DeviceInfo
	var err error
	di, err = Pinger.NewDeviceInfo(
		clientID,
		getString(deviceInfo, "DeviceID"),
		getString(deviceInfo, "PushToken"),
		getString(deviceInfo, "PushService"),
		platform,
		getString(deviceInfo, "MailProtocol"))
	if err != nil {
		return nil, err
	}
	if di == nil {
		return nil, errors.New("Could not create DeviceInfo")
	}
	return di, nil
}

func updateDeviceInfoFromMap(di *Pinger.DeviceInfo, deviceInfo map[string]interface{}) (bool, error) {
	changed := false
	// TODO Figure out how to do all this with introspected/reflection/whatever
	for key, value := range deviceInfo {
		switch {
		case strings.EqualFold(key, "DeviceID"):
			value := value.(string)
			if di.DeviceId != value {
				di.DeviceId = value
				changed = true
			}

		case strings.EqualFold(key, "MailProtocol"):
			value := value.(string)
			if di.MailClientType != value {
				di.MailClientType = value
				changed = true
			}

		case strings.EqualFold(key, "PushToken"):
			value := value.(string)
			if di.PushToken != value {
				di.PushToken = value
				changed = true
			}

		case strings.EqualFold(key, "PushService"):
			value := value.(string)
			if di.PushService != value {
				di.PushService = value
				changed = true
			}

		default:
			return false, errors.New(fmt.Sprintf("Not a valid Field %s", key))
		}
	}
	return changed, nil
}

func saveDeviceInfo(context *Context, clientId, platform string, deviceInfo map[string]interface{}) (*Pinger.DeviceInfo, error) {
	var err error
	di, err := Pinger.GetDeviceInfo(context.Dbm, clientId)
	if err != nil {
		return nil, err
	}
	if di == nil {
		di, err = newDeviceInfoFromMap(clientId, platform, deviceInfo)
		if err != nil {
			return nil, err
		}
		err = context.Dbm.Insert(di)
		if err != nil {
			return nil, err
		}
		context.Logger.Debug("Created new entry for %s", clientId)
	} else {
		changed, err := updateDeviceInfoFromMap(di, deviceInfo)
		if err != nil {
			return nil, err
		}
		if changed {
			n, err := context.Dbm.Update(di)
			if err != nil {
				return nil, err
			}
			if n > 0 {
				context.Logger.Debug("Updated %s (%d rows)", clientId, n)
			}
		} else {
			context.Logger.Debug("No change from %s. No DB action take.", clientId)
		}
	}
	return di, nil
}
