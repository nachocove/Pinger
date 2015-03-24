package Telemetry

import ()

// TelemetryConfiguration the telemetry configuration section in a file
type TelemetryConfiguration struct {
	FileLocationPrefix   string
	UploadLocationPrefix string
	IncludeDebug bool
}
