package Pinger

import (
	"fmt"
	"github.com/anachronistic/apns"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"time"
)

const (
	APNSServer                = "gateway.push.apple.com:2195"
	APNSFeedbackServer        = "feedback.push.apple.com:2196"
	APNSSandboxServer         = "gateway.sandbox.push.apple.com:2195"
	APNSSandboxFeedbackServer = "feedback.sandbox.push.apple.com:2196"
)

func FeedbackListener(logger *Logging.Logger) {
	if globals.config.APNSCertFile == "" {
		return
	}
	if globals.config.APNSKeyFile == "" {
		return
	}
	var apnsHost string
	if globals.config.APNSSandbox {
		apnsHost = APNSSandboxFeedbackServer
	} else {
		apnsHost = APNSFeedbackServer
	}
	for {
		time.Sleep(time.Duration(1) * time.Minute)
		logger.Debug("APNS FEEDBACK: Checking feedback service")
		client := apns.NewClient(apnsHost, globals.config.APNSCertFile, globals.config.APNSKeyFile)
		go client.ListenForFeedback()

		for {
			select {
			case resp := <-apns.FeedbackChannel:
				logger.Warning("APNS FEEDBACK recv'd:", resp.DeviceToken)
			case <-apns.ShutdownChannel:
				logger.Info("APNS FEEDBACK nothing returned from the feedback service")
			}
		}
	}
}

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
		payload["alert"] = "Yo! Jan says you got mail!"
	} else {
		payload["alert"] = "That would be in the butt, bob!"
	}
	payload["sound"] = "silent.wav"
	payload["content-available"] = 1

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
	if resp.AppleResponse != "" {
		di.Debug("Response from apple: %s", resp.AppleResponse)
	}
	if resp.Error != nil {
		return fmt.Errorf("APNS Push error: %s", resp.Error)
	}
	if !resp.Success {
		di.Error("response is not success, but no error indicated")
		return fmt.Errorf("Unknown error occurred during push")
	}
	return nil
}
