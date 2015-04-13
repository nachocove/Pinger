package AWS

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/dynamodb"
	"strconv"
	"github.com/satori/go.uuid"
)

type DynamoDbHandler interface {
	Get(tableName string, args ...interface{}) (*map[string]interface{}, error)
	Insert(tableName string, entry map[string]interface{}) (string, error)
	Update(tableName string, entry map[string]interface{}) error
	Delete(args ...interface{}) error
}
type DynamoDb struct {
	DynamoDbHandler
	session *dynamodb.DynamoDB
	tableDefinitions map[string]string
}

func newDynamoDbSession(accessKey, secretKey, region string) *DynamoDb {
	return &DynamoDb{
		session: dynamodb.New(aws.Creds(accessKey, secretKey, ""), region, nil),
	}
}

func (ah *AWSHandle) GetDynamoDbSession() DynamoDbHandler {
	return newDynamoDbSession(ah.AccessKey, ah.SecretKey, ah.DynamoDbRegionName)
}

func (d *DynamoDb) Get(tableName string, args ...interface{}) (*map[string]interface{}, error) {
//	getReq := dynamodb.GetItemInput{
//		TableName: aws.String(tableName),
//		Key: map[string]dynamodb.AttributeValue{
//			
//		},
//	}
	return nil, nil
}

func (d *DynamoDb) Search(tableName string, values map[string]map[string]interface{}) ([]map[string]interface{}, error) {
//	queReq := dynamodb.QueryInput{
//		TableName: aws.String(tableName),
//		ConsistentRead: aws.Boolean(false),
//		//IndexName:      aws.String(UnitTestIndexName),
//		KeyConditions: map[string]dynamodb.Condition{
//			"pinger": dynamodb.Condition{
//				AttributeValueList: []dynamodb.AttributeValue{
//					//goTypeToAttributeValue(s.clientRecord["pinger"]),
//				},
//				ComparisonOperator: aws.String("EQ"),
//			},
//		},
//	}
//	queResp, err := d.session.Query(&queReq)
//	return awsAttributeMapToGo(&queResp.Item), nil
	return nil, nil
}

func (d *DynamoDb) Insert(tableName string, entry map[string]interface{}) (string, error) {
	req := dynamodb.PutItemInput{
		TableName: aws.StringValue(&tableName),
		Item: *goMaptoAwsAttributeMap(&entry),
	}
	var id string
	if _, ok := req.Item["id"] ; ok == false {
		id = uuid.NewV4().String()
		req.Item["id"] = goTypeToAttributeValue(id)
	} else {
		item := req.Item["id"]
		id = awsAttributeValueToGo(&item).(string)
	}
	_, err := d.session.PutItem(&req)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DynamoDb) Update(tableName string, entry map[string]interface{}) error {
	_, err := d.Insert(tableName, entry)
	return err
}

func (d *DynamoDb) Delete(args ...interface{}) error {
	req := dynamodb.DeleteItemInput{}
	// TODO Fill this in.
	_, err := d.session.DeleteItem(&req)
	if err != nil {
		return err
	}
	return nil
}

func awsAttributeValueToGo(a *dynamodb.AttributeValue) interface{} {
	switch {
	case a.S != nil:
		return string(*(a.S))

	case a.BOOL != nil:
		return bool(*(a.BOOL))

	case a.B != nil:
		return a.B

	case a.N != nil:
		i, err := strconv.ParseInt(*(a.N), 0, 0)
		if err != nil {
			panic(err)
		}
		return i
	default:
		panic(fmt.Sprintf("unhandled type for dynamodb.AttributeValue %+v", a))
	}
}

func awsAttributeMapToGo(awsMap *map[string]dynamodb.AttributeValue) *map[string]interface{} {
	newMap := make(map[string]interface{})
	for k, item := range *awsMap {
		newMap[k] = awsAttributeValueToGo(&item)
	}
	return &newMap
}

func goTypeToAttributeValue(v interface{}) dynamodb.AttributeValue {
	a := dynamodb.AttributeValue{}
	switch v := v.(type) {
	case string:
		a.S = aws.String(v)
	case int:
		a.N = aws.String(fmt.Sprintf("%d", v))
	case int64:
		a.N = aws.String(fmt.Sprintf("%d", v))
	case bool:
		a.BOOL = aws.Boolean(v)
	default:
		panic(fmt.Sprintf("Unhandled type %+v", v))
	}
	return a
}

func goMaptoAwsAttributeMap(x *map[string]interface{}) *map[string]dynamodb.AttributeValue {
	awsMap := make(map[string]dynamodb.AttributeValue)
	for k, v := range *x {
		awsMap[k] = goTypeToAttributeValue(v)
	}
	return &awsMap
}
