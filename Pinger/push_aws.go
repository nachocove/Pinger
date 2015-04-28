package Pinger

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"regexp"
)

var alreadyRegisted *regexp.Regexp

func init() {
	alreadyRegisted = regexp.MustCompile("^.*Endpoint (?P<arn>arn:aws:sns:[^ ]+) already exists.*$")
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
			logger.Error("Length of push message is %d > 256", len(b))
			return "", APNSMessageTooLarge
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

func registerAws(aws AWS.AWSHandler, pushService, pushToken, customerData string, logger *Logging.Logger) error {
	var err error
	var attributes map[string]string
	need_attr_update := false

	switch pushService {
	case PushServiceAPNS:
		pushToken, err = AWS.DecodeAPNSPushToken(pushToken)
		if err != nil {
			return err
		}
		if len(pushToken) != 64 {
			return fmt.Errorf("APNS token length wrong. %d ('%s')", len(pushToken), string(pushToken))
		}

	default:
		return fmt.Errorf("Unsupported push service %s:%s", pushService, pushToken)
	}

	logger.Debug("Registering %s:%s with AWS.", pushService, pushToken)
	arn, registerErr := aws.RegisterEndpointArn(pushService, pushToken, customerData)
	if registerErr != nil {
		if alreadyRegisted.MatchString(registerErr.Error()) == true {
			replaceString := fmt.Sprintf("${%s}", alreadyRegisted.SubexpNames()[1])
			arn = alreadyRegisted.ReplaceAllString(registerErr.Error(), replaceString)
			logger.Warning("Previously registered as %s. Updating.", arn)
		} else {
			return registerErr
		}
	} else {
		logger.Debug("endpoint created %s", arn)
	}

	// fetch the attributes
	logger.Debug("fetching attributes for %s.", arn)
	attributes, err = aws.GetEndpointAttributes(arn)
	if err != nil {
		return err
	}
	if attributes == nil {
		panic("attributes should not be nil")
	}
	enabled, ok := attributes["Enabled"]
	if !ok || enabled != "true" {
		if enabled != "true" {
			logger.Warning("AWS has endpoint disabled. Reenabling it")
			attributes["Enabled"] = "true"
			need_attr_update = true
		}
	}

	if attributes["Token"] == "" || (pushToken != "" && pushToken != attributes["Token"]) {
		// need to update the token with aws
		attributes["Token"] = pushToken
		need_attr_update = true
	}

	if customerData != attributes["CustomUserData"] {
		attributes["CustomUserData"] = customerData
		need_attr_update = true
	}
	if need_attr_update {
		logger.Debug("Setting new attributes for %s: %+v", arn, attributes)
		err := aws.SetEndpointAttributes(arn, attributes)
		if err != nil {
			logger.Debug("Could not set attributes")
			return err
		}
	}
	return nil
}

func validateAwsClient(aws AWS.AWSHandler, pushService, pushToken, customerData string, logger *Logging.Logger) error {
	switch pushService {
	case PushServiceAPNS:
		// TODO Can we cache the validation results here? Can they change once a client ID has been invalidated? How do we even invalidate one?
		err := registerAws(aws, pushService, pushToken, customerData, logger)
		if err != nil {
			if aws.IgnorePushFailures() == false {
				return err
			} else {
				logger.Warning("Registering %s:%s error (ignored): %s", pushService, pushToken, err)
			}
		}
	}
	return nil
}
