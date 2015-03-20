package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRpcStart(t *testing.T) {
	assert := assert.New(t)
	logger := Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, true)

	config := NewConfiguration()
	config.Db.Type = "sqlite"
	config.Db.Filename = ":memory:"

	poll, err := NewBackendPolling(config, true, false, logger)
	assert.Nil(err, "err should be nil")

	mailInfo := &MailPingInformation{}
	args := StartPollArgs{
		MailInfo: mailInfo,
	}
	reply := StartPollingResponse{}

	err = poll.Start(&args, &reply)
	assert.NotNil(err, fmt.Sprintf("Did not get error from poll.Start. DeviceInfo save should have failed"))
}
