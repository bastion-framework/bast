// Package basttest provides test helpers for Bast handlers and apps.
// NewCtx builds a *bast.Ctx outside the pool — safe for unit tests, never pooled.
// NewApp wraps httptest for full integration tests through the module system.
package basttest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/bastion-framework/bast"
)

// CtxOption configures a *bast.Ctx built by NewCtx.
type CtxOption func(*bast.Ctx)

// NewCtx builds a *bast.Ctx outside the pool for unit testing.
// All fields are zero-valued unless overridden with options.
// Never call the pool release path on a test ctx — it is not pooled.
func NewCtx(opts ...CtxOption) *bast.Ctx {
	ctx := bast.NewTestCtx()
	// Wire a default request so methods like Method(), Path() don't need nil checks.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	bast.InitTestCtx(ctx, w, req)
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

// WithParam sets a route path parameter on the Ctx.
func WithParam(key, value string) CtxOption {
	return func(c *bast.Ctx) {
		bast.SetTestParam(c, key, value)
	}
}

// WithQuery sets a URL query parameter on the underlying request.
func WithQuery(key, value string) CtxOption {
	return func(c *bast.Ctx) {
		r := c.Request
		q := r.URL.Query()
		q.Set(key, value)
		r.URL.RawQuery = q.Encode()
	}
}

// WithHeader sets a request header on the Ctx.
func WithHeader(key, value string) CtxOption {
	return func(c *bast.Ctx) {
		c.Request.Header.Set(key, value)
	}
}

// WithMethod sets the HTTP method on the underlying request.
func WithMethod(method string) CtxOption {
	return func(c *bast.Ctx) {
		c.Request.Method = method
	}
}

// WithPath sets the URL path on the underlying request.
func WithPath(path string) CtxOption {
	return func(c *bast.Ctx) {
		c.Request.URL.Path = path
	}
}

// WithBody marshals v to JSON, sets Content-Type, and attaches it as the request body.
func WithBody(v any) CtxOption {
	return func(c *bast.Ctx) {
		b, _ := json.Marshal(v)
		c.Request.Body = io.NopCloser(bytes.NewReader(b))
		c.Request.ContentLength = int64(len(b))
		c.Request.Header.Set("Content-Type", "application/json")
		bast.SetTestBody(c, b)
	}
}

// WithRawBody sets raw bytes as the request body.
func WithRawBody(b []byte) CtxOption {
	return func(c *bast.Ctx) {
		c.Request.Body = io.NopCloser(bytes.NewReader(b))
		c.Request.ContentLength = int64(len(b))
		bast.SetTestBody(c, b)
	}
}

// WithStore pre-populates the Ctx store — simulates a guard having already run.
func WithStore(key string, val any) CtxOption {
	return func(c *bast.Ctx) {
		c.Set(key, val)
	}
}

// WithIP sets a spoofed RemoteAddr for IP testing.
func WithIP(ip string) CtxOption {
	return func(c *bast.Ctx) {
		c.Request.RemoteAddr = ip + ":0"
	}
}