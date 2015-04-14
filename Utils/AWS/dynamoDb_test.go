package AWS

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/dynamodb"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"os"
	"os/exec"
	"testing"
	"time"
	"net"
)

type awsDynamoDbTester struct {
	suite.Suite
	dynDb        *DynamoDb
	clientRecord map[string]interface{}
}

func (s *awsDynamoDbTester) SetupSuite() {
	readyCh := make(chan int)
	go doJavaDynamoLocal(readyCh)
	<-readyCh
	s.dynDb = newDynamoDbSession("AKIAIEKBHZUDER5TYR7Q", "9bSGWoFxSGRLS+J4EhLbR3NMkjWUbdVu+itcYT6g", "local")
	s.clientRecord = map[string]interface{}{
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
	cmd.Dir = fmt.Sprintf("%s/dynamodb_local_2013-12-12", nachoHome)
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	s.dynamoProcess = cmd.Process
	time.Sleep(1*time.Second)
	for {
		conn, err := net.Dial("tcp", "localhost:8000")
		if err == nil && conn != nil {
			conn.Close()
			readyCh <- 1
			break
		}
		time.Sleep(1*time.Second)
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
	createReq := dynamodb.CreateTableInput{
		TableName: aws.String(UnitTestTableName),
		AttributeDefinitions: []dynamodb.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: aws.String("S")},
			{AttributeName: aws.String("pinger"), AttributeType: aws.String("S")},
			{AttributeName: aws.String("client"), AttributeType: aws.String("S")},
		},
		KeySchema: []dynamodb.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: aws.String(dynamodb.KeyTypeHash)},
		},
		GlobalSecondaryIndexes: []dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String(UnitTestIndexName),
				KeySchema: []dynamodb.KeySchemaElement{
					{AttributeName: aws.String("pinger"), KeyType: aws.String(dynamodb.KeyTypeHash)},
					{AttributeName: aws.String("client"), KeyType: aws.String(dynamodb.KeyTypeRange)},
				},
				Projection:            &dynamodb.Projection{ProjectionType: aws.String(dynamodb.ProjectionTypeAll)},
				ProvisionedThroughput: &dynamodb.ProvisionedThroughput{ReadCapacityUnits: aws.Long(10), WriteCapacityUnits: aws.Long(10)},
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{ReadCapacityUnits: aws.Long(10), WriteCapacityUnits: aws.Long(10)},
	}

	_, err := s.dynDb.session.CreateTable(&createReq)
	require.NoError(s.T(), err)
}

func (s *awsDynamoDbTester) TestTableCreate() {
	s.createTestTable()

	listReq := dynamodb.ListTablesInput{}
	listResp, err := s.dynDb.session.ListTables(&listReq)
	s.NoError(err)
	s.NotEmpty(listResp.TableNames)
	
	descReq := dynamodb.DescribeTableInput{
		TableName: aws.String(UnitTestTableName),
	}
	descResp, err := s.dynDb.session.DescribeTable(&descReq)
	s.NoError(err)
	s.NotEmpty(descResp.Table.GlobalSecondaryIndexes)
}

func (s *awsDynamoDbTester) itemCreate() string {
	id := uuid.NewV4().String()
	item := *goMaptoAwsAttributeMap(&s.clientRecord)
	item["id"] = goTypeToAttributeValue(id)
	putReq := dynamodb.PutItemInput{
		TableName: aws.String(UnitTestTableName),
		Item:      item,
	}
	putResp, err := s.dynDb.session.PutItem(&putReq)
	s.NoError(err)
	s.Empty(putResp.Attributes)
	return id
}

func (s *awsDynamoDbTester) TestItemCreate() {
	s.createTestTable()
	s.itemCreate()
}

func (s *awsDynamoDbTester) TestItemQuery() {
	s.createTestTable()
	s.itemCreate()
	queReq := dynamodb.QueryInput{
		TableName: aws.String(UnitTestTableName),
		//AttributesToGet: []string{"id", "client", "pinger", "device", "push_service", "push_token"},
		ConsistentRead: aws.Boolean(false),
		IndexName:      aws.String(UnitTestIndexName),
		KeyConditions: map[string]dynamodb.Condition{
			"pinger": dynamodb.Condition{
				AttributeValueList: []dynamodb.AttributeValue{
					goTypeToAttributeValue(s.clientRecord["pinger"]),
				},
				ComparisonOperator: aws.String("EQ"),
			},
		},
	}
	queResp, err := s.dynDb.session.Query(&queReq)
	s.NoError(err)
	s.NotEmpty(queResp.Items)
	for _, item := range queResp.Items {
		x := *(awsAttributeMapToGo(&item))

		v, ok := x["client"]
		s.True(ok)
		s.Equal(s.clientRecord["client"], v)

		v, ok = x["pinger"]
		s.True(ok)
		s.Equal(s.clientRecord["pinger"], v)

		v, ok = x["device"]
		s.True(ok)
		s.Equal(s.clientRecord["device"], v)

		v, ok = x["push_service"]
		s.True(ok)
		s.Equal(s.clientRecord["push_service"], v)

		v, ok = x["push_token"]
		s.True(ok)
		s.Equal(s.clientRecord["push_token"], v)
	}
}

func (s *awsDynamoDbTester) TestItemBatchGet() {
	s.createTestTable()
	id := s.itemCreate()
	getReq := dynamodb.BatchGetItemInput{
		RequestItems: map[string]dynamodb.KeysAndAttributes{
			UnitTestTableName: dynamodb.KeysAndAttributes{
				ConsistentRead: aws.Boolean(true),
				Keys: []map[string]dynamodb.AttributeValue{
					map[string]dynamodb.AttributeValue{
						"id": goTypeToAttributeValue(id),
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
		x := *(awsAttributeMapToGo(&item))

		v, ok := x["client"]
		s.True(ok)
		s.Equal(s.clientRecord["client"], v)

		v, ok = x["pinger"]
		s.True(ok)
		s.Equal(s.clientRecord["pinger"], v)

		v, ok = x["device"]
		s.True(ok)
		s.Equal(s.clientRecord["device"], v)

		v, ok = x["push_service"]
		s.True(ok)
		s.Equal(s.clientRecord["push_service"], v)

		v, ok = x["push_token"]
		s.True(ok)
		s.Equal(s.clientRecord["push_token"], v)
	}
}

func (s *awsDynamoDbTester) TestItemDelete() {
	s.createTestTable()
	id := s.itemCreate()
	delReq := dynamodb.DeleteItemInput{
		TableName: aws.String(UnitTestTableName),
		Key: map[string]dynamodb.AttributeValue{
			"id": goTypeToAttributeValue(id),
		},
	}

	_, err := s.dynDb.session.DeleteItem(&delReq)
	s.NoError(err)
}
