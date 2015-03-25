package Pinger

import (
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/suite"
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
	s.dbmap, err = initDB(&dbconfig, true, true, s.logger)
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
	parent := &MailClientContext{}
	debug := true

	ex, err := NewExchangeClient(parent, debug, s.logger)
	s.NoError(err)
	s.NotNil(ex)

	s.NotNil(ex.parent)
	s.Equal(parent, ex.parent)
	s.Equal(debug, ex.debug)
	s.NotEqual(s.logger, ex.logger)
}
