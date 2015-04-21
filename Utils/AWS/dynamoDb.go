package AWS

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/dynamodb"
	"strconv"
)

type DynamoDb struct {
	session *dynamodb.DynamoDB
}

type DBAttributeType int

const (
	String DBAttributeType = iota
	Number DBAttributeType = iota
)

func (a DBAttributeType) AwsString() aws.StringValue {
	switch a {
	case String:
		return aws.String("S")

	case Number:
		return aws.String("N")
	}
	panic("Unknown value")
}

type DBAttrDefinition struct {
	Name string
	Type DBAttributeType
}

type KeyComparisonType int

const (
	KeyComparisonEq KeyComparisonType = 0
	KeyComparisonGt KeyComparisonType = iota
	KeyComparisonLt KeyComparisonType = iota
)

func (c KeyComparisonType) awsComparison() aws.StringValue {
	switch c {
	case KeyComparisonEq:
		return aws.String("EQ")

	case KeyComparisonGt:
		return aws.String("GT")

	case KeyComparisonLt:
		return aws.String("LT")
	}
	panic("unknown KeyComparisonType")
}

type DBKeyValue struct {
	Key        string
	Value      interface{}
	Comparison KeyComparisonType
}

type KeyType int

const (
	KeyTypeHash  KeyType = iota
	KeyTypeRange KeyType = iota
)

func (t KeyType) awsKeyType() aws.StringValue {
	switch t {
	case KeyTypeHash:
		return aws.String(dynamodb.KeyTypeHash)

	case KeyTypeRange:
		return aws.String(dynamodb.KeyTypeRange)
	}
	panic("unknown keytype")
}

type DBKeyType struct {
	Name string
	Type KeyType
}

type ThroughPut struct {
	Read  int64
	Write int64
}

func newDynamoDbSession(accessKey, secretKey, region string) *DynamoDb {
	return &DynamoDb{session: dynamodb.New(aws.Creds(accessKey, secretKey, ""), region, nil)}
}

func (ah *AWSHandle) GetDynamoDbSession() *DynamoDb {
	return newDynamoDbSession(ah.AccessKey, ah.SecretKey, ah.DynamoDbRegionName)
}

func (d *DynamoDb) Get(tableName string, keys []DBKeyValue) (*map[string]interface{}, error) {
	dKeys := make(map[string]dynamodb.AttributeValue)
	for _, k := range keys {
		if k.Comparison != KeyComparisonEq {
			return nil, fmt.Errorf("Can not use anything but EQ for Get call")
		}
		dKeys[k.Key] = goTypeToAttributeValue(k.Value)
	}
	getReq := dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       dKeys,
	}
	getResp, err := d.session.GetItem(&getReq)
	if err != nil {
		return nil, err
	}
	return awsAttributeMapToGo(&getResp.Item), nil
}

func (d *DynamoDb) Search(tableName string, attributes []DBKeyValue) ([]map[string]interface{}, error) {
	req := dynamodb.QueryInput{
		TableName:      aws.String(tableName),
		ConsistentRead: aws.Boolean(false),
	}

	// TODO Need to map the keys passed in to an Indexname, if appropriate
	indexName := ""
	if indexName != "" {
		req.IndexName = aws.String(indexName)
	}

	req.KeyConditions = make(map[string]dynamodb.Condition)
	for _, attr := range attributes {
		req.KeyConditions[attr.Key] = dynamodb.Condition{
			AttributeValueList: []dynamodb.AttributeValue{goTypeToAttributeValue(attr.Value)},
			ComparisonOperator: attr.Comparison.awsComparison(),
		}
	}
	queResp, err := d.session.Query(&req)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, 1)
	for _, item := range queResp.Items {
		items = append(items, *awsAttributeMapToGo(&item))
	}
	return items, nil
}

