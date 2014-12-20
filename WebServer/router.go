package WebServer

import (
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/gorilla/context"
	"github.com/codegangsta/negroni"
)

type contextKey int

// Define keys that support equality.
const (
	serverConfigkey contextKey = iota
)

func GetServerConfig(r *http.Request) (*ServerConfiguration) {
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

type ContextMiddleWare struct {
	
}
func (c *ContextMiddleWare) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if Server.templateDir == "" {
		panic("No templateDir defined!")
	}
	context.Set(r, serverConfigkey, &Server)
	next(rw, r)
}
func NewContextMiddleWare() *ContextMiddleWare {
	return &ContextMiddleWare{}
}

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
