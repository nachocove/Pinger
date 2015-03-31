package testHandler

import ()

type TestAwsHandler struct {
	registeredEndpoint    string
	registeredEndpointErr error

	returnGetAttributes    map[string]string
	returnGetAttributesErr error

	returnSetAttributesErr error

	returnDeleteAttributesErr error

	returnPushNotificationError error

	returnValidateCognitoIdError error

	returnPutFileError error

	ignorePushFailure bool
}

func NewTestAwsHandler() *TestAwsHandler {
	return &TestAwsHandler{
		registeredEndpoint: "arn:aws:sns:us-west-2:263277746520:endpoint/APNS/com.nachocove.nachomail.alpha/1bd0418c-48da-36f4-8653-8d54c36d54bd",
		registeredEndpointErr: nil,
		returnGetAttributes: map[string]string{
			"Enabled": "true",
			"Token": "12345",
			"CustomUserData": "",
		},
		returnGetAttributesErr: nil,
		returnSetAttributesErr: nil,
		returnDeleteAttributesErr: nil,
		returnPushNotificationError: nil,
		returnValidateCognitoIdError: nil,
		returnPutFileError: nil,
		ignorePushFailure: false,
	}
}
func (ah *TestAwsHandler) SetReturnRegisteredEndpoint(endpoint string, err error) {
	ah.registeredEndpoint = endpoint
	ah.registeredEndpointErr = err
}
func (ah *TestAwsHandler) SetReturnGetAttributes(attrs map[string]string, err error) {
	ah.returnGetAttributes = attrs
	ah.returnGetAttributesErr = err
}
func (ah *TestAwsHandler) SetReturnSetAttributes(err error) {
	ah.returnSetAttributesErr = err
}
func (ah *TestAwsHandler) SetReturnDeleteAttributes(err error) {
	ah.returnDeleteAttributesErr = err
}
func (ah *TestAwsHandler) SetIgnorePushFailure(ignore bool) {
	ah.ignorePushFailure = ignore
}
func (ah *TestAwsHandler) SetPushNotificationError(err error) {
	ah.returnPushNotificationError = err
}
func (ah *TestAwsHandler) SetValidateCognitoIdError(err error) {
	ah.returnValidateCognitoIdError = err
}
func (ah *TestAwsHandler) SetPutFileError(err error) {
	ah.returnPutFileError = err
}

func (ah *TestAwsHandler) RegisterEndpointArn(service, token, customerData string) (string, error) {
	return ah.registeredEndpoint, ah.registeredEndpointErr
}
func (ah *TestAwsHandler) GetEndpointAttributes(endpointArn string) (map[string]string, error) {
	return ah.returnGetAttributes, ah.returnGetAttributesErr
}
func (ah *TestAwsHandler) SetEndpointAttributes(endpointArn string, attributes map[string]string) error {
	return ah.returnSetAttributesErr
}
func (ah *TestAwsHandler) DeleteEndpointArn(endpointArn string) error {
	return ah.returnDeleteAttributesErr
}
func (ah *TestAwsHandler) SendPushNotification(endpointArn, message string) error {
	return ah.returnPushNotificationError
}
func (ah *TestAwsHandler) ValidateCognitoID(clientId string) error {
	return ah.returnValidateCognitoIdError
}
func (ah *TestAwsHandler) PutFile(bucket, srcFilePath, destFilePath string) error {
	return ah.returnPutFileError
}
func (ah *TestAwsHandler) IgnorePushFailures() bool {
	return ah.ignorePushFailure
}
