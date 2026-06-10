package basttest

import (
	"net/http"
	"net/http/httptest"

	"github.com/bastion-framework/bast"
)

// StreamCtxOption configures a *bast.StreamCtx built by NewStreamCtx.
type StreamCtxOption func(*bast.StreamCtx)

// NewStreamCtx builds a *bast.StreamCtx outside the pool for unit testing stream handlers.
// All fields are zero-valued unless overridden with options.
// Never pool or reuse a test StreamCtx.
func NewStreamCtx(opts ...StreamCtxOption) *bast.StreamCtx {
	sctx := bast.NewTestStreamCtx()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	sctx.Request = req
	for _, opt := range opts {
		opt(sctx)
	}
	return sctx
}

// WithStreamParam sets a route path parameter on the StreamCtx.
func WithStreamParam(key, value string) StreamCtxOption {
	return func(s *bast.StreamCtx) {
		bast.SetStreamTestParam(s, key, value)
	}
}

// WithStreamStore pre-populates the StreamCtx store — simulates a guard having already run.
func WithStreamStore(key string, val any) StreamCtxOption {
	return func(s *bast.StreamCtx) {
		bast.SetStreamTestStore(s, key, val)
	}
}

// WithStreamHeader sets a request header on the StreamCtx.
func WithStreamHeader(key, value string) StreamCtxOption {
	return func(s *bast.StreamCtx) {
		s.Request.Header.Set(key, value)
	}
}

// WithStreamMethod sets the HTTP method on the underlying request.
func WithStreamMethod(method string) StreamCtxOption {
	return func(s *bast.StreamCtx) {
		s.Request.Method = method
	}
}

// WithStreamPath sets the URL path on the underlying request.
func WithStreamPath(path string) StreamCtxOption {
	return func(s *bast.StreamCtx) {
		s.Request.URL.Path = path
	}
}
