package Pinger

import ()

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