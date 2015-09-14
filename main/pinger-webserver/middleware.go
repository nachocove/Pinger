package main

import (
	"github.com/gorilla/context"
	"net/http"
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
		panic("No serverContext in context")
	}

	context, ok := val.(*Context)
	if !ok {
		panic("No string template in context")
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
