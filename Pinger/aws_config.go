package Pinger

import (
	"errors"
	"code.google.com/p/gcfg"
)

type AwsCredentials struct {
	RegionName string
	AccessKey string
	SecretKey string
}

type SnsSettings struct {
	RegionName string
	IOSPlatformArn string
}

type CognitoSettings struct {
	RegionName string
	IdentityPoolId string
}

type AWSConfiguration struct {
	Aws AwsCredentials
	Sns SnsSettings
	Cognito CognitoSettings
}

var AwsConfig *AWSConfiguration

func ReadAwsConfig(filename string) (*AWSConfiguration, error) {
	awsConfig := AWSConfiguration{}
	err := gcfg.ReadFileInto(&awsConfig, filename)
	if err != nil {
		return nil, err
	}
	if awsConfig.Aws.AccessKey == "" || awsConfig.Aws.SecretKey == "" || awsConfig.Aws.RegionName == "" {
		return nil, errors.New("aws section must be filled in")
	}
	if awsConfig.Sns.RegionName == "" {
		awsConfig.Sns.RegionName = awsConfig.Aws.RegionName
	}
	if awsConfig.Cognito.RegionName == "" {
		awsConfig.Cognito.RegionName = awsConfig.Aws.RegionName
	}
	AwsConfig = &awsConfig
	return &awsConfig, nil
}


