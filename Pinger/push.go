package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/nachocove/Pinger/Utils/Telemetry"
	"time"
)

var APNSMessageTooLarge error
var APNSInvalidToken error

func init() {
	APNSMessageTooLarge = fmt.Errorf("APNS message exceeds 256 bytes")
	APNSInvalidToken = fmt.Errorf("APNS message used an invalid token")
}

func Push(service, token, alert, sound string, contentAvailable int, ttl int64, pingerMap map[string]interface{}, logger *Logging.Logger) error {
	var err error
	retryInterval := time.Duration(1) * time.Second
	for i := 0; i < 10; i++ {
		switch service {
		case PushServiceAPNS:
			if globals.config.APNSCertFile == "" || globals.config.APNSKeyFile == "" {
				return fmt.Errorf("No APNS credentials configured")
			}
			err = APNSpushMessage(token, alert, sound, contentAvailable, ttl, pingerMap, logger)
			if err != nil {
				// TODO: if the error is APNSMessageTooLarge, then split up the message if possible and try again
				if err == APNSInvalidToken {
					return err
				} else if err != APNSMessageTooLarge {
					logger.Warning("Push error %s. Retrying attempt %d in %s", err, i, retryInterval)
					time.Sleep(retryInterval)
				}
			} else {
				logger.Debug("Successfully pushed after %d retries", i)
				break
			}
		}
	}
	return err
}

// AWS Push message code

func pingerPushMessageMapV1(message PingerNotification, contextIds []string, sessionId string) map[string]interface{} {
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

type sessionContextMessage struct {
	message PingerNotification
	context string
	session string
}

func newSessionContextMessage(message PingerNotification, context, session string) *sessionContextMessage {
	return &sessionContextMessage{message, context, session}
}

func pingerPushMessageMapV2(contexts [](*sessionContextMessage)) map[string]interface{} {
	//"contexts": {"context1": { "command": "new" | "register", "session": "abc123"},  ... ]\}
	//"metadata": {"timestamp": "2015-04-10T09:30:00Z, ...}
	pingerMap := make(map[string]interface{})
	metadataMap := make(map[string]string)
	metadataMap["time"] = fmt.Sprintf("%d", time.Now().UTC().Unix())
	pingerMap["meta"] = metadataMap

	if len(contexts) > 0 {
		contextsMap := make(map[string]map[string]string)
		for _, context := range contexts {
			ctxMap := make(map[string]string)
			ctxMap["cmd"] = string(context.message)
			if context.session != "" {
				ctxMap["ses"] = context.session
			}
			contextsMap[context.context] = ctxMap
		}
		pingerMap["ctxs"] = contextsMap
	}
	return pingerMap
}

func alertAllDevices(db DBHandler, logger *Logging.Logger) int {
	var alert string
	if globals.config.APNSAlert {
		alert = "Nacho says: Reregister!"
	}
	h := newDeviceInfoDbHandler(db)
	servicesAndTokens, err := h.distinctPushServiceTokens(pingerHostId)
	if err != nil {
		panic(err)
	}
	count := 0
	pushesSent := 0
	for _, serviceAndToken := range servicesAndTokens {
		contexts, err := h.clientContexts(serviceAndToken.PushService, serviceAndToken.PushToken)
		if err != nil {
			panic(err)
		}
		sessionContexts := make([](*sessionContextMessage), 0, 5)
		for _, c := range contexts {
			sessionContexts = append(sessionContexts, newSessionContextMessage(PingerNotificationRegister, c, ""))
		}
		pingerMap := pingerPushMessageMapV2(sessionContexts)
		err = Push(serviceAndToken.PushService, serviceAndToken.PushToken,
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
