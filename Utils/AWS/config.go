package AWS

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
)

// AWSConfiguration is used by Pinger/config.go to read the aws config section
type AWSConfiguration struct {
	RegionName                string
	AccessKey                 string
	SecretKey                 string
	SnsRegionName             string
	SnsIOSPlatformArn         string
	CognitoIdentityRegionName string
	CognitoIdentityPoolID     string
	S3RegionName              string
	IgnorePushFailure         bool
	DynamoDbRegionName        string
}

// NewHandle creates a new AWSHandle from the information from the config file.
func (config *AWSConfiguration) NewHandle() *AWSHandle {
	token := ""
	creds := aws.Creds(config.AccessKey, config.SecretKey, token)
	return &AWSHandle{*config, creds}
}

func (config *AWSConfiguration) Validate() error {
	if config.AccessKey == "" || config.SecretKey == "" || config.RegionName == "" {
		return fmt.Errorf("aws section must be filled in")
	}
	if config.SnsRegionName == "" {
		config.SnsRegionName = config.RegionName
	}
	if config.CognitoIdentityRegionName == "" {
		config.CognitoIdentityRegionName = config.RegionName
	}
	if config.S3RegionName == "" {
		config.S3RegionName = config.RegionName
	}
	if config.DynamoDbRegionName == "" {
		config.DynamoDbRegionName = config.RegionName
	}
	return nil
}
