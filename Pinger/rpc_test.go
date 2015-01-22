package Pinger

import (
	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
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

func (di *DeviceInfo) NewMailServer(hostname string, port, pingPeriodicity int, ssl, debug bool, logger *logging.Logger) MailServer {
	fmt.Printf("Called NewMailServer\n")
	return NewDummyMailServer()
}

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
