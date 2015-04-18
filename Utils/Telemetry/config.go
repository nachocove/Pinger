package Telemetry

import (
	"fmt"
)

// TelemetryConfiguration The telemetry configuration section in a file
type TelemetryConfiguration struct {
	FileLocationPrefix   string
	UploadLocationPrefix string
	IncludeDebug         bool
	UploadInterval       int64
}

// NewTelemetryConfiguration creates a new TelemetryConfiguration
func NewTelemetryConfiguration() *TelemetryConfiguration {
	return &TelemetryConfiguration{
		IncludeDebug:   false,
		UploadInterval: 10,
	}
}

// Validate validate the TelemetryConfiguration
func (config *TelemetryConfiguration) Validate() error {
	if config.UploadInterval <= 0 {
		return fmt.Errorf("UploadInterval can not be <= 0")
	}
	return nil
}
