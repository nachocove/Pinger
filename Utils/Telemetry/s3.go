package Telemetry

import (
	"path"
)

func (writer *TelemetryWriter) pushToS3(fileName string) error {
	if writer.uploadLocationPrefixUrl != nil {
		bucket := writer.uploadLocationPrefixUrl.Host
		srcPath := path.Join(writer.fileLocationPrefix, fileName)
		destPath := path.Join("/", writer.uploadLocationPrefixUrl.Path, fileName)
		err := writer.awsConfig.PutFile(bucket, srcPath, destPath)
		if err != nil {
			return err
		}
	}
	return nil
}
