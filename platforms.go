package Pinger

import (

)

type DevicePlatform int
const (
	PLATFORM_UNKNOWN DevicePlatform = iota
 	PLATFORM_IOS DevicePlatform = iota
 	PLATFORM_ANDROID DevicePlatform = iota 
)

var devicePlatforms = [...]string {
 "UNKNOWN",
 "IOS",
 "ANDROID",
}

func (platform DevicePlatform) String() string {
 return devicePlatforms[platform]
}

type Device struct {
    DeviceId string
    CognitoId string
    Platform  DevicePlatform
    MailClientType     string
    PersonId int64
}
