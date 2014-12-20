package WebServer

import (
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
)

func init() {
    router.HandleFunc("/register/{deviceid}/{platform:ios|android}", registerDevice)	
}

// TODO Need to figure out Auth
func registerDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "UNKNOWN METHOD", http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	deviceid := vars["deviceid"]
	platform := vars["platform"]
	
	// This is where we would save the device information (using a goroutine) and 
	// set up pinging of the device's mail server.
	fmt.Println(deviceid, platform)
	return
}
