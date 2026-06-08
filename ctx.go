package bast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/bastion-framework/bast/internal/router"
)

const defaultMaxBodySize = 4 * 1024 * 1024 // 4MB

// Ctx is the request context passed to every Bast handler.
// Pooled via sync.Pool — reset and reused after every request.
//
// CRITICAL: *Ctx does NOT implement context.Context intentionally.
// This means you cannot pass *Ctx where context.Context is expected.
// The compiler enforces correct usage, always call ctx.Context() for
// passing to services, goroutines, or DB calls.
type Ctx struct {
	// detachedCtx is a copy of the request context, safe to pass anywhere.
	detachedCtx context.Context

	// Raw stdlib request.
	Request *http.Request

	// writer is unexported — handlers never write directly.
	writer http.ResponseWriter

	// paramStorage is pre-allocated inside the struct — no heap allocation for params.
	// params slices into paramStorage so router appends never escape to the heap.
	paramStorage [8]router.Param
	params       []router.Param

	// Internal store for middleware-injected typed values.
	store map[string]any

	// Buffered request body — read once, reused on subsequent calls.
	bodyBuf     []byte
	maxBodySize int64

	// trustedProxies is set by the app for IP resolution.
	trustedProxies []*net.IPNet

	// validator is used by Bind.
	validator Validator
}

// ctxPool manages *Ctx recycling.
var ctxPool = sync.Pool{
	New: func() any {
		c := &Ctx{store: make(map[string]any, 8)}
		c.params = c.paramStorage[:0]
		return c
	},
}

// acquireCtx gets a *Ctx from the pool and initializes it for the request.
func acquireCtx(w http.ResponseWriter, r *http.Request, maxBody int64, proxies []*net.IPNet, v Validator) *Ctx {
	c := ctxPool.Get().(*Ctx)
	c.Request = r
	c.writer = w
	c.maxBodySize = maxBody
	c.trustedProxies = proxies
	c.validator = v
	return c
}

// releaseCtx zeros all fields and returns the *Ctx to the pool.
func releaseCtx(c *Ctx) {
	c.wipe()
	ctxPool.Put(c)
}

// wipe zeros every field. No partial resets, no map allocation.
func (c *Ctx) wipe() {
	c.Request = nil
	c.writer = nil
	c.detachedCtx = nil
	c.bodyBuf = nil
	c.maxBodySize = 0
	c.trustedProxies = nil
	c.validator = nil
	c.params = c.paramStorage[:0] // reset length, keep backing array
	for k := range c.store {
		delete(c.store, k)
	}
}

// newTestCtx builds a *Ctx outside the pool for use in basttest.
// Never call releaseCtx on a test ctx — it is not pooled.
func newTestCtx() *Ctx {
	c := &Ctx{
		store:       make(map[string]any, 8),
		maxBodySize: defaultMaxBodySize,
	}
	c.params = c.paramStorage[:0]
	return c
}

// NewTestCtx is the exported form for basttest only.
func NewTestCtx() *Ctx {
	return newTestCtx()
}

// BenchAcquireCtx and BenchReleaseCtx expose the pool for benchmark tests.
func BenchAcquireCtx(w http.ResponseWriter, r *http.Request) *Ctx {
	return acquireCtx(w, r, defaultMaxBodySize, nil, nil)
}

func BenchReleaseCtx(c *Ctx) { releaseCtx(c) }

// NewTestCtxWithPath builds a test Ctx with a specific URL path set.
func NewTestCtxWithPath(path string) *Ctx {
	c := newTestCtx()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	c.Request = req
	return c
}

// InitTestCtx wires a writer and request onto a test Ctx. For basttest only.
func InitTestCtx(c *Ctx, w http.ResponseWriter, r *http.Request) {
	c.Request = r
	c.writer = w
}

// SetTestParam sets a route param on a test Ctx. For basttest only.
func SetTestParam(c *Ctx, key, value string) {
	n := len(c.params)
	if n < len(c.paramStorage) {
		c.paramStorage[n] = router.Param{Key: key, Value: value}
		c.params = c.paramStorage[:n+1]
	}
}

// SetTestBody pre-loads the body buffer on a test Ctx so readBody() is skipped.
// For basttest only.
func SetTestBody(c *Ctx, b []byte) {
	c.bodyBuf = b
}

// --- Context propagation ---

