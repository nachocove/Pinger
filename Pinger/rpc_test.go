package Pinger

import (
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRpcStart(t *testing.T) {
	assert := assert.New(t)
	logger := Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)

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
	assert.NoError(err)
	assert.Equal(PollingReplyError, reply.Code)
	assert.Empty(reply.Token)
	assert.NotEmpty(reply.Message)
}
