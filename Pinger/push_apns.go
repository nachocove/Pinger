package Pinger

import (
	"fmt"
	"github.com/anachronistic/apns"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"time"
)

const (
	PushServiceAPNS = "APNS"
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
		time.Sleep(time.Duration(globals.config.APNSFeedbackPeriod) * time.Minute)
		logger.Debug("APNS FEEDBACK: Checking feedback service")
		client := apns.NewClient(apnsHost, globals.config.APNSCertFile, globals.config.APNSKeyFile)
		go client.ListenForFeedback()

		for {
			select {
			case resp := <-apns.FeedbackChannel:
				logger.Warning("APNS FEEDBACK recv'd:", resp.DeviceToken)
			case <-apns.ShutdownChannel:
				logger.Debug("APNS FEEDBACK nothing returned from the feedback service")
			}
		}
	}
}

func APNSpushMessage(token string, alert, sound string, contentAvailable int, ttl int64, pingerMap map[string]interface{}, logger *Logging.Logger) error {
	if globals.config.APNSCertFile == "" {
		panic("No apns cert set. Can not push to APNS")
	}
	if globals.config.APNSKeyFile == "" {
		panic("No apns key set. Can not push to APNS")
	}
	pn := apns.NewPushNotification()
	token, err := AWS.DecodeAPNSPushToken(token)
	if err != nil {
		return err
	}
	pn.DeviceToken = token
	pn.Priority = 10
	if ttl > 0 {
		expiration := time.Now().Add(time.Duration(ttl) * time.Second).UTC()
		logger.Debug("Setting push expiration to %s (unix utc %d)", expiration, expiration.Unix())
		pn.Expiry = uint32(expiration.UTC().Unix())
	}

	payload := make(map[string]interface{})
	if alert != "" {
		payload["alert"] = alert
	}
	if sound != "" {
		payload["sound"] = sound
	}
	if contentAvailable > 0 {
		payload["content-available"] = contentAvailable
	}

	pn.Set("aps", payload)

	pn.Set("pinger", pingerMap)

	msg, _ := pn.PayloadString()
	if len(msg) >= 256 {
		logger.Error("Push message to APNS exceeds 256 bytes: pushToken: %s %s", token, msg)
		return APNSMessageTooLarge
	}
	logger.Debug("Sending push message to APNS: pushToken: %s %s", token, msg)

	var apnsHost string
	if globals.config.APNSSandbox {
		apnsHost = APNSSandboxServer
	} else {
		apnsHost = APNSServer
	}
	client := apns.NewClient(apnsHost, globals.config.APNSCertFile, globals.config.APNSKeyFile)
	resp := client.Send(pn)
	if resp.AppleResponse != "" {
		logger.Debug("Response from apple: %s", resp.AppleResponse)
	}
	if resp.Error != nil {
		if resp.Error.Error () == "INVALID_TOKEN" {
            return APNSInvalidToken
        } else {
		    return fmt.Errorf("APNS Push error: %s", resp.Error)
        }
	}
	if !resp.Success {
		logger.Error("response is not success, but no error indicated")
		return fmt.Errorf("Unknown error occurred during push")
	}
	return nil
}
