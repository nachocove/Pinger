package Pinger

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/negroni"
)


// NewStatic create a new negroni.Static router.
func NewStatic(directory, prefix, index string) *negroni.Static {
	return &negroni.Static{
		Dir:       http.Dir(directory),
		Prefix:    prefix,
		IndexFile: index,
	}
}

func NewRecovery(name string, printStack bool) *negroni.Recovery {
	return &negroni.Recovery{
		Logger:     log.New(os.Stdout, fmt.Sprintf("[%s] ", name), 0),
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
