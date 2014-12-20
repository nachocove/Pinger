package main

import (
	"github.com/nachocove/Pinger/WebServer"
)

/*
 Design/Architecture

 Web-interface is used by clients to upload credentials and other data needed to monitor mail

 Web-interface feeds various channels, one per push-service (APNS, GCM, etc)

 If not go-routine for a service exists, one is spawned. Each go-routine handles X outbound connections
 to amortize the amount of memory needed for each go-routine.
 TODO: Need to find the sweet-spot for outbound connections per routine
*/

func main() {
	WebServer.GetConfigAndRun()
}
