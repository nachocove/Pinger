package WebServer

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"code.google.com/p/gcfg"
	"github.com/codegangsta/negroni"
	"github.com/coopernurse/gorp"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nachocove/Pinger/Pinger"
	"github.com/op/go-logging"
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
	defaultLogFileName    = "webserver.log"
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

type GlobalConfiguration struct {
	Debug       bool
	Development bool
	LogDir      string
	LogFileName string
}

type RPCServerConfiguration struct {
	Hostname string
	Port     int
}

// Configuration - The top level configuration structure.
type Configuration struct {
	Server ServerConfiguration
	Global GlobalConfiguration
	DB     Pinger.DBConfiguration
	Rpc    RPCServerConfiguration
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
		NonTlsPort:     defaultNonTLSPort,
		SessionSecret:  "",
	},
		Global: GlobalConfiguration{
			Debug:       defaultDebug,
			Development: defaultDevelopment,
			LogDir:      defaultLogDir,
			LogFileName: defaultLogFileName,
		},
		DB: Pinger.DBConfiguration{
			Type:        "sqlite",
			Filename:    "pinger.db",
			Host:        "",
			Port:        0,
			Name:        "",
			Username:    "",
			Password:    "",
			Certificate: "",
		},
		Rpc: RPCServerConfiguration{
			Hostname: "localhost",
			Port:     Pinger.RPCPort,
		},
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "USAGE: %s [args] <config file>\n Args:\n", path.Base(os.Args[0]))
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

type Context struct {
	Config           *Configuration
	Dbm              *gorp.DbMap
	Logger           *logging.Logger
	RpcConnectString string
	SessionStore     *sessions.CookieStore
}

func NewContext(
	config *Configuration,
	dbm *gorp.DbMap,
	logger *logging.Logger,
	rpcConnectString string,
	sessionstore *sessions.CookieStore) *Context {
	return &Context{
		Config:           config,
		Dbm:              dbm,
		Logger:           logger,
		RpcConnectString: rpcConnectString,
		SessionStore:     sessionstore,
	}
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
	flag.Usage = usage
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
			log.Fatalf("Error reading config file:\n%v\n", err)
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
		log.Fatalf("No template directory specified!")
		os.Exit(1)
	}
	if !exists(config.Global.LogDir) {
		log.Fatalf("Logging directory %s does not exist.\n", config.Global.LogDir)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(path.Join(config.Global.LogDir, config.Global.LogFileName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
	var screenLogging = false
	var screenLevel = logging.ERROR
	if debug {
		screenLogging = true
		screenLevel = logging.DEBUG
	}
	logger := Pinger.InitLogging("pinger-webfe", logFile, logging.DEBUG, screenLogging, screenLevel)
	dbm := Pinger.InitDB(&config.DB, true)
	if dbm == nil {
		logger.Error("Could not open DB Connection")
		os.Exit(1)
	}
	defer dbm.Db.Close()

	context := NewContext(
		config,
		dbm,
		logger,
		fmt.Sprintf("%s:%d", config.Rpc.Hostname, config.Rpc.Port),
		sessions.NewCookieStore([]byte(config.Server.SessionSecret)))
	err = context.run()
	if err != nil {
		logger.Error("Could not run server!")
		os.Exit(1)
	}
	logger.Info("Exiting Server.\n")
	os.Exit(0)
}

var httpsRouter = mux.NewRouter()

func (context *Context) run() error {
	config := context.Config
	httpsMiddlewares := negroni.New(
		Pinger.NewRecovery("Pinger-web", config.Global.Debug),
		negroni.NewLogger(),
		Pinger.NewStatic("/public", "/static", ""),
		NewContextMiddleWare(context))

	httpsMiddlewares.UseHandler(httpsRouter)

	addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.Port)
	context.Logger.Info("Listening on %s (redirecting from %d)\n", addr, config.Server.NonTlsPort)
	// start the server on the non-tls port to redirect
	go func() {
		httpMiddlewares := negroni.New(
			Pinger.NewRecovery("Pinger-web", config.Global.Debug),
			negroni.NewLogger(),
			Pinger.NewRedirectMiddleware("", config.Server.Port),
		)
		httpRouter := mux.NewRouter()
		httpMiddlewares.UseHandler(httpRouter)
		addr := fmt.Sprintf("%s:%d", config.Server.BindAddress, config.Server.NonTlsPort)
		err := http.ListenAndServe(addr, httpMiddlewares)
		if err != nil {
			context.Logger.Fatalf("Could not start server on port %d\n", config.Server.NonTlsPort)
		}
	}()
	// start the https server
	err := http.ListenAndServeTLS(addr, config.Server.ServerCertFile, config.Server.ServerKeyFile, httpsMiddlewares)
	return err
}
