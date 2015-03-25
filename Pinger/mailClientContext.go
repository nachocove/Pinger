package Pinger

import (
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"sync"
)

type MailClientContextType interface {
	stop() error
	validateStopToken(token string) bool
	deferPoll(timeout int64) error
	updateLastContact() error
	Status() (MailClientStatus, error)
	Action(action PingerCommand) error
	getStopToken() string
	getSessionInfo() (*ClientSessionInfo, error)
}

type MailClientContext struct {
	MailClientContextType
	mailClient MailClient // a mail client with the MailClient interface
	stopToken  string
	logger     *Logging.Logger
	errCh      chan error
	stopAllCh  chan int // closed when client is exiting, so that any sub-routine can exit
	exitCh     chan int // used by MailClient{} to signal it has exited
	command    chan PingerCommand
	lastError  error
	stats      *Utils.StatLogger
	di         *DeviceInfo
	pi         *MailPingInformation
	wg         sync.WaitGroup
	status     MailClientStatus
}
