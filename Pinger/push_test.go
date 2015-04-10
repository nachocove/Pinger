package Pinger

import (
	"encoding/json"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"testing"
	"fmt"
)

type pushTester struct {
	suite.Suite
	logger *Logging.Logger
}

func (s *pushTester) SetupSuite() {
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
}
func (s *pushTester) SetupTest() {
}
func (s *pushTester) TearDownTest() {
}

func TestPushMessages(t *testing.T) {
	s := new(pushTester)
	suite.Run(t, s)
}

func (s *deviceInfoTester) TestIOSDevicePushMessageCreate() {
	var days_28 int64 = 2419200

	contexts := []string{"context1234567"}
	sessionId := "sessionid1234"
	pingerMessage := pingerPushMessageMap(PingerNotificationRegister, contexts, sessionId)
	s.NotEmpty(pingerMessage)
	s.Equal(sessionId, pingerMessage["session"])
	s.NotEqual("", pingerMessage["timestamp"])
	for _, c := range contexts {
		_, ok := pingerMessage[c]
		s.True(ok, fmt.Sprintf("Context %s not in pinger message", c))
	}
	s.Equal(len(contexts)+2, len(pingerMessage))

	platform := "ios"
	alert := "foo"
	sound := "bar"
	contentAvailable := 1
	
	message, err := awsPushMessageString(platform, alert, sound, contentAvailable, days_28, pingerMessage, s.logger)
	s.NoError(err)
	s.NotEmpty(message)

	pushMessage := make(map[string]string)
	err = json.Unmarshal([]byte(message), &pushMessage)
	s.NoError(err)

	sections := []string{"APNS", "APNS_SANDBOX", "default"}
	for _, sec := range sections {
		secStr, ok := pushMessage[sec]
		s.True(ok, sec)
		s.NotEqual("", secStr)
		secMap := make(map[string]interface{})
		err := json.Unmarshal([]byte(secStr), &secMap)
		s.NoError(err)
		s.NotEmpty(secMap)
	}
}

func (s *deviceInfoTester) TestAndroidDevicePushMessageCreate() {
	var days_28 int64 = 2419200

	contexts := []string{"context1234567"}
	sessionId := "sessionid1234"
	pingerMessage := pingerPushMessageMap(PingerNotificationRegister, contexts, sessionId)
	s.NotEmpty(pingerMessage)
	s.Equal(sessionId, pingerMessage["session"])
	s.NotEqual("", pingerMessage["timestamp"])
	for _, c := range contexts {
		_, ok := pingerMessage[c]
		s.True(ok, fmt.Sprintf("Context %s not in pinger message", c))
	}
	s.Equal(len(contexts)+2, len(pingerMessage))

	platform := "android"
	alert := "foo"
	sound := "bar"
	contentAvailable := 1
	
	message, err := awsPushMessageString(platform, alert, sound, contentAvailable, days_28, pingerMessage, s.logger)
	s.NoError(err)
	s.NotEmpty(message)

	pushMessage := make(map[string]string)
	err = json.Unmarshal([]byte(message), &pushMessage)
	s.NoError(err)

	sections := []string{"default", "GCM"}
	for _, sec := range sections {
		secStr, ok := pushMessage[sec]
		s.True(ok, sec)
		s.NotEqual("", secStr)
		secMap := make(map[string]interface{})
		err := json.Unmarshal([]byte(secStr), &secMap)
		s.NoError(err)
		s.NotEmpty(secMap)
	}
}

