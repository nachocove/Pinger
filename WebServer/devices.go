package WebServer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
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

	err = saveDeviceInfo(context, clientid, platform, deviceInfo)
	if err != nil {
		context.Logger.Warning("Could not save deviceInfo: %v", err)
		http.Error(w, "MISSING DATA", http.StatusBadRequest)
		return
	}
	return
}

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
	switch {
	case platform == "ios":
		di, err = Pinger.NewDeviceInfoIOS(clientID,
			getString(deviceInfo, "DeviceID"),
			getString(deviceInfo, "PushToken"),
			getString(deviceInfo, "Topic"),
			getString(deviceInfo, "ResetToken"),
			getString(deviceInfo, "MailProtocol"))
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New(fmt.Sprintf("Unhandled platform %s", platform))
	}
	if di == nil {
		return nil, errors.New("Could not create DeviceInfo")
	}
	return di, nil
}

func updateDeviceInfoFromMap(di *Pinger.DeviceInfo, deviceInfo map[string]interface{}) (bool, error) {
	switch {
	case di.Platform == "ios":
		return updateDeviceInfoFromIOSMap(di, deviceInfo)
	}
	return false, errors.New(fmt.Sprintf("Unknown platform %s", di.Platform))
}

func updateDeviceInfoFromIOSMap(di *Pinger.DeviceInfo, deviceInfo map[string]interface{}) (bool, error) {
	changed := false
	info := Pinger.IOSDeviceInfoMap("", "", "")
	for key, value := range deviceInfo {
		switch {
		case strings.EqualFold(key, "DeviceID"):
			value := value.(string)
			if di.DeviceId != value {
				fmt.Printf("Changed DeviceID %s -> %s\n", di.DeviceId, value)
				di.DeviceId = value
				changed = true
			}

		case strings.EqualFold(key, "AWSPushToken"):
			value := value.(string)
			if di.AWSPushToken != value {
				fmt.Printf("Changed AWSPushToken %s -> %s\n", di.AWSPushToken, value)
				di.AWSPushToken = value
				changed = true
			}

		case strings.EqualFold(key, "MailProtocol"):
			value := value.(string)
			if di.MailClientType != value {
				fmt.Printf("Changed MailProtocol %s -> %s\n", di.MailClientType, value)
				di.MailClientType = value
				changed = true
			}

		default:
			matched, err := regexp.MatchString("(PushToken|Topic|ResetToken)", key)
			if err != nil {
				return false, err
			}
			if matched {
				info[key] = value.(string)
			} else {
				return false, errors.New(fmt.Sprintf("Not a valid Field %s", key))
			}
		}
	}
	infoMarshaled, err := json.Marshal(info)
	if err != nil {
		return false, err
	}
	infoString := string(infoMarshaled)
	if infoString != di.Info {
		fmt.Printf("Changed Info %s -> %s\n", di.Info, infoString)
		di.Info = string(infoString)
		changed = true
	}
	return changed, nil
}

func saveDeviceInfo(context *Context, clientId, platform string, deviceInfo map[string]interface{}) error {
	var err error
	di, err := Pinger.GetDeviceInfo(context.Dbm, clientId)
	if err != nil {
		return err
	}
	if di == nil {
		di, err = newDeviceInfoFromMap(clientId, platform, deviceInfo)
		if err != nil {
			return err
		}
		err = di.Insert(context.Dbm)
		if err != nil {
			return err
		}
		context.Logger.Debug("Created new entry for %s", clientId)
	} else {
		changed, err := updateDeviceInfoFromMap(di, deviceInfo)
		if err != nil {
			return err
		}
		if changed {
			n, err := di.Update(context.Dbm)
			if err != nil {
				return err
			}
			if n > 0 {
				context.Logger.Debug("Updated %s (%d rows)", clientId, n)
			}
		} else {
			context.Logger.Debug("No change from %s. No DB action take.", clientId)
		}
	}
	return nil
}
