package Pinger

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type ServerConfigTests struct {
	suite.Suite
	cfg *ServerConfiguration
}

func (s *ServerConfigTests) SetupSuite() {
	s.cfg = NewServerConfiguration()
}

func (s *ServerConfigTests) SetupTest() {
}

func (s *ServerConfigTests) TearDownTest() {
}

func TestServerConfiguration(t *testing.T) {
	s := new(ServerConfigTests)
	suite.Run(t, s)
}

func (s *ServerConfigTests) TestServerConfigValidation() {
	s.cfg.AliveCheckIPList = []string{"1", "2", "3"}
	err := s.cfg.validate()
	s.Error(err)
}

func (s *ServerConfigTests) TestTokenCreationValidation() {
	testClientId := "us-east-1:44211d8c-caf6-4b17-80cf-72febe0ebb2d"
	testClientContext := "123451234512345"
	testDeviceId := "NchoDC28E565X072CX46B1XBF205"
	s.cfg.AliveCheckIPList = nil
	err := s.cfg.validate()
	require.NoError(s.T(), err)
	token, err := s.cfg.CreateAuthToken(testClientId, testClientContext, testDeviceId)
	s.NoError(err)
	fmt.Printf("%d %s\n", len(token), token)

	t, err := s.cfg.ValidateAuthToken(testClientId, testClientContext, testDeviceId, token)
	s.NoError(err)
	s.NotEmpty(t)
	s.NotEqual(time.Time{}, t)
}
