package main

import (
	"flag"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"net/http"
	"os"
	"path"
	"runtime"
)

func usage() {
	fmt.Fprintf(os.Stderr, "USAGE: %s [args]\n Args:\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

type Context struct {
	Config       *Pinger.Configuration
	Logger       *Logging.Logger
	loggerLevel  Logging.Level
	SessionStore *sessions.CookieStore
}

func NewContext(
	config *Pinger.Configuration,
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

var httpsRouter = mux.NewRouter()

func (context *Context) run() error {
	config := context.Config
	httpsMiddlewares := negroni.New(
		Utils.NewRecovery("Pinger-web", config.Server.Debug),
		Utils.NewLogger(context.Logger),
		NewContextMiddleWare(context))

	httpsMiddlewares.UseHandler(httpsRouter)

	addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.Port)
	context.Logger.Info("Listening on %s\n", addr)
	if config.Server.NonTlsPort > 0 {
		context.Logger.Info("(redirecting from %d)\n", config.Server.NonTlsPort)
		// start the server on the non-tls port to redirect
		go func() {
			httpMiddlewares := negroni.New(
				Utils.NewRecovery("Pinger-web", config.Server.Debug),
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
	}
	Utils.AddDebugToggleSignal(context)
	var err error
	if config.Server.ServerCertFile != "" && config.Server.ServerKeyFile != "" {
		// start the https server
		err = http.ListenAndServeTLS(addr, config.Server.ServerCertFile, config.Server.ServerKeyFile, httpsMiddlewares)
	} else {
		err = http.ListenAndServe(addr, httpsMiddlewares)
	}
	return err
}

func main() {
	var configFile string
	var debug bool
	var port int
	var bindAddress string
	var err error
	var printErrors bool

	flag.IntVar(&port, "p", Pinger.DefaultPort, "The port to bind to")
	flag.StringVar(&bindAddress, "host", Pinger.DefaultBindAddress, "The IP address to bind to")
	flag.StringVar(&configFile, "c", "", "Configuration file")
	flag.BoolVar(&debug, "d", Pinger.DefaultDebugging, "Debug")
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
	config, err := Pinger.ReadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file:\n%v\n", err)
		os.Exit(1)
	}
	err = config.Logging.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error validate global config:\n%v\n", err)
		os.Exit(1)
	}
	if port != Pinger.DefaultPort {
		config.Server.Port = port
	}
	if bindAddress != Pinger.DefaultBindAddress {
		config.Server.BindAddress = bindAddress
	}
	if debug != Pinger.DefaultDebugging {
		config.Server.Debug = debug
	}
	var screenLogging = false
	var screenLevel = Logging.ERROR
	if debug {
		screenLogging = true
		screenLevel = Logging.DEBUG
	}
	// From here on, treat the cfg debug and cli debug the same.
	// Don't do this before we decide on the screen output above
	debug = debug || config.Server.Debug
	logger, err := config.Logging.InitLogging(screenLogging, screenLevel, nil, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error InitLogging: %v\n", err)
		os.Exit(1)
	}
	if config.Server.TemplateDir != "" {
		logger.Warning("templateDir is deprecated. Please remove from config.")
	}
	context := NewContext(
		config,
		logger,
		fmt.Sprintf("%s:%d", config.Rpc.Hostname, config.Rpc.Port),
		sessions.NewCookieStore([]byte(config.Server.SessionSecret)))

	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Debug("Running with %d Processors", runtime.NumCPU())

	logger.Info("Started %v", os.Args)
	err = context.run()
	if err != nil {
		logger.Error("Could not run server! %v", err)
		os.Exit(1)
	}
	logger.Info("Exiting Server.\n")
	os.Exit(0)
}
