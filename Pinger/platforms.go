package Pinger

import ()

// DevicePlatform The Platform type for a device
type DevicePlatform int

const (
	// PlatformUnknown an unknown platform
	PlatformUnknown DevicePlatform = iota
	// PlatformIOS iOS (Apple)
	PlatformIOS DevicePlatform = iota
	// PlatfromAndroid Android (Google)
	PlatfromAndroid DevicePlatform = iota
)

// DevicePlatforms the known device platforms
var DevicePlatforms = [...]string{
	"UNKNOWN",
	"iOS",
	"ANDROID",
}

func (platform DevicePlatform) String() string {
	return DevicePlatforms[platform]
}

// Device the various items to track a device
type Device struct {
	DeviceID       string
	CognitoID      string
	Platform       DevicePlatform
	MailClientType string
	PersonID       int64
}
