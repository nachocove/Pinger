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
	testClientId      string
	testClientContext string
	testDeviceId      string
}

func (s *deviceContactTester) SetupSuite() {
	var err error
	AWS.NewLocalDynamoDbProcess()
	s.logger = Logging.InitLogging("unittest", "", Logging.DEBUG, true, Logging.DEBUG, nil, true)
	dbconfig := DBConfiguration{Type: "sqlite", Filename: ":memory:"}
	s.dbm, err = initDB(&dbconfig, true, s.logger)
	if err != nil {
		panic("Could not create DB")
	}
	s.db = newDeviceContactSqlDbHandler(s.dbm)
	s.testClientId = "sometestClientId"
	s.testClientContext = "sometestclientContext"
	s.testDeviceId = "NCHOXfherekgrgr"
}

func (s *deviceContactTester) TearDownSuite() {
	AWS.KillLocalDynamoDbProcess()
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
	dc := newDeviceContact(s.db, s.testClientId, s.testClientContext, s.testDeviceId)
	require.NotNil(s.T(), dc)
	err := dc.insert()
	s.NoError(err)

	dc1, err := deviceContactGet(s.db, s.testClientId, s.testClientContext, s.testDeviceId)
	s.NoError(err)
	require.NotNil(s.T(), dc1)

	s.Equal(dc.ClientId, dc1.ClientId)
	s.Equal(dc.ClientContext, dc1.ClientContext)
	s.Equal(dc.DeviceId, dc1.DeviceId)
}

func (s *deviceContactTester) TestDeviceContactUpdate() {
	dc := newDeviceContact(s.db, s.testClientId, s.testClientContext, s.testDeviceId)
	require.NotNil(s.T(), dc)
	err := dc.insert()
	s.NoError(err)
	s.Equal(dc.Created, dc.LastContact)
	s.Equal(0, dc.LastContactRequest)

	err = dc.updateLastContact()
	s.NoError(err)
	s.True(dc.Created < dc.LastContact)
	s.Equal(0, dc.LastContactRequest)

	dc1, err := deviceContactGet(s.db, s.testClientId, s.testClientContext, s.testDeviceId)
	s.NoError(err)
	require.NotNil(s.T(), dc1)

	s.Equal(dc1.LastContact, dc.LastContact)

	err = dc.updateLastContactRequest()
	s.NoError(err)
	s.NotEqual(0, dc.LastContactRequest)

	dc1, err = deviceContactGet(s.db, s.testClientId, s.testClientContext, s.testDeviceId)
	s.NoError(err)
	require.NotNil(s.T(), dc1)

	s.Equal(dc1.LastContactRequest, dc.LastContactRequest)
}

func (s *deviceContactTester) createTable(dynamo *AWS.DynamoDb) {
	createReq := dynamo.CreateTableReq(dynamoDeviceContactTableName,
		[]AWS.DBAttrDefinition{
			{Name: "id", Type: AWS.Number},
			{Name: "pinger", Type: AWS.String},
			{Name: "client", Type: AWS.String},
		},
		[]AWS.DBKeyType{
			{Name: "id", Type: AWS.KeyTypeHash},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)
	if createReq == nil {
		panic("No createReq")
	}
	err := dynamo.AddGlobalSecondaryIndexStruct(createReq, dynamoDeviceContactTableName,
		[]AWS.DBKeyType{
			{Name: "pinger", Type: AWS.KeyTypeHash},
			{Name: "client", Type: AWS.KeyTypeRange},
		},
		AWS.ThroughPut{Read: 10, Write: 10},
	)
	if err != nil {
		panic(err)
	}
	err = dynamo.CreateTable(createReq)
	if err != nil {
		panic(err)
	}
}

func (s *deviceContactTester) TestDeviceContatDynamo() {
	dynamoHandler := newDeviceContactDynamoDbHandler(s.aws)
	defer dynamoHandler.dynamo.DeleteTable(dynamoDeviceContactTableName)
	
	s.createTable(dynamoHandler.dynamo)
	require.NotNil(s.T(), dynamoHandler)
	require.NotNil(s.T(), dynamoHandler.dynamo)
	require.NotEqual(s.T(), "", dynamoHandler.tableName)
	
	dc := newDeviceContact(dynamoHandler, s.testClientId, s.testClientContext, s.testDeviceId)
	require.NotNil(s.T(), dc)
	err := dc.insert()
	s.NoError(err)
	
	err = dc.updateLastContact()
	s.NoError(err)
	
	err = dc.delete()
	s.NoError(err)
}