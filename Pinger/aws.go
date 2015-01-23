package Pinger

import (
	"fmt"
	"github.com/nachocove/goamz/aws"
	"github.com/nachocove/goamz/sns"
	"time"
	"errors"
)

func getSNSSession(config *AWSConfiguration) (*sns.SNS, error) {
	// TODO See about caching the sessions
	
	expiration := time.Now().Add(time.Duration(300)*time.Second)
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

func validateEnpointArn(endpointArn string) error {
	snsSession, err := getSNSSession(&DefaultPollingContext.config.Aws)
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

func registerEndpointArn(service, token, customerData string) (string, error) {
	var platformArn string
	if service == "APNS" {
		platformArn = DefaultPollingContext.config.Aws.SnsIOSPlatformArn
	} else {
		return "", errors.New(fmt.Sprintf("Unsupported platform service %s", service))
	}
	options := sns.PlatformEndpointOptions{
		Attributes: nil,
		PlatformApplicationArn: platformArn,
		CustomUserData: customerData,
		Token: token,
	}
	snsSession, err := getSNSSession(&DefaultPollingContext.config.Aws)
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

func sendPushNotification(endpointArn, message string) error {
	snsSession, err := getSNSSession(&DefaultPollingContext.config.Aws)
	if err != nil {
		return err
	}
	options := sns.PublishOptions{
		Message: message,
		MessageStructure: "", // set to "json" if the message is a json-formatted platform specific message (see AWS SDK docs)
		Subject: "", // Not used. Email notifications.
		TopicArn: "", // Not used for mobile push messages. Use only TargetArn
		TargetArn: endpointArn,
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