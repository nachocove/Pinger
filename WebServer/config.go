package WebServer

import (
	"code.google.com/p/gcfg"
	"flag"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/op/go-logging"
	"net/http"
	"os"
	"path"
)

const (
	defaultPort           = 443
	defaultBindAddress    = "0.0.0.0"
	defaultTemplateDir    = "templates"
	defaultDebugging      = false
	defaultserverCertFile = ""
	defaultserverKeyFile  = ""
	defaultDebug          = false
	defaultNonTLSPort     = 80
	defaultDevelopment    = false
	defaultLogDir         = "/var/log/"
)

// ServerConfiguration - The structure of the json config needed for server values, like port, and bind_address
type ServerConfiguration struct {
	Port           int
	BindAddress    string
	TemplateDir    string
	ServerCertFile string
	ServerKeyFile  string
	Non_TLS_Port   int // underscores here, because this reflects the config-file key 'non-tls-port'
}

type GlobalConfiguration struct {
	Debug       bool
	Development bool
	LogDir      string
}

// Configuration - The top level configuration structure.
type Configuration struct {
	Server ServerConfiguration
	Global GlobalConfiguration
}

func (config *Configuration) Read(filename string) error {
	return gcfg.ReadFileInto(config, filename)
}

func NewConfiguration() *Configuration {
	return &Configuration{Server: ServerConfiguration{
		Port:           defaultPort,
		BindAddress:    defaultBindAddress,
		TemplateDir:    defaultTemplateDir,
		ServerCertFile: defaultserverCertFile,
		ServerKeyFile:  defaultserverKeyFile,
		Non_TLS_Port:   defaultNonTLSPort,
	},
		Global: GlobalConfiguration{
			Debug:       defaultDebug,
			Development: defaultDevelopment,
			LogDir:      defaultLogDir,
		},
	}
}

var usage = func() {
	fmt.Fprintf(os.Stderr, "USAGE: %s ....\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// GetConfigAndRun process command line arguments and run the server
func GetConfigAndRun() {
	var configFile string
	var debug bool
	var development bool
	var port int
	var bindAddress string
	var err error

	flag.IntVar(&port, "p", defaultPort, "The port to bind to")
	flag.StringVar(&bindAddress, "host", defaultBindAddress, "The IP address to bind to")
	flag.BoolVar(&debug, "d", defaultDebug, "Debug")
	flag.BoolVar(&development, "devel", defaultDebug, "In Development")
	flag.Parse()
	if len(flag.Args()) != 1 {
		usage()
		os.Exit(1)
	}
	configFile = flag.Arg(0)
	config := NewConfiguration()
	if configFile != "" {
		err = config.Read(configFile)
		if err != nil {
			Pinger.Log.Error("Error reading config file:\n%v\n", err)
			os.Exit(1)
		}
	}
	if port != defaultPort {
		config.Server.Port = port
	}
	if development != defaultDevelopment {
		config.Global.Development = development
	}
	if bindAddress != defaultBindAddress {
		config.Server.BindAddress = bindAddress
	}
	if debug != defaultDebug {
		config.Global.Debug = debug
	}
	if config.Server.TemplateDir == "" {
		Pinger.Log.Error("No template directory specified!")
		os.Exit(1)
	}
	if !exists(config.Global.LogDir) {
		Pinger.Log.Error("Logging directory %s does not exist.\n", config.Global.LogDir)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(path.Join(config.Global.LogDir, "webserver.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	var screenLogging = false
	var screenLevel = logging.ERROR
	if debug {
		screenLogging = true
		screenLevel = logging.DEBUG
	}
	logger = Pinger.InitLogging("pinger-webfe", logFile, logging.DEBUG, screenLogging, screenLevel)

	err = config.run()
	if err != nil {
		Pinger.Log.Error("Could not run server!")
		os.Exit(1)
	}
	Pinger.Log.Info("Exiting Server.\n")
	os.Exit(0)
}

var httpsRouter = mux.NewRouter()
var logger *logging.Logger

func (config *Configuration) run() error {
	httpsMiddlewares := negroni.New(
		NewRecovery(config.Global.Debug),
		negroni.NewLogger(),
		NewStatic("/public", "/static", ""),
		NewContextMiddleWare(config))

	httpsMiddlewares.UseHandler(httpsRouter)

	addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.Port)
	logger.Info("Listening on %s (redirecting from %d)\n", addr, config.Server.Non_TLS_Port)
	// start the server on the non-tls port to redirect
	go func() {
		httpMiddlewares := negroni.New(
			NewRecovery(config.Global.Debug),
			negroni.NewLogger(),
			NewRedirectMiddleware("", config.Server.Port),
		)
		httpRouter := mux.NewRouter()
		httpMiddlewares.UseHandler(httpRouter)
		addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.Non_TLS_Port)
		err := http.ListenAndServe(addr, httpMiddlewares)
		if err != nil {
			Pinger.Log.Fatalf("Could not start server on port %d\n", config.Server.Non_TLS_Port)
		}
	}()
	// start the https server
	err := http.ListenAndServeTLS(addr, config.Server.ServerCertFile, config.Server.ServerKeyFile, httpsMiddlewares)
	return err
}
