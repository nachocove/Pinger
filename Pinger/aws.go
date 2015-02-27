package Pinger

import (
	"encoding/base64"
	"encoding/hex"
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
	CognitoIdentityPoolID     string
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
		return "", fmt.Errorf("Unsupported platform service %s", service)
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
	messageType := "json"
	input := sns.PublishInput{
		Message:          aws.StringValue(&message),
		MessageStructure: aws.StringValue(&messageType),
		TargetARN:        aws.StringValue(&endpointArn),
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

func (config *AWSConfiguration) validateCognitoID(clientId string) error {
	cognitoSession, err := config.getCognitoIdentitySession()
	if err != nil {
		return err
	}
	input := cognitoidentity.DescribeIdentityInput{IdentityID: aws.StringValue(&clientId)}
	response, err := cognitoSession.DescribeIdentity(&input)
	if err != nil {
		return err
	}
	if string(*response.IdentityID) == "" {
		return errors.New("No IdentityId returned.")
	}
	return nil
}

func decodeAPNSPushToken(token string) (string, error) {
	if len(token) == 64 {
		// see if the string decodes, in which case it's probably already passed to us in hex
		_, err := hex.DecodeString(token)
		if err == nil {
			return token, nil
		}
	} else {
		tokenBytes, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return "", err
		}
		tokenstring := hex.EncodeToString(tokenBytes)
		if len(tokenstring) != 64 {
			return "", fmt.Errorf("Decoded token is not 64 bytes long")
		}
		return tokenstring, nil
	}
	return "", fmt.Errorf("Could not determine push token format: %s", token)
}
