package bast

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bastion-framework/bast/internal/jsonx"
	"github.com/bastion-framework/bast/internal/router"
)

const defaultMaxBodySize = 4 * 1024 * 1024 // 4MB

// nullEnvelope is the pre-baked JSON envelope for nil data.
// Returned directly from jsonBody when data == nil — zero allocations.
var nullEnvelope = []byte(`{"data":null,"meta":null}`)

// jsonEnvelope is the response envelope type, pooled to avoid heap escapes.
type jsonEnvelope struct {
	Data any `json:"data"`
	Meta any `json:"meta"`
}

var envPool = sync.Pool{New: func() any { return new(jsonEnvelope) }}

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

	Request *http.Request
	writer  http.ResponseWriter

	// params slices into paramStorage; both live in the struct — zero heap alloc on the hot path.
	paramStorage [8]router.Param
	params       []router.Param

	store       map[string]any // lazily allocated on first Set; reused across pool cycles
	bodyBuf     []byte
	bodyOwned   *bytes.Buffer // pooled buffer backing bodyBuf, returned on wipe
	maxBodySize int64
	bodyLimited bool // true once Request.Body has been wrapped by limitBody

	trustedProxies []*net.IPNet
	validator      Validator
}

var ctxPool = sync.Pool{
	New: func() any {
		c := &Ctx{}
		c.params = c.paramStorage[:0]
		return c
	},
}

// bodyBufPool recycles request-body read buffers. Buffers above the cap are
// dropped so one giant upload doesn't pin memory for the process lifetime.
var bodyBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

