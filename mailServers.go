package Pinger

import (

)

type MailServerType int
const (
	MAILSERVER_UNKNOWN MailServerType = iota
 	MAILSERVER_EXCHANGE MailServerType = iota
 	MAILSERVER_HOTMAIL MailServerType = iota 
)

var mailServers = [...]string {
 "UNKNOWN",
 "EXCHANGE",
 "HOTMAIL",
}

func (mailServer MailServerType) String() string {
 return mailServers[mailServer]
}
