package Pinger

import (
	"sync"
	"testing"
	"github.com/op/go-logging"
  "github.com/stretchr/testify/assert"
)

type DummyMailServer struct {
}

func NewDummyMailServer() *DummyMailServer {
	return &DummyMailServer{}
}
func (d *DummyMailServer) Listen(wait *sync.WaitGroup) error {
	return nil
}
func (d *DummyMailServer) Action(action int) error {
	return nil
}

func TestRpcSstart(t *testing.T) {
	logger, err := logging.GetLogger("Unittests")
	assert.Nil(t, err)
	InitPolling(logger)
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
