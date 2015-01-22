package Pinger

import (
	"fmt"
	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/sns"
	"time"
)

var snsSession *sns.SNS

func getSNSSession() (*sns.SNS, error) {
	if snsSession == nil {
		regionName := "us-west-2"
		accessKey := "AKIAIEKBHZUDER5TYR7Q"
		secretKey := "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"
		token := ""
		expiration := time.Now()
		region := aws.GetRegion(regionName)
		auth, err := aws.GetAuth(accessKey, secretKey, token, expiration)
		if err != nil {
			return nil, err
		}
		snsSession, err = sns.New(auth, region)
		if err != nil {
			return nil, err
		}
	}
	return snsSession, nil
}

func validateEnpointArn(endpointArn string) error {
	snsSession, err := getSNSSession()
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
