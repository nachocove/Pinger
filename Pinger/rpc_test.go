package Pinger

import (
	"fmt"
	logging "github.com/nachocove/Pinger/Pinger/logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRpcStart(t *testing.T) {
	assert := assert.New(t)
	logger := logging.InitLogging("unittest", "", logging.DEBUG, true, logging.DEBUG, true)

	config := NewConfiguration()
	config.Db.Type = "sqlite"
	config.Db.Filename = ":memory:"

	poll, err := NewBackendPolling(config, true, logger)
	assert.Nil(err, "err should be nil")

	mailInfo := &MailPingInformation{}
	args := StartPollArgs{
		MailInfo: mailInfo,
	}
	reply := StartPollingResponse{}

	err = poll.Start(&args, &reply)
	assert.NotNil(err, fmt.Sprintf("Did not get error from poll.Start. DeviceInfo save should have failed"))
}
