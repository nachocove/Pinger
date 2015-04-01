package AWS

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/sns"
	"strings"
)

func (ah *AWSHandle) getSNSSession() (*sns.SNS, error) {
	// TODO See about caching the sessions
	snsSession := sns.New(ah.awsCreds, ah.SnsRegionName, nil)
	return snsSession, nil
}

func (ah *AWSHandle) RegisterEndpointArn(service, token, customerData string) (string, error) {
	var platformArn string
	if strings.EqualFold(service, PushServiceAPNS) {
		platformArn = ah.SnsIOSPlatformArn
	} else {
		return "", fmt.Errorf("Unsupported platform service %s", service)
	}
	options := sns.CreatePlatformEndpointInput{
		Attributes:             nil,
		PlatformApplicationARN: aws.StringValue(&platformArn),
		CustomUserData:         aws.StringValue(&customerData),
		Token:                  aws.StringValue(&token),
	}
	snsSession, err := ah.getSNSSession()
	if err != nil {
		return "", err
	}
	response, err := snsSession.CreatePlatformEndpoint(&options)
	if err != nil {
		return "", err
	}
	return *response.EndpointARN, nil
}

func (ah *AWSHandle) GetEndpointAttributes(endpointArn string) (map[string]string, error) {
	options := sns.GetEndpointAttributesInput{
		EndpointARN: aws.StringValue(&endpointArn),
	}
	snsSession, err := ah.getSNSSession()
	if err != nil {
		return nil, err
	}
	response, err := snsSession.GetEndpointAttributes(&options)
	if err != nil {
		return nil, err
	}
	return response.Attributes, nil
}

func (ah *AWSHandle) SetEndpointAttributes(endpointArn string, attributes map[string]string) error {
	options := sns.SetEndpointAttributesInput{
		EndpointARN: aws.StringValue(&endpointArn),
		Attributes:  attributes,
	}
	snsSession, err := ah.getSNSSession()
	if err != nil {
		return err
	}
	err = snsSession.SetEndpointAttributes(&options)
	if err != nil {
		return err
	}
	return nil
}

func (ah *AWSHandle) DeleteEndpointArn(endpointArn string) error {
	options := sns.DeleteEndpointInput{
		EndpointARN: aws.StringValue(&endpointArn),
	}
	snsSession, err := ah.getSNSSession()
	if err != nil {
		return err
	}
	err = snsSession.DeleteEndpoint(&options)
	if err != nil {
		return err
	}
	return nil
}

func (ah *AWSHandle) SendPushNotification(endpointArn, message string) error {
	snsSession, err := ah.getSNSSession()
	if err != nil {
		return err
	}
	messageType := "json"
	attributes := make(sns.MessageAttributeMap)
	//	ttl := "12345"
	//	strString := "String"
	//	attrTTL := sns.MessageAttributeValue{DataType: aws.StringValue(&strString), StringValue: aws.StringValue(&ttl)}
	//	attributes["AWS.SNS.MOBILE.APNS.TTL"] = attrTTL
	//	attributes["AWS.SNS.MOBILE.GCM.TTL"] = attrTTL
	//	attributes["AWS.SNS.MOBILE.APNS_SANDBOX.TTL"] = attrTTL

	// sadly, PRIORITY does not exist (yet?)
	//	priority := "10"
	//	attrPriority := sns.MessageAttributeValue{DataType: aws.StringValue(&strString), StringValue: aws.StringValue(&priority)}
	//	attributes["AWS.SNS.MOBILE.APNS.PRIORITY"] = attrPriority

	input := sns.PublishInput{
		Message:           aws.StringValue(&message),
		MessageAttributes: attributes,
		MessageStructure:  aws.StringValue(&messageType),
		TargetARN:         aws.StringValue(&endpointArn),
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

func DecodeAPNSPushToken(token string) (string, error) {
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
