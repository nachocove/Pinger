package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
)

type globalStuff struct {
	config *BackendConfiguration
	aws    AWS.AWSHandler
}

var globals *globalStuff

func setGlobal(config *BackendConfiguration, aws AWS.AWSHandler) {
	if globals != nil {
		panic("Can not set globals multiple times")
	}
	globals = &globalStuff{aws: aws, config: config}
}
