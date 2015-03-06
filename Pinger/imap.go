package Pinger

import (
	"fmt"
	"github.com/op/go-logging"
)

type IMAPClient struct {
	debug      bool
	logger     *logging.Logger
	parent     *MailClientContext	
}

func (imap *IMAPClient) getLogPrefix() (prefix string) {
	if imap.parent != nil && imap.parent.di != nil {
		prefix = imap.parent.di.getLogPrefix()
	}
	return
}

func NewIMAPClient(parent *MailClientContext, debug bool, logger *logging.Logger) (*IMAPClient, error) {
	imap := IMAPClient{
		debug:     debug,
		logger:    logger,
		parent:    parent,		
	}
	return &imap, nil
}

func (imap *IMAPClient) LongPoll(stopCh, exitCh chan int) {
	defer recoverCrash(imap.logger)
	defer func() {
		imap.logger.Debug("%s: Stopping", imap.getLogPrefix())
		exitCh <- 1 // tell the parent we've exited.
	}()
	
	imap.parent.sendError(fmt.Errorf("Not implemented"))
}

func (imap *IMAPClient) Cleanup() {
	imap.parent = nil
}