package Pinger

import (
	"code.google.com/p/gcfg"
)

type Configuration struct {
	Global GlobalConfiguration
	Aws AWSConfiguration
	Db  DBConfiguration
}

type GlobalConfiguration struct {
	DumpRequests bool	
}
func ReadConfig(filename string) (*Configuration, error) {
	config := Configuration{}
	err := gcfg.ReadFileInto(&config, filename)
	if err != nil {
		return nil, err
	}
	err = config.Aws.Validate()
	if err != nil {
		return nil, err
	}
	return &config, nil
}
