package WebServer

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"net/http"
)

type contextKey int

// Define keys that support equality.
const (
	serverConfigkey contextKey = iota
)

// GetServerConfig get the server config from the context
func GetServerConfig(r *http.Request) *ServerConfiguration {
	val, ok := context.GetOk(r, serverConfigkey)
	if !ok {
		panic("No template in context")
	}

	serverConfig, ok := val.(*ServerConfiguration)
	if !ok {
		panic("No string template in context")
	}

	return serverConfig
}

// ContextMiddleWare placeholder to attach methods to
type ContextMiddleWare struct {
}

func (c *ContextMiddleWare) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if server.templateDir == "" {
		panic("No templateDir defined!")
	}
	context.Set(r, serverConfigkey, &server)
	next(rw, r)
}

// NewContextMiddleWare create new ContextMiddleWare
func NewContextMiddleWare() *ContextMiddleWare {
	return &ContextMiddleWare{}
}

// NewStatic create a new negroni.Static router.
func NewStatic(directory, prefix, index string) *negroni.Static {
	return &negroni.Static{
		Dir:       http.Dir(directory),
		Prefix:    prefix,
		IndexFile: index,
	}
}

var router = mux.NewRouter()

func run(configuration Configuration) {
	middlewares := negroni.New(negroni.NewRecovery(), negroni.NewLogger(), NewStatic("/public", "/static", ""), NewContextMiddleWare())
	middlewares.UseHandler(router)
	middlewares.Run(fmt.Sprintf("%s:%d", configuration.Server.bindAddress, configuration.Server.port))
}