const maxPooledBodyCap = 64 << 10

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
// Called only from releaseCtx via defer, so it runs even if a handler panics —
// the net/http server catches the panic after the defer unwinds.
func (c *Ctx) wipe() {
	c.Request = nil
	c.writer = nil
	c.detachedCtx = nil
	c.bodyBuf = nil
	if c.bodyOwned != nil {
		if c.bodyOwned.Cap() <= maxPooledBodyCap {
			bodyBufPool.Put(c.bodyOwned)
		}
		c.bodyOwned = nil
	}
	c.maxBodySize = 0
	c.bodyLimited = false
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

// WithValue returns a copy of Ctx with a value injected into its context.
// The copy is strictly request-scoped: it shares the pooled body buffer and
// Request with the original, so it must never outlive the handler.
func (c *Ctx) WithValue(key, val any) *Ctx {
	cp := *c
	// The struct copy duplicates paramStorage by value, but params still points
	// into the ORIGINAL's array — re-point it into the copy's own storage.
	cp.params = cp.paramStorage[:len(c.params)]
	// The store map is shared by reference; clone it so writes on either side
	// can't touch the other — the original's map is recycled into the pool.
	if len(c.store) > 0 {
		cp.store = make(map[string]any, len(c.store))
		for k, v := range c.store {
			cp.store[k] = v
		}
	} else {
		cp.store = nil
	}
	cp.detachedCtx = context.WithValue(c.Context(), key, val)
	return &cp
}



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
//
// X-Forwarded-For is walked right to left: each proxy appends the address it
// saw, so the rightmost entry NOT itself a trusted proxy is the real client.
// The leftmost entries are attacker-controlled — a client can send a forged
// XFF header and a trusted proxy will simply append the truth after it.
func (c *Ctx) IP() string {
	if c.Request == nil {
		return ""
	}
	remoteIP := stripPort(c.Request.RemoteAddr)
	if !c.isTrustedProxy(remoteIP) {
		return remoteIP
	}
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip == "" {
				continue
			}
			if !c.isTrustedProxy(ip) {
				return ip
			}
		}
		// Every hop is a trusted proxy — the chain's first entry is the origin.
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	if xri := strings.TrimSpace(c.Request.Header.Get("X-Real-IP")); xri != "" {
		return xri
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

// limitBody wraps Request.Body in http.MaxBytesReader exactly once using the
// resolved per-request limit, so every framework read path (Bind, RawBody,
// FormValue, File, Files, BindForm) is bounded uniformly — not just JSON. A
// negative maxBodySize disables the limit (opt-out); zero uses the default.
//
// Note: reading Request.Body directly, bypassing these accessors, is not bounded.
func (c *Ctx) limitBody() {
	if c.bodyLimited || c.Request == nil || c.Request.Body == nil {
		return
	}
	c.bodyLimited = true
	limit := c.maxBodySize
	if limit == 0 {
		limit = defaultMaxBodySize
	}
	if limit < 0 {
		return // explicitly disabled
	}
	if c.writer != nil {
		c.Request.Body = http.MaxBytesReader(c.writer, c.Request.Body, limit)
	} else {
		// No writer (e.g. unit tests): fall back to a hard cap without the
		// connection-close behaviour MaxBytesReader adds.
		c.Request.Body = http.MaxBytesReader(nil, c.Request.Body, limit)
	}
}

func (c *Ctx) readBody() error {
	if c.bodyBuf != nil {
		return nil
	}
	c.limitBody()
	buf := bodyBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	// Pre-size from Content-Length only when it fits inside a positive limit, so
	// an attacker-supplied Content-Length can never drive a large speculative
	// allocation.
	if cl := c.Request.ContentLength; cl > 0 && c.maxBodySize > 0 && cl <= c.maxBodySize {
		buf.Grow(int(cl))
	}
	if _, err := buf.ReadFrom(c.Request.Body); err != nil {
		bodyBufPool.Put(buf)
		// limitBody wraps the body in http.MaxBytesReader; an overflow surfaces
		// here as *http.MaxBytesError, which we map to a clean 413 instead of a
		// confusing 400/500 on a silently truncated body.
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return ErrPayloadTooLarge("request body exceeds the configured size limit")
		}
		return fmt.Errorf("bast: read body: %w", err)
	}
	c.bodyOwned = buf
	c.bodyBuf = buf.Bytes()
	if c.bodyBuf == nil {
		c.bodyBuf = []byte{} // empty body still marks readBody as done
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(c.bodyBuf))
	return nil
}

// RawBody reads and returns the request body bytes. Safe to call multiple times.
// The slice is backed by a pooled buffer: valid until the handler returns,
// copy it if you need to retain it longer.
func (c *Ctx) RawBody() ([]byte, error) {
	if err := c.readBody(); err != nil {
		return nil, err
	}
	return c.bodyBuf, nil
}

// BindJSON decodes JSON body into v without running validation.
// Returns a 400 BastError on malformed JSON so the error boundary
// produces a proper Bad Request response instead of a 500.
func (c *Ctx) BindJSON(v any) error {
	if v == nil {
		return fmt.Errorf("bast: BindJSON: target must not be nil")
	}
	if err := c.readBody(); err != nil {
		return err
	}
	if err := jsonx.Unmarshal(c.bodyBuf, v); err != nil {
		return ErrInvalidBody(err.Error())
	}
	return nil
}

// Bind decodes and validates the request body into v.
// Returns a 400 BastError on malformed JSON, or a ValidationError on
// failed struct validation — both flow cleanly through the error boundary.
func (c *Ctx) Bind(v any) error {
	if v == nil {
		return fmt.Errorf("bast: Bind: target must not be nil")
	}
	if err := c.readBody(); err != nil {
		return err
	}
	if err := jsonx.Unmarshal(c.bodyBuf, v); err != nil {
		return ErrInvalidBody(err.Error())
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
	c.limitBody()
	return c.Request.FormValue(key)
}

// BindForm binds a url-encoded or multipart form into a struct pointer.
// Fields are matched by their `form:"name"` tag, falling back to the field name.
// A tag of "-" skips the field. Uses reflection — never on the hot path.
//
// Supported field types: string, the signed/unsigned integer kinds, float32/64,
// bool, time.Duration, and []string (repeated form values). Invalid input yields
// a 400 BastError so it flows through the error boundary. Values populated by
// FormValue's r.Form (query + POST body) are used; multipart file parts are read
// via File/Files, not here.
func (c *Ctx) BindForm(v any) error {
	if v == nil {
		return fmt.Errorf("bast: BindForm: target must not be nil")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("bast: BindForm: target must be a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("bast: BindForm: target must point to a struct")
	}
	c.limitBody()
	if err := c.Request.ParseForm(); err != nil {
		return ErrBadRequest(CodeBadRequest, "malformed form body")
	}

	form := c.Request.Form
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		name := field.Tag.Get("form")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		vals, ok := form[name]
		if !ok || len(vals) == 0 {
			continue
		}
		if err := setFormField(fv, field.Type, vals, name); err != nil {
			return err
		}
	}
	return nil
}

// setFormField parses raw form value(s) into the target field. Parse failures
// return a 400 BastError so callers get a Bad Request, not a 500.
func setFormField(fv reflect.Value, ft reflect.Type, vals []string, name string) error {
	// []string captures every repeated value for this key.
	if ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.String {
		sl := reflect.MakeSlice(ft, len(vals), len(vals))
		for i, s := range vals {
			sl.Index(i).SetString(s)
		}
		fv.Set(sl)
		return nil
	}

	raw := vals[0]

	// time.Duration is an int64 under the hood; handle it before the integer
	// kinds so "5s" parses as a duration rather than an integer.
	if ft == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return ErrBadRequest(CodeBadRequest, fmt.Sprintf("field %q: invalid duration", name))
		}
		fv.SetInt(int64(d))
		return nil
	}

	switch ft.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return ErrBadRequest(CodeBadRequest, fmt.Sprintf("field %q: invalid integer", name))
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return ErrBadRequest(CodeBadRequest, fmt.Sprintf("field %q: invalid unsigned integer", name))
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return ErrBadRequest(CodeBadRequest, fmt.Sprintf("field %q: invalid number", name))
		}
		fv.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return ErrBadRequest(CodeBadRequest, fmt.Sprintf("field %q: invalid boolean", name))
		}
		fv.SetBool(b)
	default:
		return ErrBadRequest(CodeBadRequest, fmt.Sprintf("field %q: unsupported type %s", name, ft))
	}
	return nil
}

