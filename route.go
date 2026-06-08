package bast

import "time"

// Route binds an HTTP method + path pattern to a handler.
type Route struct {
	Method      string
	Pattern     string
	Handler     HandlerFunc
	Stream      StreamHandlerFunc // non-nil when this is a streaming route
	Middleware  []MiddlewareFunc
	Guards      []Guard
	Timeout     time.Duration
	MaxBodySize int64
	Doc         Doc
}

// GET constructs a GET route.
func GET(pattern string, handler HandlerFunc, opts ...RouteOption) Route {
	return newRoute("GET", pattern, handler, opts)
}

// POST constructs a POST route.
func POST(pattern string, handler HandlerFunc, opts ...RouteOption) Route {
	return newRoute("POST", pattern, handler, opts)
}

// PUT constructs a PUT route.
func PUT(pattern string, handler HandlerFunc, opts ...RouteOption) Route {
	return newRoute("PUT", pattern, handler, opts)
}

// PATCH constructs a PATCH route.
func PATCH(pattern string, handler HandlerFunc, opts ...RouteOption) Route {
	return newRoute("PATCH", pattern, handler, opts)
}

// DELETE constructs a DELETE route.
func DELETE(pattern string, handler HandlerFunc, opts ...RouteOption) Route {
	return newRoute("DELETE", pattern, handler, opts)
}

// STREAM constructs a streaming route. The handler owns the connection for its lifetime.
func STREAM(pattern string, handler StreamHandlerFunc, opts ...RouteOption) Route {
	r := Route{
		Method:  "GET",
		Pattern: pattern,
		Stream:  handler,
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

func newRoute(method, pattern string, handler HandlerFunc, opts []RouteOption) Route {
	r := Route{
		Method:  method,
		Pattern: pattern,
		Handler: handler,
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}