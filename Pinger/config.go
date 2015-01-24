package Pinger

import (
	"code.google.com/p/gcfg"
	"errors"
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

func (awsConfig *AWSConfiguration) Validate() error {
	if awsConfig.AccessKey == "" || awsConfig.SecretKey == "" || awsConfig.RegionName == "" {
		return errors.New("aws section must be filled in")
	}
	if awsConfig.SnsRegionName == "" {
		awsConfig.SnsRegionName = awsConfig.RegionName
	}
	if awsConfig.CognitoRegionName == "" {
		awsConfig.CognitoRegionName = awsConfig.RegionName
	}
	return nil
}

type Configuration struct {
	Aws AWSConfiguration
	Db  DBConfiguration
}

func ReadConfig(filename string) (*Configuration, error) {
	config := Configuration{}
	err := gcfg.ReadFileInto(&config, filename)
	if err != nil {
		return nil, err
	}
	err = config.Aws.Validate()
	if err != nil {
		return nil, err
	}
	return &config, nil
}
