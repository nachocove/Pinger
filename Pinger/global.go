package Pinger

import (
	"github.com/nachocove/Pinger/Utils/AWS"
)

type globalStuff struct {
	config      *GlobalConfiguration	
	aws         AWS.AWSHandler
}

var globals *globalStuff

func setGlobal(config *GlobalConfiguration, aws AWS.AWSHandler) {
	if globals != nil {
		panic("Can not set globals multiple times")
	}
	globals = &globalStuff{aws: aws, config: config}
}	
