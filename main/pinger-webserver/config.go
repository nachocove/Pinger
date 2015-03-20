package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"code.google.com/p/gcfg"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
)

// TODO Need to combine the configs into one, since there's shared settings. Just
//  have a webserver section and a backend section for daemon-specific stuff.
// Tricky: Need to be able to override some of the Global stuff per daemon
//  (like DumpRequests)

const (
	defaultPort           = 443
	defaultBindAddress    = "0.0.0.0"
	defaultTemplateDir    = "templates"
	defaultDebugging      = false
	defaultServerCertFile = ""
	defaultServerKeyFile  = ""
	defaultNonTLSPort     = 80
	defaultDevelopment    = false
	defaultDebug          = false
)

// ServerConfiguration - The structure of the json config needed for server values, like port, and bind_address
type ServerConfiguration struct {
	Port           int
	BindAddress    string
	TemplateDir    string
	ServerCertFile string
	ServerKeyFile  string
	NonTlsPort     int    `gcfg:"non-tls-port"`
	SessionSecret  string `gcfg:"session-secret"`
}

// Configuration - The top level configuration structure.
type Configuration struct {
	Server ServerConfiguration
	Global Pinger.GlobalConfiguration
	Rpc    Pinger.RPCServerConfiguration
}

func (config *Configuration) Read(filename string) error {
	err := gcfg.ReadFileInto(config, filename)
	if err != nil {
		return err
	}
	return nil
}

func NewConfiguration() *Configuration {
	config := &Configuration{
		Global: *Pinger.NewGlobalConfiguration(),
		Server: ServerConfiguration{
			Port:           defaultPort,
			BindAddress:    defaultBindAddress,
			TemplateDir:    defaultTemplateDir,
			ServerCertFile: defaultServerCertFile,
			ServerKeyFile:  defaultServerKeyFile,
			NonTlsPort:     defaultNonTLSPort,
			SessionSecret:  "",
		},
		Rpc: Pinger.NewRPCServerConfiguration(),
	}
	return config
}

func usage() {
	fmt.Fprintf(os.Stderr, "USAGE: %s [args]\n Args:\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

type Context struct {
	Config       *Configuration
	Logger       *Logging.Logger
	loggerLevel  Logging.Level
	SessionStore *sessions.CookieStore
}

func NewContext(
	config *Configuration,
	logger *Logging.Logger,
	rpcConnectString string,
	sessionStore *sessions.CookieStore) *Context {
	return &Context{
		Config:       config,
		Logger:       logger,
		loggerLevel:  -1,
		SessionStore: sessionStore,
	}
}

func (context *Context) ToggleDebug() {
	context.loggerLevel = Logging.ToggleLogging(context.Logger, context.loggerLevel)
}

// GetConfigAndRun process command line arguments and run the server
func GetConfigAndRun() {
	var configFile string
	var debug bool
	var port int
	var bindAddress string
	var err error
	var printErrors bool

	flag.IntVar(&port, "p", defaultPort, "The port to bind to")
	flag.StringVar(&bindAddress, "host", defaultBindAddress, "The IP address to bind to")
	flag.StringVar(&configFile, "c", "", "Configuration file")
	flag.BoolVar(&debug, "d", defaultDebug, "Debug")
	flag.BoolVar(&printErrors, "print-errors", false, "Print Error messages")
	flag.Usage = usage
	flag.Parse()

	if printErrors {
		printErrorsForDoc()
		os.Exit(0)
	}
	if configFile == "" {
		usage()
		os.Exit(1)
	}
	config := NewConfiguration()
	err = config.Read(configFile)
	if err != nil {
		log.Fatalf("Error reading config file:\n%v\n", err)
		os.Exit(1)
	}
	err = config.Global.Validate()
	if err != nil {
		log.Fatalf("Error validate global config:\n%v\n", err)
		os.Exit(1)
	}

	if port != defaultPort {
		config.Server.Port = port
	}
	if bindAddress != defaultBindAddress {
		config.Server.BindAddress = bindAddress
	}
	if debug != defaultDebug {
		config.Global.Debug = debug
	}
	debug = debug || config.Global.Debug
	if config.Server.TemplateDir == "" {
		log.Fatalf("No template directory specified!")
		os.Exit(1)
	}
	var screenLogging = false
	var screenLevel = Logging.ERROR
	if debug {
		screenLogging = true
		screenLevel = Logging.DEBUG
	}
	logger, err := config.Global.InitLogging(screenLogging, screenLevel, debug)
	if err != nil {
		log.Fatalf("Error InitLogging: %v\n", err)
		os.Exit(1)
	}
	context := NewContext(
		config,
		logger,
		fmt.Sprintf("%s:%d", config.Rpc.Hostname, config.Rpc.Port),
		sessions.NewCookieStore([]byte(config.Server.SessionSecret)))
	err = context.run()
	if err != nil {
		logger.Error("Could not run server! %v", err)
		os.Exit(1)
	}
	logger.Info("Exiting Server.\n")
	os.Exit(0)
}

var httpsRouter = mux.NewRouter()

func (context *Context) run() error {
	config := context.Config
	httpsMiddlewares := negroni.New(
		Utils.NewRecovery("Pinger-web", config.Global.Debug),
		Utils.NewLogger(context.Logger),
		Utils.NewStatic("/public", "/static", ""),
		NewContextMiddleWare(context))

	httpsMiddlewares.UseHandler(httpsRouter)

	addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.Port)
	context.Logger.Info("Listening on %s (redirecting from %d)\n", addr, config.Server.NonTlsPort)
	// start the server on the non-tls port to redirect
	go func() {
		httpMiddlewares := negroni.New(
			Utils.NewRecovery("Pinger-web", config.Global.Debug),
			Utils.NewLogger(context.Logger),
			Utils.NewRedirectMiddleware("", config.Server.Port),
		)
		httpRouter := mux.NewRouter()
		httpMiddlewares.UseHandler(httpRouter)
		addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.NonTlsPort)
		err := http.ListenAndServe(addr, httpMiddlewares)
		if err != nil {
			context.Logger.Fatalf("Could not start server on port %d\n", config.Server.NonTlsPort)
		}
	}()

	Utils.AddDebugToggleSignal(context)

	// start the https server
	err := http.ListenAndServeTLS(addr, config.Server.ServerCertFile, config.Server.ServerKeyFile, httpsMiddlewares)
	return err
}
