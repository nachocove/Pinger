package Pinger

import "sync"

// MailServerType the type of the mail server
type MailServerType int

const (
	// MailServerUnknown an unknown mail server
	MailServerUnknown MailServerType = iota
	// MailServerExchange Exchange by Microsoft
	MailServerExchange MailServerType = iota
	// MailServerHotmail hosted hotmail domain
	MailServerHotmail MailServerType = iota
)

var mailServers = [...]string{
	"UNKNOWN",
	"EXCHANGE",
	"HOTMAIL",
}

func (mailServer MailServerType) String() string {
	return mailServers[mailServer]
}

type MailServer interface {
	Listen(wait *sync.WaitGroup) error
	Action(action int) error
}
