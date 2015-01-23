package main

import (
	"encoding/json"
	"net/http"
	"fmt"

	"github.com/nachocove/Pinger/Pinger"
)

func init() {
	httpsRouter.HandleFunc("/register", registerDevice)
	httpsRouter.HandleFunc("/defer", deferPolling)
}

const SessionVarClientId = "ClientId"

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

	session.Values[SessionVarClientId] = postInfo.ClientId

	token, err := Pinger.StartPoll(context.RpcConnectString, &postInfo)
	if err != nil {
		context.Logger.Warning("Could not re/start polling for device %s: %s", postInfo.ClientId, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	context.Logger.Debug("Re/Started Polling for %s", postInfo.ClientId)

	err = session.Save(r, w)
	if err != nil {
		context.Logger.Warning("Could not save session")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	responseData := make(map[string]string)
	responseData["Token"] = token
	responseData["Status"] = "OK"
	responseData["Message"] = ""
	
	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("Could not json encode reply: %v", responseData)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}

type deferPost struct {
	ClientId string
	StopToken string
}

func deferPolling(w http.ResponseWriter, r *http.Request) {
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
	
	deferData := deferPost{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&deferData)
	if err != nil {
		context.Logger.Error("Could not parse json %s", err)
		http.Error(w, "Could not parse json", http.StatusBadRequest)
		return
	}
	if session.Values[SessionVarClientId] != deferData.ClientId {
		context.Logger.Error("Client ID %s does not match session", deferData.ClientId)
		http.Error(w, "Unknown Client ID", http.StatusForbidden)
		return		
	}
	err = Pinger.DeferPoll(context.RpcConnectString, deferData.ClientId, deferData.StopToken)
	if err != nil {
		context.Logger.Error("Error deferring poll %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responseData := make(map[string]string)
	responseData["Status"] = "OK"
	responseData["Message"] = ""
	
	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("Could not json encode reply: %v", responseData)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}
