package Pinger

import (
	"fmt"
	"github.com/anachronistic/apns"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"time"
	"github.com/nachocove/Pinger/Utils/AWS"
)

const (
	APNSServer = "gateway.push.apple.com:2195"
	APNSSandboxServer = "gateway.sandbox.push.apple.com:2195"
)

var apnsCert string
var apnsKey string

func (di *DeviceInfo) APNSpushMessage(message PingerNotification) error {
	if globals.config.APNSCertFile == "" {
		panic("No apns cert set. Can not push to APNS")
	}
	if globals.config.APNSKeyFile == "" {
		panic("No apns key set. Can not push to APNS")
	}
	pn := apns.NewPushNotification()
	token, err := AWS.DecodeAPNSPushToken(di.PushToken)
	if err != nil {
		return err
	}
	pn.DeviceToken = token
	pn.Priority = 10

	payload := make(map[string]interface{})
	if message == "new" {
		payload["Alert"] = "Yo! You got mail!"
	} else {
		payload["Alert"] = "Yo! You need to re-register!"
	}
	payload["Sound"] = "silent.wav"
	payload["ContentAvailable"] = 1

	pn.Set("aps", payload)

	pingerMap := make(map[string]interface{})
	pingerMap[di.ClientContext] = string(message)
	pingerMap["timestamp"] = time.Now().UTC().Round(time.Millisecond).Format(Telemetry.TelemetryTimeZFormat)
	pingerMap["session"] = di.sessionId
	pn.Set("pinger", pingerMap)

	msg, _ := pn.PayloadString()
	di.Debug("Push Message to APNS/%s: %s", token, msg)

	var apnsHost string
	if globals.config.APNSSandbox {
		apnsHost = APNSSandboxServer
	} else {
		apnsHost = APNSServer
	}
	client := apns.NewClient(apnsHost, globals.config.APNSCertFile, globals.config.APNSKeyFile)
	resp := client.Send(pn)
	di.Debug("Response from apple: %s", resp.AppleResponse)
	if resp.Error != nil {
		return fmt.Errorf("APNS Push error: %s", resp.Error)
	}
	if !resp.Success {
		di.Error("response is not success, but no error indicated")
		return fmt.Errorf("Unknown error occurred during push")
	}
	return nil
}
