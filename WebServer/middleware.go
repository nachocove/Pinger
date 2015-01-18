package WebServer

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
)

type contextKey int

// Define keys that support equality.
const (
	serverContext contextKey = iota
)

// GetServerConfig get the server config from the context
func GetContext(r *http.Request) *Context {
	val, ok := context.GetOk(r, serverContext)
	if !ok {
		log.Fatal("No template in context")
	}

	context, ok := val.(*Context)
	if !ok {
		log.Fatal("No string template in context")
	}

	return context
}

// ContextMiddleWare placeholder to attach methods to
type ContextMiddleWare struct {
	context *Context
}

func (c *ContextMiddleWare) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	context.Set(r, serverContext, c.context)
	next(rw, r)
}

// NewContextMiddleWare create new ContextMiddleWare
func NewContextMiddleWare(context *Context) *ContextMiddleWare {
	return &ContextMiddleWare{context: context}
}

// NewStatic create a new negroni.Static router.
func NewStatic(directory, prefix, index string) *negroni.Static {
	return &negroni.Static{
		Dir:       http.Dir(directory),
		Prefix:    prefix,
		IndexFile: index,
	}
}

func NewRecovery(printStack bool) *negroni.Recovery {
	return &negroni.Recovery{
		Logger:     log.New(os.Stdout, "[negroni] ", 0),
		PrintStack: printStack,
		StackAll:   false,
		StackSize:  1024 * 8,
	}
}

type RedirectMiddleWare struct {
	redirectPort int
	redirectHost string
}

func (redir *RedirectMiddleWare) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	var host string
	switch {
	case redir.redirectHost != "":
		host = redir.redirectHost

	case r.Host != "":
		host = strings.Split(r.Host, ":")[0]
	}
	var portString string
	if redir.redirectPort > 0 {
		portString = fmt.Sprintf(":%d", redir.redirectPort)
	} else {
		portString = ""
	}
	redirAddr := fmt.Sprintf("https://%s%s%s", host, portString, r.RequestURI)
	http.Redirect(rw, r, redirAddr, http.StatusMovedPermanently)
}

func NewRedirectMiddleware(host string, port int) *RedirectMiddleWare {
	return &RedirectMiddleWare{redirectPort: port, redirectHost: host}
}
