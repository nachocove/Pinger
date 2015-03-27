package Telemetry

import (
	"fmt"
)

// TelemetryConfiguration the telemetry configuration section in a file
type TelemetryConfiguration struct {
	FileLocationPrefix   string
	UploadLocationPrefix string
	IncludeDebug         bool
	UploadInterval       int64
}

func NewTelemetryConfiguration() *TelemetryConfiguration {
	return &TelemetryConfiguration{
		IncludeDebug: false,
		UploadInterval: 10,
	}
}

func (config *TelemetryConfiguration) Validate() error {
	if config.UploadInterval <= 0 {
		return fmt.Errorf("UploadInterval can not be <= 0")
	}
	return nil
}