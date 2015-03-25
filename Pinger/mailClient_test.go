package Pinger

import (
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testingMailClientContext struct {
	MailClientContextType
	logger *Logging.Logger
}

func (client *testingMailClientContext) stop() error {
	client.logger.Debug("stop")
	return nil
}
func (client *testingMailClientContext) validateStopToken(token string) bool {
	client.logger.Debug("validateStopToken")
	return true
}
func (client *testingMailClientContext) deferPoll(timeout int64) error {
	client.logger.Debug("deferPoll")
	return nil
}
func (client *testingMailClientContext) updateLastContact() error {
	client.logger.Debug("updateLastContact")
	return nil
}
func (client *testingMailClientContext) Status() (MailClientStatus, error) {
	client.logger.Debug("Status")
	return MailClientStatusPinging, nil
}
func (client *testingMailClientContext) Action(action PingerCommand) error {
	client.logger.Debug("Action")
	return nil
}
func (client *testingMailClientContext) getStopToken() string {
	client.logger.Debug("getStopToken")
	return "1234"
}
func (client *testingMailClientContext) getSessionInfo() (*ClientSessionInfo, error) {
	client.logger.Debug("getSessionInfo")
	return nil, nil
}

func TestMailClient(t *testing.T) {
	assert := assert.New(t)
	logger := Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	assert.NotNil(logger)
}
