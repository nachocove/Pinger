package Telemetry

import (
	"path"
)

func (writer *TelemetryWriter) pushToS3(fileName string) error {
	if writer.uploadLocationPrefixUrl != nil {
		bucket := writer.uploadLocationPrefixUrl.Host
		srcPath := path.Join(writer.fileLocationPrefix, fileName)
		datePrefix := fileName[5:13]
		destPath := path.Join("/", writer.uploadLocationPrefixUrl.Path, "/", datePrefix, "/", fileName)
		if writer.debug {
			writer.logger.Printf("Uploading %s to s3://%s%s", srcPath, bucket, destPath)
		}
		err := writer.aws.PutFile(bucket, srcPath, destPath)
		if err != nil {
			return err
		}
	}
	return nil
}