// Context returns a detached context.Context safe to pass anywhere.
func (c *Ctx) Context() context.Context {
	if c.detachedCtx == nil {
		if c.Request != nil {
			c.detachedCtx = c.Request.Context()
		} else {
			c.detachedCtx = context.Background()
		}
	}
	return c.detachedCtx
}

// WithValue returns a shallow copy of Ctx with a value injected.
func (c *Ctx) WithValue(key, val any) *Ctx {
	copy := *c
	copy.detachedCtx = context.WithValue(c.Context(), key, val)
	return &copy
}

// --- Request access ---

// Param returns a URL path parameter by name.
func (c *Ctx) Param(key string) string {
	for i := range c.params {
		if c.params[i].Key == key {
			return c.params[i].Value
		}
	}
	return ""
}

// Query returns a URL query parameter by name.
func (c *Ctx) Query(key string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.URL.Query().Get(key)
}

// QueryDefault returns a query param or a fallback if missing.
func (c *Ctx) QueryDefault(key, fallback string) string {
	if v := c.Query(key); v != "" {
		return v
	}
	return fallback
}

// Header returns a request header value by name.
func (c *Ctx) Header(key string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.Header.Get(key)
}

// IP returns the real client IP, respecting X-Forwarded-For and X-Real-IP
// only when the request comes from a trusted proxy.
func (c *Ctx) IP() string {
	if c.Request == nil {
		return ""
	}
	remoteIP := stripPort(c.Request.RemoteAddr)
	if c.isTrustedProxy(remoteIP) {
		if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first (leftmost) IP — the original client.
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		if xri := c.Request.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	return remoteIP
}

func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func (c *Ctx) isTrustedProxy(ip string) bool {
	if len(c.trustedProxies) == 0 {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range c.trustedProxies {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

// Method returns the HTTP method of the request.
func (c *Ctx) Method() string {
	if c.Request == nil {
		return ""
	}
	return c.Request.Method
}

// Path returns the request URL path.
func (c *Ctx) Path() string {
	if c.Request == nil {
		return ""
	}
	return c.Request.URL.Path
}

// --- Body parsing ---

func (c *Ctx) readBody() error {
	if c.bodyBuf != nil {
		return nil
	}
	limit := c.maxBodySize
	if limit == 0 {
		limit = defaultMaxBodySize
	}
	lr := io.LimitReader(c.Request.Body, limit)
	buf, err := io.ReadAll(lr)
	if err != nil {
		return fmt.Errorf("bast: read body: %w", err)
	}
	c.bodyBuf = buf
	c.Request.Body = io.NopCloser(bytes.NewReader(buf))
	return nil
}

// RawBody reads and returns the request body bytes. Safe to call multiple times.
func (c *Ctx) RawBody() ([]byte, error) {
	if err := c.readBody(); err != nil {
		return nil, err
	}
	return c.bodyBuf, nil
}

// BindJSON decodes JSON body into v without running validation.
func (c *Ctx) BindJSON(v any) error {
	if v == nil {
		return fmt.Errorf("bast: BindJSON: target must not be nil")
	}
	if err := c.readBody(); err != nil {
		return err
	}
	if err := json.Unmarshal(c.bodyBuf, v); err != nil {
		return fmt.Errorf("bast: decode JSON: %w", err)
	}
	return nil
}

// Bind decodes and validates the request body into v.
func (c *Ctx) Bind(v any) error {
	if v == nil {
		return fmt.Errorf("bast: Bind: target must not be nil")
	}
	if err := c.readBody(); err != nil {
		return err
	}
	if err := json.Unmarshal(c.bodyBuf, v); err != nil {
		return fmt.Errorf("bast: decode JSON: %w", err)
	}
	if c.validator != nil {
		if err := c.validator.Validate(v); err != nil {
			return err
		}
	}
	return nil
}

// FormValue reads a form field (multipart or url-encoded).
func (c *Ctx) FormValue(key string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.FormValue(key)
}

// BindForm binds a multipart or url-encoded form into a struct.
// Uses reflection — only called by user, never on the hot path.
func (c *Ctx) BindForm(v any) error {
	if v == nil {
		return fmt.Errorf("bast: BindForm: target must not be nil")
	}
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("bast: parse form: %w", err)
	}
	return nil
}

// File retrieves a single uploaded file by field name.
func (c *Ctx) File(field string) (*multipart.FileHeader, error) {
	_, fh, err := c.Request.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("bast: file %q: %w", field, err)
	}
	return fh, nil
}

// Files retrieves multiple uploaded files from one field.
func (c *Ctx) Files(field string) ([]*multipart.FileHeader, error) {
	if err := c.Request.ParseMultipartForm(c.maxBodySize); err != nil {
		return nil, fmt.Errorf("bast: parse multipart: %w", err)
	}
	fhs := c.Request.MultipartForm.File[field]
	return fhs, nil
}

// Cookie retrieves a named cookie from the request.
func (c *Ctx) Cookie(name string) (*http.Cookie, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return nil, fmt.Errorf("bast: cookie %q: %w", name, err)
	}
	return cookie, nil
}

// Cookies returns all cookies from the request.
func (c *Ctx) Cookies() []*http.Cookie {
	if c.Request == nil {
		return nil
	}
	return c.Request.Cookies()
}

// --- Middleware store ---

// Set stores a value in the request-scoped store.
func (c *Ctx) Set(key string, val any) {
	c.store[key] = val
}

// Get retrieves a value from the request-scoped store.
func (c *Ctx) Get(key string) (any, bool) {
	v, ok := c.store[key]
	return v, ok
}

// MustGet retrieves a value and panics if not found.
// Use only when a guard guarantees the value was set.
func (c *Ctx) MustGet(key string) any {
	v, ok := c.store[key]
	if !ok {
		panic(fmt.Sprintf("bast: MustGet: key %q not found in store", key))
	}
	return v
}

// --- Package-level typed accessors ---

// Get returns a typed value from the Ctx store. No type assertion needed.
func Get[T any](ctx *Ctx, key string) (T, bool) {
	val, ok := ctx.store[key]
	if !ok {
		var zero T
		return zero, false
	}
	typed, ok := val.(T)
	return typed, ok
}

// MustGet returns a typed value and panics if missing or wrong type.
func MustGet[T any](ctx *Ctx, key string) T {
	val, ok := Get[T](ctx, key)
	if !ok {
		panic(fmt.Sprintf("bast: MustGet: key %q not found or wrong type", key))
	}
	return val
}

// --- Response builders ---

func (c *Ctx) jsonBody(data any) ([]byte, error) {
	b, err := json.Marshal(struct {
		Data any `json:"data"`
		Meta any `json:"meta"`
	}{Data: data, Meta: nil})
	if err != nil {
		return nil, fmt.Errorf("bast: marshal response: %w", err)
	}
	return b, nil
}

// OK returns a 200 response with a JSON envelope.
func (c *Ctx) OK(data any) Response {
	b, err := c.jsonBody(data)
	if err != nil {
		return c.Error(err)
	}
	return newRawResponse(200, "application/json", b)
}

// Created returns a 201 response with a JSON envelope.
func (c *Ctx) Created(data any) Response {
	b, err := c.jsonBody(data)
	if err != nil {
		return c.Error(err)
	}
	return newRawResponse(201, "application/json", b)
}

// NoContent returns a 204 response with no body.
func (c *Ctx) NoContent() Response {
	return newRawResponse(204, "", nil)
}

// Redirect returns a redirect response.
func (c *Ctx) Redirect(url string, code int) Response {
	r := newRawResponse(code, "", nil)
	r.redirect = url
	return r
}

// Error returns a response carrying an error for the boundary.
func (c *Ctx) Error(err error) Response {
	return newErrorResponse(err)
}

// Raw returns a response with a raw body. Used for escape-hatch streaming.
func (c *Ctx) Raw(status int, contentType string, body []byte) Response {
	return newRawResponse(status, contentType, body)
}

// Paginated returns a 200 response with data and pagination metadata.
func (c *Ctx) Paginated(data any, meta PaginationMeta) Response {
	b, err := json.Marshal(struct {
		Data any            `json:"data"`
		Meta PaginationMeta `json:"meta"`
	}{Data: data, Meta: meta})
	if err != nil {
		return c.Error(err)
	}
	return newRawResponse(200, "application/json", b)
}

// PaginationMeta holds standard pagination information.
type PaginationMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
	Pages   int `json:"pages"`
}

// WithCookie chains a cookie onto a response.
func (c *Ctx) WithCookie(cookie *http.Cookie) Response {
	return newRawResponse(200, "", nil).WithCookie(cookie)
}

// Validator is a pluggable struct validation interface.
type Validator interface {
	Validate(v any) error
}