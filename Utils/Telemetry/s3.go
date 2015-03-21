package Telemetry

import (
	"fmt"
)

func (writer *TelemetryWriter) pushToS3(fileName string) error {
	if writer.uploadLocationPrefix != "" {
		return fmt.Errorf("Not implemented")
	}
	return nil
}
