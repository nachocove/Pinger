package main

import (
	"log"
	"net/http"
	"net/http/httputil"

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
	if context.Config.Global.DumpRequests {
		responseBytes, err := httputil.DumpRequest(r, true)
		if err != nil {
			context.Logger.Error("Could not dump request %+v", r)
		} else {
			context.Logger.Warning("Request:\n%s", responseBytes)
		}
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
