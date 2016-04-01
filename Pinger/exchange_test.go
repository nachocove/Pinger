package Pinger

import (
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
)

type exchangeTester struct {
	suite.Suite
	logger *Logging.Logger
	dbmap  *gorp.DbMap
}

func (s *exchangeTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbmap, err = initDB(&dbconfig, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
}

func (s *exchangeTester) SetupTest() {
}

func (s *exchangeTester) TearDownTest() {
}

func TestExchange(t *testing.T) {
	s := new(exchangeTester)
	suite.Run(t, s)
}

func (s *exchangeTester) TestNewExchangeClient() {
	pi := &MailPingInformation{}
	wg := &sync.WaitGroup{}
	debug := true

	ex, err := NewExchangeClient(pi, wg, debug, s.logger)
	s.NoError(err)
	s.NotNil(ex)

	s.NotNil(ex.pi)
	s.Equal(pi, ex.pi)
	s.Equal(debug, ex.debug)
	s.NotEqual(s.logger, ex.logger) // NewExchangeClient makes a copy
}

func (s *exchangeTester) TestUserRedaction() {
	urls := make(map[string]string)
	urls["https://mail.enel.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=someUser@company.fr&DeviceId=Ncho4B1AE6BFX5E1BX46A1XB3C7B&DeviceType=iPhone"] = "https://mail.enel.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=<redacted>&DeviceId=Ncho4B1AE6BFX5E1BX46A1XB3C7B&DeviceType=iPhone"
	urls["https://mail.enel.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=otherDomain\\user001&DeviceId=NchoA2CE1911X846CX49C3X83C7B&DeviceType=iPad"] = "https://mail.enel.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=<redacted>&DeviceId=NchoA2CE1911X846CX49C3X83C7B&DeviceType=iPad"
	urls["https://mail.aspirezone.net/Microsoft-Server-ActiveSync?Cmd=Ping&User=myDomain\\userid&DeviceId=NchoFFCB24ABX741CX46BBXA3C7B&DeviceType=iPhone"] = "https://mail.aspirezone.net/Microsoft-Server-ActiveSync?Cmd=Ping&User=<redacted>&DeviceId=NchoFFCB24ABX741CX46BBXA3C7B&DeviceType=iPhone"
	urls["https://mobilesync.level3.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=some.company.com.foo\\first.last&DeviceId=Ncho2615D601X244EX4B0EX93C7B&DeviceType=iPad"] = "https://mobilesync.level3.com/Microsoft-Server-ActiveSync?Cmd=Ping&User=<redacted>&DeviceId=Ncho2615D601X244EX4B0EX93C7B&DeviceType=iPad"

	for key, value := range urls {
		redacted := RedactEmailFromError(key)
		s.Equal(value, redacted)
	}
}
