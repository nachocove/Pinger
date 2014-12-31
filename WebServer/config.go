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

var server ServerConfiguration

func init() {
	server.templateDir = defaultTemplateDir
	flag.IntVar(&server.port, "port", defaultPort, "The post to bind to")
	flag.IntVar(&server.port, "p", defaultPort, "The post to bind to")
	flag.StringVar(&server.bindAddress, "host", defaultHost, "The IP address to bind to")
	flag.StringVar(&configFile, "config", "", "A config file to read.")
	flag.StringVar(&configFile, "c", "", "A config file to read.")
	flag.BoolVar(&server.debug, "debug", false, "Debug")
	flag.BoolVar(&server.debug, "d", false, "Debug")
}

// Configuration - The top level configuration structure.
type Configuration struct {
	Server ServerConfiguration
}

// GetConfigAndRun process command line arguments and run the server
func GetConfigAndRun() {
	flag.Parse()
	configuration := Configuration{server}
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
