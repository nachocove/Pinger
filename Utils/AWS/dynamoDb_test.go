package AWS

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/dynamodb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

type awsDynamoDbTester struct {
	suite.Suite
	dynDb         *DynamoDb
	clientRecord  map[string]interface{}
	dynamoProcess *os.Process
}

func (s *awsDynamoDbTester) SetupSuite() {
	readyCh := make(chan int)
	go s.doJavaDynamoLocal(readyCh)
	<-readyCh
	s.dynDb = newDynamoDbSession("", "", "local")
	s.clientRecord = map[string]interface{}{
		"id":           int64(1),
		"client":       "foo12334",
		"pinger":       "pinger1",
		"device":       "NchoXDFF",
		"push_service": "APNS",
		"push_token":   "12345678",
	}
}

func (s *awsDynamoDbTester) TearDownSuite() {
	s.dynamoProcess.Kill()
}

func (s *awsDynamoDbTester) cleanUp() {
	req := dynamodb.DeleteTableInput{TableName: aws.String(UnitTestTableName)}
	s.dynDb.session.DeleteTable(&req) // don't care about response or error
}

func (s *awsDynamoDbTester) SetupTest() {
	s.cleanUp() // in case a previous run crashed or somehow left things unclean.
}

func (s *awsDynamoDbTester) TearDownTest() {
	s.cleanUp()
}

const dynamodb_local_dir string = "dynamodb_local_2016-03-01"

func (s *awsDynamoDbTester) doJavaDynamoLocal(readyCh chan int) {
	java, err := exec.LookPath("java")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(java, "-Djava.library.path=./DynamoDBLocal_lib", "-jar", "DynamoDBLocal.jar")
	nachoHome := os.Getenv("NACHO_HOME")
	if nachoHome == "" {
		nachoHome = fmt.Sprintf("%s/src/nacho", os.Getenv("HOME"))
	}
	cmd.Dir = fmt.Sprintf("%s/%s", nachoHome, dynamodb_local_dir)
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	s.dynamoProcess = cmd.Process
	time.Sleep(1 * time.Second)
	for {
		conn, err := net.Dial("tcp", "localhost:8000")
		if err == nil && conn != nil {
			conn.Close()
			readyCh <- 1
			break
		}
		time.Sleep(1 * time.Second)
	}
	err = cmd.Wait()
}

func TestAWSDynamoDb(t *testing.T) {
	s := new(awsDynamoDbTester)
	suite.Run(t, s)
}

const (
	UnitTestTableName = "dev.pinger.unittestTable"
	UnitTestIndexName = "index.pinger-device"
)

func (s *awsDynamoDbTester) createTestTable() {
	createReq := s.dynDb.CreateTableReq(UnitTestTableName,
		[]DBAttrDefinition{
			{Name: "id", Type: Number},
			{Name: "pinger", Type: String},
			{Name: "client", Type: String},
		},
		[]DBKeyType{
			{Name: "id", Type: KeyTypeHash},
		},
		ThroughPut{Read: 10, Write: 10},
	)
	require.NotNil(s.T(), createReq)

	err := s.dynDb.AddGlobalSecondaryIndexStruct(createReq, UnitTestIndexName,
		[]DBKeyType{
			{Name: "pinger", Type: KeyTypeHash},
			{Name: "client", Type: KeyTypeRange},
		},
		ThroughPut{Read: 10, Write: 10},
	)
	require.NoError(s.T(), err)

	err = s.dynDb.CreateTable(createReq)
	require.NoError(s.T(), err)
}

func (s *awsDynamoDbTester) TestTableCreate() {
	table, err := s.dynDb.DescribeTable(UnitTestTableName)
	require.Error(s.T(), err)
	s.Nil(table)

	s.createTestTable()

	table, err = s.dynDb.DescribeTable(UnitTestTableName)
	require.NoError(s.T(), err)
	s.NotNil(table)
	s.NotEmpty(table)
	s.NotEmpty(table.AttributeDefinitions)
	s.NotEmpty(table.GlobalSecondaryIndexes)

	listReq := dynamodb.ListTablesInput{}
	listResp, err := s.dynDb.session.ListTables(&listReq)
	s.NoError(err)
	s.NotEmpty(listResp.TableNames)
}

func (s *awsDynamoDbTester) itemCreate(rec map[string]interface{}) {
	err := s.dynDb.Insert(UnitTestTableName, rec)
	require.NoError(s.T(), err)
}

func (s *awsDynamoDbTester) TestItemCreate() {
	s.createTestTable()
	s.itemCreate(s.clientRecord)
}

func (s *awsDynamoDbTester) itemValidate(item *map[string]interface{}) {
	v, ok := (*item)["id"]
	s.True(ok)
	s.Equal(s.clientRecord["id"], v)

	v, ok = (*item)["client"]
	s.True(ok)
	s.Equal(s.clientRecord["client"], v)

	v, ok = (*item)["pinger"]
	s.True(ok)
	s.Equal(s.clientRecord["pinger"], v)

	v, ok = (*item)["device"]
	s.True(ok)
	s.Equal(s.clientRecord["device"], v)

	v, ok = (*item)["push_service"]
	s.True(ok)
	s.Equal(s.clientRecord["push_service"], v)

	v, ok = (*item)["push_token"]
	s.True(ok)
	s.Equal(s.clientRecord["push_token"], v)
}

func (s *awsDynamoDbTester) TestItemQuery() {
	s.createTestTable()
	s.itemCreate(s.clientRecord)

	resp, err := s.dynDb.Search(UnitTestTableName, []DBKeyValue{
		{Key: "id", Value: s.clientRecord["id"], Comparison: KeyComparisonEq},
	},
	)
	s.NoError(err)
	s.NotNil(resp)
	s.Equal(1, len(resp))
	for _, item := range resp {
		s.itemValidate(&item)
	}
}

func (s *awsDynamoDbTester) TestItemBatchGet() {
	s.createTestTable()
	s.itemCreate(s.clientRecord)

	resp, err := s.dynDb.Get(UnitTestTableName, []DBKeyValue{
		{Key: "id", Value: s.clientRecord["id"], Comparison: KeyComparisonEq},
	},
	)
	s.NoError(err)
	s.NotNil(resp)

	getReq := dynamodb.BatchGetItemInput{
		RequestItems: map[string]dynamodb.KeysAndAttributes{
			UnitTestTableName: dynamodb.KeysAndAttributes{
				ConsistentRead: aws.Boolean(true),
				Keys: []map[string]dynamodb.AttributeValue{
					map[string]dynamodb.AttributeValue{
						"id": goTypeToAttributeValue(s.clientRecord["id"]),
					},
				},
			},
		},
	}

	getResp, err := s.dynDb.session.BatchGetItem(&getReq)
	s.NoError(err)
	s.NotEmpty(getResp.Responses)
	s.NotEmpty(getResp.Responses[UnitTestTableName])
	for _, item := range getResp.Responses[UnitTestTableName] {
		s.itemValidate(awsAttributeMapToGo(&item))
	}
}

func (s *awsDynamoDbTester) TestItemDelete() {
	s.createTestTable()
	s.itemCreate(s.clientRecord)
	err := s.dynDb.Delete(UnitTestTableName, map[string]interface{}{"id": s.clientRecord["id"].(int64)})
	s.NoError(err)
}
