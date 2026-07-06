package middleware

import (
	"strings"

	"github.com/bastion-framework/bast"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string // Use ["*"] to allow all origins.
	AllowedMethods   []string // Defaults to GET, POST, PUT, PATCH, DELETE, OPTIONS.
	AllowedHeaders   []string // Defaults to Content-Type, Authorization.
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // Preflight cache duration in seconds.
}

var defaultMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
var defaultHeaders = []string{"Content-Type", "Authorization"}

// CORS returns a middleware that applies Cross-Origin Resource Sharing headers.
// Panics on AllowedOrigins ["*"] combined with AllowCredentials: the Fetch spec
// forbids the pair and every browser rejects it, so it is always a config bug.
func CORS(cfg CORSConfig) bast.MiddlewareFunc {
	if cfg.AllowCredentials && isWildcard(cfg.AllowedOrigins) {
		panic(`middleware: CORS AllowedOrigins ["*"] cannot be combined with AllowCredentials`)
	}
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = defaultMethods
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = defaultHeaders
	}
	allowMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowedHeaders, ", ")

	return func(next bast.HandlerFunc) bast.HandlerFunc {
		return func(ctx *bast.Ctx) bast.Response {
			origin := ctx.Header("Origin")
			if origin == "" {
				return next(ctx)
			}

			allowed := matchOrigin(cfg.AllowedOrigins, origin)
			if !allowed {
				// A preflight from a disallowed origin must not reach the handler.
				// Reply without CORS headers — the browser blocks the real request.
				if ctx.Method() == "OPTIONS" && ctx.Header("Access-Control-Request-Method") != "" {
					return ctx.NoContent()
				}
				return next(ctx)
			}

			// Preflight — short-circuit before reaching the handler.
			if ctx.Method() == "OPTIONS" {
				resp := ctx.NoContent()
				if isWildcard(cfg.AllowedOrigins) {
					resp = resp.WithHeader("Access-Control-Allow-Origin", "*")
				} else {
					resp = resp.WithHeader("Access-Control-Allow-Origin", origin)
					resp = resp.WithHeader("Vary", "Origin")
				}
				resp = resp.WithHeader("Access-Control-Allow-Methods", allowMethods)
				resp = resp.WithHeader("Access-Control-Allow-Headers", allowHeaders)
				if cfg.AllowCredentials {
					resp = resp.WithHeader("Access-Control-Allow-Credentials", "true")
				}
				return resp
			}

			resp := next(ctx)
			if isWildcard(cfg.AllowedOrigins) {
				resp = resp.WithHeader("Access-Control-Allow-Origin", "*")
			} else {
				resp = resp.WithHeader("Access-Control-Allow-Origin", origin)
				resp = resp.WithHeader("Vary", "Origin")
			}
			if len(cfg.ExposedHeaders) > 0 {
				resp = resp.WithHeader("Access-Control-Expose-Headers",
					strings.Join(cfg.ExposedHeaders, ", "))
			}
			if cfg.AllowCredentials {
				resp = resp.WithHeader("Access-Control-Allow-Credentials", "true")
			}
			return resp
		}
	}
}

func matchOrigin(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}

func isWildcard(allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
	}
	return false
}