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
	ValidateEndpointArn(endpointArn string) (map[string]string, error)
	SendPushNotification(endpointArn, message string) error
	ValidateCognitoID(clientId string) error
	PutFile(bucket, srcFilePath, destFilePath string) error
}

// AWSHandle is the collection of AWS related information
type AWSHandle struct {
	AWSConfiguration
	awsCreds aws.CredentialsProvider
}
