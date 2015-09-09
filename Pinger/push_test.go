package Pinger

import (
	"encoding/json"
	"fmt"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
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

func (s *pushTester) TestDevicePushMessageCreateV2() {
	var days_28 int64 = 2419200
	context := "context1234567"
	ctxtMessage := newContextMessage(PingerNotificationRegister, context)
	pingerMessage := pingerPushMessageMapV2([](*contextMessage){ctxtMessage})
	s.NotEmpty(pingerMessage)
	_, ok := pingerMessage["meta"]
	require.True(s.T(), ok, "meta not in pinger message")
	meta := pingerMessage["meta"].(map[string]string)
	t, ok := meta["time"]
	s.True(ok, "time not in pinger message['meta']")
	s.NotEqual("", t)

	_, ok = pingerMessage["ctxs"]
	s.True(ok, "ctxs not in pinger message")
	ctxs := pingerMessage["ctxs"].(map[string]map[string]string)
	_, ok = ctxs[context]
	s.True(ok, fmt.Sprintf("context %s not in pinger message['ctxs']", context))

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

	platform = "android"
	message, err = awsPushMessageString(platform, alert, sound, contentAvailable, days_28, pingerMessage, s.logger)
	s.NoError(err)
	s.NotEmpty(message)

	pushMessage = make(map[string]string)
	err = json.Unmarshal([]byte(message), &pushMessage)
	s.NoError(err)

	sections = []string{"default", "GCM"}
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
