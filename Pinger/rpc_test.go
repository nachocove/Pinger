package Pinger

import (
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"testing"
	"fmt"
	"os"
)

const testingDbFilename = "testingDB.db"

func TestRpcSstart(t *testing.T) {
	assert := assert.New(t)
	logger, err := logging.GetLogger("Unittests")
	assert.Nil(err, "err should be nil")
	
	config := NewConfiguration()
	config.Db.Type = "sqlite"
	config.Db.Filename = testingDbFilename
	
	poll, err := NewBackendPolling(config, true, logger)
	assert.Nil(err, "err should be nil")
	defer os.Remove(testingDbFilename)
	
	mailInfo := &MailPingInformation{}	
	args := StartPollArgs{
		MailInfo:       mailInfo,
	}
	reply := StartPollingResponse{}

	poll.Start(&args, &reply)
	assert.Equal(reply.Code, PollingReplyError, fmt.Sprintf("Did not get error from poll.Start. DeviceInfo save should have failed"))
}
