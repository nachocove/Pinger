package Pinger

import (
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRpcSstart(t *testing.T) {
	logger, err := logging.GetLogger("Unittests")
	assert.Nil(t, err)
	InitRpc(logger)
	di, err := NewDeviceInfo(
		"1234567",
		"MyDeviceID",
		"SomePushToken",
		"AWS",
		"ios",
		"exchange")
	assert.Nil(t, err)
	assert.NotNil(t, di)

	args := StartPollArgs{
		Device:       di,
		MailEndpoint: "",
	}
	reply := PollingResponse{}

	poll := new(BackendPolling)
	poll.start(&args, &reply)
}
