package AWS

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	cognitoidentity "github.com/awslabs/aws-sdk-go/gen/cognito/identity"
)

func (ah *AWSHandle) getCognitoIdentitySession() (*cognitoidentity.CognitoIdentity, error) {
	// TODO See about caching the sessions
	cognitoSession := cognitoidentity.New(ah.awsCreds, ah.CognitoIdentityRegionName, nil)
	return cognitoSession, nil
}

func (ah *AWSHandle) ValidateCognitoID(userId string) error {
	cognitoSession, err := ah.getCognitoIdentitySession()
	if err != nil {
		return err
	}
	input := cognitoidentity.DescribeIdentityInput{IdentityID: aws.StringValue(&userId)}
	response, err := cognitoSession.DescribeIdentity(&input)
	if err != nil {
		return err
	}
	if string(*response.IdentityID) == "" {
		return fmt.Errorf("No IdentityId returned.")
	}
	return nil
}
