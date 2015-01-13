package WebServer

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
	"net/http"
	"strings"
)

type contextKey int

// Define keys that support equality.
const (
	configKey contextKey = iota
)

// GetServerConfig get the server config from the context
func GetConfig(r *http.Request) *Configuration {
	val, ok := context.GetOk(r, configKey)
	if !ok {
		log.Fatal("No template in context")
	}

	config, ok := val.(*Configuration)
	if !ok {
		log.Fatal("No string template in context")
	}

	return config
}

// ContextMiddleWare placeholder to attach methods to
type ContextMiddleWare struct {
	config *Configuration
}

func (c *ContextMiddleWare) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	context.Set(r, configKey, c.config)
	next(rw, r)
}

// NewContextMiddleWare create new ContextMiddleWare
func NewContextMiddleWare(config *Configuration) *ContextMiddleWare {
	return &ContextMiddleWare{config: config}
}

// NewStatic create a new negroni.Static router.
func NewStatic(directory, prefix, index string) *negroni.Static {
	return &negroni.Static{
		Dir:       http.Dir(directory),
		Prefix:    prefix,
		IndexFile: index,
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
	log.Info("Redirecting to %s\n", redirAddr)
	http.Redirect(rw, r, redirAddr, http.StatusMovedPermanently)
}

func NewRedirectMiddleware(host string, port int) *RedirectMiddleWare {
	return &RedirectMiddleWare{redirectPort: port, redirectHost: host}
}
