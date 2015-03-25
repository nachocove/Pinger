package AWS

import (
	"github.com/awslabs/aws-sdk-go/aws"
	"fmt"
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
	return nil
}

