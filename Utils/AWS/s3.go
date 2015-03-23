package AWS

import (
	"fmt"
	"github.com/rlmcpherson/s3gof3r"
	"io"
	"os"
)

func (config *AWSConfiguration) PutFile(bucket, srcFilePath, destFilePath string) error {
	k := s3gof3r.Keys{
		AccessKey:     config.AccessKey,
		SecretKey:     config.SecretKey,
		SecurityToken: "",
	}

	endpoint := fmt.Sprintf("s3-%s.%s", config.S3RegionName, "amazonaws.com")
	// Open bucket to put file into
	s3 := s3gof3r.New(endpoint, k)
	b := s3.Bucket(bucket)

	// open file to upload
	file, err := os.Open(srcFilePath)
	if err != nil {
		return err
	}

	// Open a PutWriter for upload
	w, err := b.PutWriter(destFilePath, nil, nil)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, file); err != nil { // Copy into S3
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return nil
}
