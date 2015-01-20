package Pinger

type pollMapItem struct {
	startArgs  *StartPollArgs
	mailServer MailServer
}

var pollMap map[string]*pollMapItem
