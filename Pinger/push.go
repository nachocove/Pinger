package Pinger

import (
	"errors"
	"fmt"
)

type PushInformation struct {
	service string
	token string
}

func (push *PushInformation) send(message string) error {
	if push.service != "AWS" {
		return errors.New(fmt.Sprintf("Unsupported push service: %s", push.service))
	}
	return sendPushNotification(push.token, message)
}

func NewPushInformation(service, token, customerData string) (*PushInformation, error) {
	var pushInfo PushInformation
	if service != "APNS" {
		return nil, errors.New(fmt.Sprintf("Unsupported push service %s", service))
	}
	endpointArn, err := registerEndpointArn(service, token, customerData)
	if err != nil {
		return nil, err
	}
	pushInfo.service = "AWS"
	pushInfo.token = endpointArn
	return &pushInfo, nil
}
