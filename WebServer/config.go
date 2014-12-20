package WebServer

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const (
	defaultPort        = 8080
	defaultHost        = "0.0.0.0"
	defaultTemplateDir = "templates"
)

// ServerConfiguration - The structure of the json config needed for server values, like port, and bind_address
type ServerConfiguration struct {
	port        int
	bindAddress string
	templateDir string
	debug       bool
}

var configFile string

var Server ServerConfiguration

func init() {
	Server.templateDir = defaultTemplateDir
	flag.IntVar(&Server.port, "port", defaultPort, "The post to bind to")
	flag.IntVar(&Server.port, "p", defaultPort, "The post to bind to")
	flag.StringVar(&Server.bindAddress, "host", defaultHost, "The IP address to bind to")
	flag.StringVar(&configFile, "config", "", "A config file to read.")
	flag.StringVar(&configFile, "c", "", "A config file to read.")
	flag.BoolVar(&Server.debug, "debug", false, "Debug")
	flag.BoolVar(&Server.debug, "d", false, "Debug")
}

// Configuration - The top level configuration structure.
type Configuration struct {
	Server ServerConfiguration
}

func GetConfigAndRun() {
	flag.Parse()
	configuration := Configuration{Server}
	if configuration.Server.templateDir == "" {
		fmt.Println("No template directory specified!")
		os.Exit(1)
	}
	if configFile != "" {
		file, err := os.Open(configFile)
		if err != nil {
			fmt.Println("Could not open file", "conf.json", err)
			os.Exit(1)
		}
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&configuration)
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	}
	run(configuration)
}