// File retrieves a single uploaded file by field name.
func (c *Ctx) File(field string) (*multipart.FileHeader, error) {
	c.limitBody()
	_, fh, err := c.Request.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("bast: file %q: %w", field, err)
	}
	return fh, nil
}

// Files retrieves multiple uploaded files from one field.
func (c *Ctx) Files(field string) ([]*multipart.FileHeader, error) {
	c.limitBody()
	// maxMemory bounds how much of the multipart body is buffered in RAM before
	// spilling to temp files; the total body is already capped by limitBody.
	mem := c.maxBodySize
	if mem <= 0 {
		mem = defaultMaxBodySize
	}
	if err := c.Request.ParseMultipartForm(mem); err != nil {
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
	if c.store == nil {
		c.store = make(map[string]any, 8)
	}
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
	if data == nil {
		return nullEnvelope, nil
	}
	env := envPool.Get().(*jsonEnvelope)
	env.Data = data
	b, err := jsonx.Marshal(env)
	env.Data = nil // clear before returning to pool to avoid extending data lifetime
	envPool.Put(env)
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

// paginatedEnvelope is pooled to avoid a heap allocation per Paginated call.
type paginatedEnvelope struct {
	Data any            `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

var paginatedPool = sync.Pool{New: func() any { return new(paginatedEnvelope) }}

// Paginated returns a 200 response with data and pagination metadata.
func (c *Ctx) Paginated(data any, meta PaginationMeta) Response {
	env := paginatedPool.Get().(*paginatedEnvelope)
	env.Data = data
	env.Meta = meta
	b, err := jsonx.Marshal(env)
	env.Data = nil
	paginatedPool.Put(env)
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