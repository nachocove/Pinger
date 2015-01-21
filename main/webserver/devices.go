package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nachocove/Pinger/Pinger"
)

func init() {
	httpsRouter.HandleFunc("/register", registerDevice)
}

// TODO Need to figure out Auth
// Protocol: POST device=<json-encoded deviceInfo>
func registerDevice(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "POST" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	session, err := context.SessionStore.Get(r, "pinger-session")
	if err != nil {
		context.Logger.Warning("Could not get session")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "UNKNOWN Encoding", http.StatusBadRequest)
		return
	}

	postInfo := Pinger.MailPingInformation{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&postInfo)
	if err != nil {
		context.Logger.Error("Could not parse json %s", err)
		http.Error(w, "Could not parse json", http.StatusBadRequest)
		return
	}
	if postInfo.Validate() == false {
		context.Logger.Warning("Missing non-optional data")
		http.Error(w, "MISSING DATA", http.StatusBadRequest)
		return
	}

	err = saveDeviceInfo(context, &postInfo)
	if err != nil {
		context.Logger.Warning("Could not save deviceInfo: %s", err)
		http.Error(w, "MISSING DATA", http.StatusBadRequest)
		return
	}
	context.Logger.Debug("created/updated device info %s", postInfo.ClientId)

	session.Values["ClientId"] = postInfo.ClientId

	err = postInfo.StartPoll(context.RpcConnectString)
	if err != nil {
		context.Logger.Warning("Could not start polling for device %s: %s", postInfo.ClientId, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	context.Logger.Debug("Re/Started Polling for %s", postInfo.ClientId)
	
	postInfo.StartPoll(context.Config.Rpc.String())

	err = session.Save(r, w)
	if err != nil {
		context.Logger.Warning("Could not save session")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
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
func newDeviceInfo(pi *Pinger.MailPingInformation) (*Pinger.DeviceInfo, error) {
	var di *Pinger.DeviceInfo
	var err error
	di, err = Pinger.NewDeviceInfo(
		pi.ClientId,
		pi.PushToken,
		pi.PushService,
		pi.Platform)
	if err != nil {
		return nil, err
	}
	if di == nil {
		return nil, errors.New("Could not create DeviceInfo")
	}
	return di, nil
}

func updateDeviceInfo(di *Pinger.DeviceInfo, pi *Pinger.MailPingInformation) (bool, error) {
	changed := false
	if di.ClientId != pi.ClientId {
		panic("Can not have a different ClientID")
	}
	if di.Platform != pi.Platform {
		di.Platform = pi.Platform
		changed = true
	}
	if di.PushService != pi.PushService {
		di.PushService = pi.PushService
		changed = true
	}
	if di.PushToken != pi.PushToken {
		di.PushToken = pi.PushToken
		changed = true
	}
	return changed, nil
}

func saveDeviceInfo(context *Context, pi *Pinger.MailPingInformation) error {
	var err error
	di, err := Pinger.GetDeviceInfo(context.Dbm, pi.ClientId)
	if err != nil {
		return err
	}
	if di == nil {
		di, err = newDeviceInfo(pi)
		if err != nil {
			return err
		}
		err = context.Dbm.Insert(di)
		if err != nil {
			return err
		}
		context.Logger.Debug("Created new entry for %s", pi.ClientId)
	} else {
		changed, err := updateDeviceInfo(di, pi)
		if err != nil {
			return err
		}
		if changed {
			n, err := context.Dbm.Update(di)
			if err != nil {
				return err
			}
			if n > 0 {
				context.Logger.Debug("Updated %s (%d rows)", pi.ClientId, n)
			}
		} else {
			context.Logger.Debug("No change from %s. No DB action take.", pi.ClientId)
		}
	}
	pi.SetDeviceInfo(di)
	return nil
}
