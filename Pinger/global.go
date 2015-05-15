package Pinger

type globalStuff struct {
	config *BackendConfiguration
}

var globals *globalStuff

func setGlobal(config *BackendConfiguration) {
	if globals != nil {
		panic("Can not set globals multiple times")
	}
	globals = &globalStuff{
		config: config,
	}
}
