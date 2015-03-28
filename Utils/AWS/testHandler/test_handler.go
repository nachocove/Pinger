package testHandler

import ()

type TestAwsHandler struct{}

func (ah *TestAwsHandler) RegisterEndpointArn(service, token, customerData string) (string, error) {
	return "someRegisteredEndpoint", nil
}
func (ah *TestAwsHandler) GetEndpointAttributes(endpointArn string) (map[string]string, error) {
	return make(map[string]string), nil
}
func (ah *TestAwsHandler) SetEndpointAttributes(endpointArn string, attributes map[string]string) error {
	return nil
}
func (ah *TestAwsHandler) DeleteEndpointArn(endpointArn string) error {
	return nil
}
func (ah *TestAwsHandler) ValidateEndpointArn(endpointArn string) (map[string]string, error) {
	attr := make(map[string]string)
	attr["Enabled"] = "true"
	return attr, nil
}
func (ah *TestAwsHandler) SendPushNotification(endpointArn, message string) error {
	return nil
}
func (ah *TestAwsHandler) ValidateCognitoID(clientId string) error {
	return nil
}
func (ah *TestAwsHandler) PutFile(bucket, srcFilePath, destFilePath string) error {
	return nil
}
