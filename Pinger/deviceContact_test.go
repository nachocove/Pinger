package Pinger

import (
	"github.com/coopernurse/gorp"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

type deviceContactTester struct {
	suite.Suite
	dbm               *gorp.DbMap
	aws               *AWS.TestAwsHandler
	db                DeviceContactDbHandler
	logger            *Logging.Logger
	testUserId      string
	testClientContext string
	testDeviceId      string
}

func (s *deviceContactTester) SetupSuite() {
	var err error
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbm, err = initDB(&dbconfig, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
	s.db = newDeviceContactSqlDbHandler(s.dbm)
	s.testUserId = "sometestUserId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
}

func (s *deviceContactTester) SetupTest() {
	s.dbm.TruncateTables()
}

func (s *deviceContactTester) TearDownTest() {
	globals = nil
}

func TestDeviceContact(t *testing.T) {
	s := new(deviceContactTester)
	suite.Run(t, s)
}

func (s *deviceContactTester) TestDeviceContactCreate() {
	dc := newDeviceContact(s.db, s.testUserId, s.testClientContext, s.testDeviceId)
	require.NotNil(s.T(), dc)
	err := dc.insert()
	s.NoError(err)

	dc1, err := deviceContactGet(s.db, s.testUserId, s.testClientContext, s.testDeviceId)
	s.NoError(err)
	require.NotNil(s.T(), dc1)

	s.Equal(dc.UserId, dc1.UserId)
	s.Equal(dc.ClientContext, dc1.ClientContext)
	s.Equal(dc.DeviceId, dc1.DeviceId)
}

func (s *deviceContactTester) TestDeviceContactUpdate() {
	dc := newDeviceContact(s.db, s.testUserId, s.testClientContext, s.testDeviceId)
	require.NotNil(s.T(), dc)
	err := dc.insert()
	s.NoError(err)
	s.Equal(dc.Created, dc.LastContact)
	s.Equal(0, dc.LastContactRequest)

	err = dc.updateLastContact()
	s.NoError(err)
	s.True(dc.Created < dc.LastContact)
	s.Equal(0, dc.LastContactRequest)

	dc1, err := deviceContactGet(s.db, s.testUserId, s.testClientContext, s.testDeviceId)
	s.NoError(err)
	require.NotNil(s.T(), dc1)

	s.Equal(dc1.LastContact, dc.LastContact)

	err = dc.updateLastContactRequest()
	s.NoError(err)
	s.NotEqual(0, dc.LastContactRequest)

	dc1, err = deviceContactGet(s.db, s.testUserId, s.testClientContext, s.testDeviceId)
	s.NoError(err)
	require.NotNil(s.T(), dc1)

	s.Equal(dc1.LastContactRequest, dc.LastContactRequest)
}
