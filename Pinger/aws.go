package Pinger

import (
	"fmt"
	"github.com/nachocove/goamz/aws"
	"github.com/nachocove/goamz/sns"
	"time"
)

type AwsCredentials struct {
	regionName string
	accessKey string
	secretKey string
}
func getSNSSession(creds *AwsCredentials) (*sns.SNS, error) {
	// TODO See about caching the sessions
	expiration := time.Now().Add(time.Duration(300)*time.Second)
	token := ""
	region := aws.GetRegion(creds.regionName)
	auth, err := aws.GetAuth(creds.accessKey, creds.secretKey, token, expiration)
	if err != nil {
		return nil, err
	}
	snsSession, err := sns.New(auth, region)
	if err != nil {
		return nil, err
	}
	return snsSession, nil
}

func validateEnpointArn(creds *AwsCredentials, endpointArn string) error {
	snsSession, err := getSNSSession(creds)
	if err != nil {
		return err
	}
	attributes, err := snsSession.GetEndpointAttributes(endpointArn)
	if err != nil {
		return err
	}
	fmt.Println(attributes)
	return nil
}

func registerEndpointArn(creds *AwsCredentials, platformArn, token string) (string, error) {
	options := sns.PlatformEndpointOptions{
		Attributes: nil,
		PlatformApplicationArn: platformArn,
		CustomUserData: "",
		Token: token,
	}
		
	snsSession, err := getSNSSession(creds)
	if err != nil {
		return "", err
	}
	response, err := snsSession.CreatePlatformEndpoint(&options)
	if err != nil {
		return "", err
	}
	return response.EndpointArn, nil
}



type Cognito int

func getCognitoSession() (*Cognito, error) {
	panic("Cognito not yet supported")
}

func validateCognitoId() (bool, error) {
	_, err := getCognitoSession()
	if err != nil {
		return false, err
	}
	return true, nil
}