package Pinger

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"strings"
	"time"
)

func Push(aws AWS.AWSHandler, platform, service, token, endpointArn, alert, sound string, contentAvailable int, ttl int64, pingerMap map[string]interface{}, logger *Logging.Logger) error {
	var err error
	retryInterval := time.Duration(1) * time.Second
	for i := 0; i < 10; i++ {
		if strings.EqualFold(service, PushServiceAPNS) == false || globals.config.APNSCertFile == "" || globals.config.APNSKeyFile == "" {
			if endpointArn == "" {
				return fmt.Errorf("Endpoint not registered: Token ('%s:%s')", service, token)
			}
			pushMessage, err := awsPushMessageString(
				platform, alert, sound, contentAvailable, ttl, pingerMap, logger)
			if err != nil {
				return err
			}
			logger.Debug("Sending push message to AWS: pushToken: %s/%s AWSEndpointArn:%s %s", service, token, endpointArn, pushMessage)
			err = aws.SendPushNotification(endpointArn, pushMessage)
		} else {
			err = APNSpushMessage(token, alert, sound, contentAvailable, ttl, pingerMap, logger)
		}
		if err != nil {
			logger.Warning("Push error %s. Retrying attempt %d in %s", err, i, retryInterval)
			time.Sleep(retryInterval)
		} else {
			logger.Debug("Successfully pushed after %d retries", i)
			break
		}
	}
	return err
}

// AWS Push message code

func pingerPushMessageMap(message PingerNotification, contextIds []string, sessionId string) map[string]interface{} {
	if message == "" {
		panic("Message can not be empty")
	}
	pingerMap := make(map[string]interface{})
	for _, context := range contextIds {
		pingerMap[context] = string(message)
	}
	pingerMap["timestamp"] = time.Now().UTC().Round(time.Millisecond).Format(Telemetry.TelemetryTimeZFormat)
	if sessionId != "" {
		pingerMap["session"] = sessionId
	}
	return pingerMap
}

func awsPushMessageString(platform, alert, sound string, contentAvailable int, ttl int64, pingerMap map[string]interface{}, logger *Logging.Logger) (string, error) {
	pingerJson, err := json.Marshal(pingerMap)
	if err != nil {
		return "", err
	}
	notificationMap := map[string]string{}
	notificationMap["default"] = string(pingerJson)

	switch {
	case platform == "ios":
		APNSMap := map[string]interface{}{}
		APNSMap["pinger"] = pingerMap
		apsMap := make(map[string]interface{})
		if contentAvailable > 0 {
			apsMap["content-available"] = contentAvailable
		}
		if sound != "" {
			apsMap["sound"] = sound
		}
		if alert != "" {
			apsMap["alert"] = alert
		}
		APNSMap["aps"] = apsMap
		b, err := json.Marshal(APNSMap)
		if err != nil {
			return "", err
		}
		if len(b) > 256 {
			logger.Warning("Length of push message is %d > 256", len(b))
		} else {
			logger.Debug("Length of push message %d", len(b))
		}
		notificationMap["APNS"] = string(b)
		notificationMap["APNS_SANDBOX"] = string(b)

	case platform == "android":
		hash := sha1.New()
		hash.Write(pingerJson)
		md := hash.Sum(nil)
		pingerMapSha := hex.EncodeToString(md)

		GCMMap := map[string]interface{}{}
		GCMMap["data"] = pingerMap
		GCMMap["collapse_key"] = string(pingerMapSha)
		GCMMap["time_to_live"] = ttl
		GCMMap["delay_while_idle"] = false

		b, err := json.Marshal(GCMMap)
		if err != nil {
			return "", err
		}
		notificationMap["GCM"] = string(b)
	}

	var notificationBytes []byte
	notificationBytes, err = json.Marshal(notificationMap)
	if err != nil {
		return "", err
	}
	if len(notificationBytes) == 0 {
		return "", fmt.Errorf("No notificationBytes created")
	}
	return string(notificationBytes), nil
}

func alertAllDevices(dbm *gorp.DbMap, aws AWS.AWSHandler, logger *Logging.Logger) int {
	servicesAndTokens := make([](struct {
		Platform       string `db:"device_platform"` // "ios", "android", etc..
		PushService    string `db:"push_service"`    // APNS, GCM, ...
		PushToken      string `db:"push_token"`
		AWSEndpointArn string `db:"aws_endpoint_arn"`
	}), 0, 10)
	_, err := dbm.Select(&servicesAndTokens, distinctPushServiceTokenSql, pingerHostId)
	if err != nil {
		panic(err)
	}
	var alert string
	if globals.config.APNSAlert {
		alert = "Nacho says: Reregister!"
	}
	count := 0
	pushesSent := 0
	for _, serviceAndToken := range servicesAndTokens {
		contexts := make([]string, 0, 5)
		_, err = dbm.Select(&contexts, clientContextsSql, serviceAndToken.PushService, serviceAndToken.PushToken)
		if err != nil {
			panic(err)
		}
		pingerMap := pingerPushMessageMap(PingerNotificationRegister, contexts, "")
		err = Push(aws, serviceAndToken.Platform, serviceAndToken.PushService, serviceAndToken.PushToken, serviceAndToken.AWSEndpointArn,
			alert, globals.config.APNSSound, globals.config.APNSContentAvailable, globals.config.APNSExpirationSeconds, pingerMap, logger)
		if err != nil {
			logger.Error("Could not send push: %s", err.Error())
		} else {
			pushesSent++
			count++
		}
		if count >= 10 {
			count = 0
			time.Sleep(time.Duration(1) * time.Second)
		}
	}
	return pushesSent
}