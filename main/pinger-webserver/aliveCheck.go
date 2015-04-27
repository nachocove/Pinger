package main

import (
	"encoding/json"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"net"
	"net/http"
	"strings"
)

func init() {
	httpsRouter.HandleFunc("/1/alive", aliveCheck)
}

func aliveCheck(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	if r.Method != "GET" {
		context.Logger.Warning("Received %s method call from %s", r.Method, r.RemoteAddr)
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}

	rIp := r.Header.Get("X-Forwarded-For")
	if rIp == "" {
		rIp = r.RemoteAddr
	}
	ipParts := strings.Split(rIp, ":")
	if len(ipParts) < 1 {
		context.Logger.Error("Could not split remote address %s", r.RemoteAddr)
		http.Error(w, "INTERNAL ERROR", http.StatusInternalServerError)
		return
	}
	remoteIP := net.ParseIP(ipParts[0])
	if remoteIP == nil {
		context.Logger.Error("Could not parse remote address %s", r.RemoteAddr)
		http.Error(w, "INTERNAL ERROR", http.StatusInternalServerError)
		return
	}
	err := r.ParseForm()
	if err != nil {
		context.Logger.Warning("Could not parse form")
		http.Error(w, "INTERNAL ERROR", http.StatusInternalServerError)
		return
	}
	token := r.FormValue("Token")
	if token == "" {
		token = r.FormValue("token")
	}
	if token == "" {
		context.Logger.Warning("No token provided")
		http.Error(w, "NO TOKEN", http.StatusForbidden)
		return
	}
	if !context.Config.Server.CheckToken(token) {
		context.Logger.Error("tokens do not match")
		http.Error(w, "TOKEN MISMATCH", http.StatusForbidden)
		return
	}
	if !context.Config.Server.CheckIP(remoteIP) {
		context.Logger.Error("remote address did not match any valid IPrange from the list %s", context.Config.Server.CheckIPListString())
		http.Error(w, "BAD IP", http.StatusForbidden)
		return
	}
	reply, err := Pinger.AliveCheck(&context.Config.Rpc)
	if err != nil {
		context.Logger.Warning("Could not check for aliveness: %v", err)
		responseError(w, RPCServerError, "")
		return
	}
	responseData := make(map[string]string)

	switch {
	case reply.Code == Pinger.PollingReplyError:
		http.Error(w, reply.Message, http.StatusBadRequest)
		return

	case reply.Code == Pinger.PollingReplyOK:
		responseData["Status"] = "OK"
		responseData["Message"] = ""

	case reply.Code == Pinger.PollingReplyWarn:
		responseData["Status"] = "WARN"
		responseData["Message"] = reply.Message

	default:
		context.Logger.Error("Unknown PollingReply Code %d", reply.Code)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		context.Logger.Warning("Could not json encode reply: %v", responseData)
		responseError(w, JSONEncodeError, "")
		return
	}
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(responseJson))
	return
}
