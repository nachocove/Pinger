package Pinger

import (
	"errors"
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	cognitoidentity "github.com/awslabs/aws-sdk-go/gen/cognito/identity"
	"github.com/awslabs/aws-sdk-go/gen/sns"
	"strings"
)

type AWSConfiguration struct {
	RegionName string
	AccessKey  string
	SecretKey  string

	SnsRegionName     string
	SnsIOSPlatformArn string

	CognitoIdentityRegionName string
	CognitoIdentityPoolId     string
}

func (config *AWSConfiguration) Validate() error {
	if config.AccessKey == "" || config.SecretKey == "" || config.RegionName == "" {
		return errors.New("aws section must be filled in")
	}
	if config.SnsRegionName == "" {
		config.SnsRegionName = config.RegionName
	}
	if config.CognitoIdentityRegionName == "" {
		config.CognitoIdentityRegionName = config.RegionName
	}
	return nil
}

func (config *AWSConfiguration) getSNSSession() (*sns.SNS, error) {
	// TODO See about caching the sessions
	token := ""
	creds := aws.Creds(config.AccessKey, config.SecretKey, token)
	snsSession := sns.New(creds, config.SnsRegionName, nil)
	return snsSession, nil
}

func (config *AWSConfiguration) registerEndpointArn(service, token, customerData string) (string, error) {
	var platformArn string
	if strings.EqualFold(service, PushServiceAPNS) {
		platformArn = config.SnsIOSPlatformArn
	} else {
		return "", errors.New(fmt.Sprintf("Unsupported platform service %s", service))
	}
	options := sns.CreatePlatformEndpointInput{
		Attributes:             nil,
		PlatformApplicationARN: aws.StringValue(&platformArn),
		CustomUserData:         aws.StringValue(&customerData),
		Token:                  aws.StringValue(&token),
	}
	snsSession, err := config.getSNSSession()
	if err != nil {
		return "", err
	}
	response, err := snsSession.CreatePlatformEndpoint(&options)
	if err != nil {
		return "", err
	}
	return *response.EndpointARN, nil
}

func (config *AWSConfiguration) validateEndpointArn(endpointArn string) (map[string]string, error) {
	snsSession, err := config.getSNSSession()
	if err != nil {
		return nil, err
	}
	input := sns.GetEndpointAttributesInput{EndpointARN: aws.StringValue(&endpointArn)}
	response, err := snsSession.GetEndpointAttributes(&input)
	if err != nil {
		return nil, err
	}
	return response.Attributes, nil
}

func (config *AWSConfiguration) sendPushNotification(endpointArn, message string) error {
	snsSession, err := config.getSNSSession()
	if err != nil {
		return err
	}
	input := sns.PublishInput{
		Message:   aws.StringValue(&message),
		TargetARN: aws.StringValue(&endpointArn),
	}
	response, err := snsSession.Publish(&input)
	if err != nil {
		return err
	}
	if string(*response.MessageID) == "" {
		return errors.New("Empty messageID. This means the message was not sent.")
	}
	return nil
}

func (config *AWSConfiguration) getCognitoIdentitySession() (*cognitoidentity.CognitoIdentity, error) {
	// TODO See about caching the sessions
	token := ""
	creds := aws.Creds(config.AccessKey, config.SecretKey, token)
	cognitoSession := cognitoidentity.New(creds, config.CognitoIdentityRegionName, nil)
	return cognitoSession, nil
}

func (config *AWSConfiguration) validateCognitoId(clientId string) error {
	cognitoSession, err := config.getCognitoIdentitySession()
	if err != nil {
		return err
	}
	input := cognitoidentity.DescribeIdentityInput{IdentityID: aws.StringValue(&clientId)}
	_, err = cognitoSession.DescribeIdentity(&input)
	if err != nil {
		// TODO There appears to be a bug either in amazon or the go toolkit. it's returning
		// {"CreationDate":1.421516513393E9,"IdentityId":"us-east-1:0005d365-c8ea-470f-8a61-a7f44f145efb","LastModifiedDate":1.421516513393E9}
		// And chokes on those odd-looking datetime fields.
		// For now I've hacked the toolkit to pass back the numbers instead of interpreting them
		//return err
		fmt.Printf("ERROR(ignored): %v\n", err)
		return nil
	}
	return nil
}
