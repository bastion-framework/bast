package bast

import "time"

// HandlerFunc is the signature every Bast handler must have.
// No variadic args, no interface{}, no side effects.
type HandlerFunc func(ctx *Ctx) Response

// StreamHandlerFunc is the signature for streaming handlers.
// Does not return a Response, the handler owns the wire for its full duration.
type StreamHandlerFunc func(ctx *StreamCtx)

// RouteOption applies optional config to a route at construction time.
type RouteOption func(*Route)

// WithMiddleware attaches middleware to a specific route.
func WithMiddleware(mw ...MiddlewareFunc) RouteOption {
	return func(r *Route) {
		r.Middleware = append(r.Middleware, mw...)
	}
}

// WithGuards attaches guards to a specific route.
func WithGuards(guards ...Guard) RouteOption {
	return func(r *Route) {
		r.Guards = append(r.Guards, guards...)
	}
}

// WithTimeout sets a per-route handler timeout.
func WithTimeout(d time.Duration) RouteOption {
	return func(r *Route) {
		r.Timeout = d
	}
}

// WithMaxBody sets a per-route request body size limit in bytes.
func WithMaxBody(n int64) RouteOption {
	return func(r *Route) {
		r.MaxBodySize = n
	}
}

// Doc holds human-readable documentation for a route, used to build OpenAPI specs.
type Doc struct {
	Summary       string
	Description   string
	Tags          []string
	Params        []Param
	Body          *BodySchema
	Returns       Returns
	Deprecated    bool
	DeprecatedMsg string
}

// Param describes a single route parameter for OpenAPI docs.
type Param struct {
	In          string // "path", "query", "header"
	Name        string
	Description string
	Required    bool
}

// PathParam constructs a path parameter descriptor.
func PathParam(name, description string) Param {
	return Param{In: "path", Name: name, Description: description, Required: true}
}

// QueryParam constructs a query parameter descriptor.
func QueryParam(name, description string) Param {
	return Param{In: "query", Name: name, Description: description}
}

// BodySchema holds a reflected type for OpenAPI schema generation.
// Populated via Body[T]() — reflection runs once at startup, never on hot path.
type BodySchema struct {
	Example any
}

// Returns maps HTTP status codes to response body schemas.
type Returns map[int]*BodySchema

// Body captures a type for OpenAPI schema generation.
// Reflection runs once at startup — zero cost at request time.
func Body[T any]() *BodySchema {
	var zero T
	return &BodySchema{Example: zero}
}

// WithDoc attaches documentation to a route.
func WithDoc(doc Doc) RouteOption {
	return func(r *Route) {
		r.Doc = doc
	}
}