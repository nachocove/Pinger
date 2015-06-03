package AWS

import (
	"github.com/awslabs/aws-sdk-go/aws"
)

const (
	PushServiceAPNS = "APNS"
)

type AWSHandler interface {
	RegisterEndpointArn(service, token, customerData string) (string, error)
	GetEndpointAttributes(endpointArn string) (map[string]string, error)
	SetEndpointAttributes(endpointArn string, attributes map[string]string) error
	DeleteEndpointArn(endpointArn string) error
	SendPushNotification(endpointArn, message string) error
	ValidateCognitoID(userId string) error
	PutFile(bucket, srcFilePath, destFilePath string) error
	IgnorePushFailures() bool
	GetDynamoDbSession() *DynamoDb
}

// AWSHandle is the collection of AWS related information
type AWSHandle struct {
	AWSConfiguration
	awsCreds aws.CredentialsProvider
}

func (aws *AWSHandle) IgnorePushFailures() bool {
	return aws.IgnorePushFailure
}