func (d *DynamoDb) Insert(tableName string, entry map[string]interface{}) error {
	req := dynamodb.PutItemInput{
		TableName: aws.StringValue(&tableName),
		Item:      *goMaptoAwsAttributeMap(&entry),
	}
	_, err := d.session.PutItem(&req)
	if err != nil {
		return err
	}
	return nil
}

func (d *DynamoDb) Update(tableName string, entry map[string]interface{}) error {
	return d.Insert(tableName, entry)
}

func (d *DynamoDb) Delete(tableName string, entry map[string]interface{}) error {
	req := dynamodb.DeleteItemInput{
		TableName: aws.StringValue(&tableName),
		Key:       *goMaptoAwsAttributeMap(&entry),
	}
	_, err := d.session.DeleteItem(&req)
	if err != nil {
		return err
	}
	return nil
}

func (d *DynamoDb) CreateTable(tableDefinition *dynamodb.CreateTableInput) error {
	_, err := d.session.CreateTable(tableDefinition)
	return err
}

func (d *DynamoDb) CreateTableReq(tableName string, attributes []DBAttrDefinition, keys []DBKeyType, throughput ThroughPut) *dynamodb.CreateTableInput {
	createReq := dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
	}

	createReq.AttributeDefinitions = make([]dynamodb.AttributeDefinition, 0, 1)
	for _, a := range attributes {
		attr := dynamodb.AttributeDefinition{AttributeName: aws.String(a.Name), AttributeType: a.Type.AwsString()}
		createReq.AttributeDefinitions = append(createReq.AttributeDefinitions, attr)
	}

	createReq.KeySchema = make([]dynamodb.KeySchemaElement, 0, 1)
	for _, k := range keys {
		el := dynamodb.KeySchemaElement{AttributeName: aws.String(k.Name), KeyType: k.Type.awsKeyType()}
		createReq.KeySchema = append(createReq.KeySchema, el)
	}
	createReq.ProvisionedThroughput = &dynamodb.ProvisionedThroughput{ReadCapacityUnits: aws.Long(throughput.Read), WriteCapacityUnits: aws.Long(throughput.Write)}

	return &createReq
}

func (d *DynamoDb) DeleteTable(tableName string) error {
	req := dynamodb.DeleteTableInput{TableName: aws.String(tableName)}
	_, err := d.session.DeleteTable(&req)
	return err
}

func (d *DynamoDb) AddGlobalSecondaryIndexStruct(createReq *dynamodb.CreateTableInput, indexName string, keys []DBKeyType, throughput ThroughPut) error {
	gsi := dynamodb.GlobalSecondaryIndex{
		IndexName:  aws.String(indexName),
		Projection: &dynamodb.Projection{ProjectionType: aws.String(dynamodb.ProjectionTypeAll)},
	}
	gsi.KeySchema = make([]dynamodb.KeySchemaElement, 0, 1)
	for _, k := range keys {
		key := dynamodb.KeySchemaElement{AttributeName: aws.String(k.Name), KeyType: k.Type.awsKeyType()}
		gsi.KeySchema = append(gsi.KeySchema, key)
	}
	gsi.ProvisionedThroughput = &dynamodb.ProvisionedThroughput{ReadCapacityUnits: aws.Long(throughput.Read), WriteCapacityUnits: aws.Long(throughput.Write)}
	if createReq.GlobalSecondaryIndexes == nil {
		createReq.GlobalSecondaryIndexes = make([]dynamodb.GlobalSecondaryIndex, 0, 1)
	}
	createReq.GlobalSecondaryIndexes = append(createReq.GlobalSecondaryIndexes, gsi)
	return nil
}

func (d *DynamoDb) DescribeTable(tableName string) (*dynamodb.TableDescription, error) {
	descReq := dynamodb.DescribeTableInput{TableName: aws.String(tableName)}
	descResp, err := d.session.DescribeTable(&descReq)
	if err != nil {
		return nil, err
	}
	return descResp.Table, nil
}

func awsAttributeValueToGo(a *dynamodb.AttributeValue) interface{} {
	switch {
	case a.S != nil:
		return string(*(a.S))

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
