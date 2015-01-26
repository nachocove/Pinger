package Pinger

import (
	"errors"
	"fmt"
	"github.com/nachocove/goamz/aws"
	"github.com/nachocove/goamz/sns"
	"time"
)

type AWSConfiguration struct {
	RegionName string
	AccessKey  string
	SecretKey  string

	SnsRegionName     string
	SnsIOSPlatformArn string

	CognitoRegionName     string
	CognitoIdentityPoolId string
}

func (config *AWSConfiguration) Validate() error {
	if config.AccessKey == "" || config.SecretKey == "" || config.RegionName == "" {
		return errors.New("aws section must be filled in")
	}
	if config.SnsRegionName == "" {
		config.SnsRegionName = config.RegionName
	}
	if config.CognitoRegionName == "" {
		config.CognitoRegionName = config.RegionName
	}
	return nil
}

func (config *AWSConfiguration) getSNSSession() (*sns.SNS, error) {
	// TODO See about caching the sessions

	expiration := time.Now().Add(time.Duration(300) * time.Second)
	token := ""
	auth, err := aws.GetAuth(config.AccessKey, config.SecretKey, token, expiration)
	if err != nil {
		return nil, err
	}
	region := aws.GetRegion(config.SnsRegionName)
	snsSession, err := sns.New(auth, region)
	if err != nil {
		return nil, err
	}
	return snsSession, nil
}

func (config *AWSConfiguration) validateEnpointArn(endpointArn string) error {
	snsSession, err := config.getSNSSession()
	if err != nil {
		return err
	}
	response, err := snsSession.GetEndpointAttributes(endpointArn)
	if err != nil {
		return err
	}
	err = validateResponseMetaData(&response.ResponseMetadata)
	if err != nil {
		return err
	}
	fmt.Println(response.Attributes)
	return nil
}

func (config *AWSConfiguration) registerEndpointArn(service, token, customerData string) (string, error) {
	var platformArn string
	if service == "APNS" {
		platformArn = config.SnsIOSPlatformArn
	} else {
		return "", errors.New(fmt.Sprintf("Unsupported platform service %s", service))
	}
	options := sns.PlatformEndpointOptions{
		Attributes:             nil,
		PlatformApplicationArn: platformArn,
		CustomUserData:         customerData,
		Token:                  token,
	}
	snsSession, err := config.getSNSSession()
	if err != nil {
		return "", err
	}
	response, err := snsSession.CreatePlatformEndpoint(&options)
	if err != nil {
		return "", err
	}
	err = validateResponseMetaData(&response.ResponseMetadata)
	if err != nil {
		return "", err
	}
	return response.EndpointArn, nil
}

func (config *AWSConfiguration) sendPushNotification(endpointArn, message string) error {
	snsSession, err := config.getSNSSession()
	if err != nil {
		return err
	}
	options := sns.PublishOptions{
		Message:          message,
		MessageStructure: "", // set to "json" if the message is a json-formatted platform specific message (see AWS SDK docs)
		Subject:          "", // Not used. Email notifications.
		TopicArn:         "", // Not used for mobile push messages. Use only TargetArn
		TargetArn:        endpointArn,
	}
	response, err := snsSession.Publish(&options)
	if err != nil {
		return err
	}
	return validateResponseMetaData(&response.ResponseMetadata)
}

func validateResponseMetaData(metaData *aws.ResponseMetadata) error {
	if metaData.RequestId == "" {
		return errors.New("No request ID in response")
	}
	return nil
}

type Cognito int

func getCognitoSession() (*Cognito, error) {
	panic("Cognito not yet supported")
}

func validateCognitoId(clientId string) error {
	// TODO Write me!
	return nil
	//	_, err := getCognitoSession()
	//	if err != nil {
	//		return err
	//	}
	//	return nil
}
