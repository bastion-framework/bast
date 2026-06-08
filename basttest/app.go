package basttest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bastion-framework/bast"
)

// TestApp wraps a Bast app backed by httptest for integration testing.
// Tests the full request lifecycle: routing, guards, middleware, error boundary.
type TestApp struct {
	app *bast.App
	srv *httptest.Server
}

// NewApp builds a TestApp from one or more modules.
func NewApp(modules ...bast.Module) *TestApp {
	app := bast.New(bast.Config{})
	app.Register(modules...)
	srv := httptest.NewServer(app)
	return &TestApp{app: app, srv: srv}
}

// Close shuts down the test server. Call in TestMain or t.Cleanup.
func (a *TestApp) Close() {
	a.srv.Close()
}

// RequestOption configures a request sent via Do.
type RequestOption func(*http.Request)

// WithJSONBody marshals v to JSON and attaches it as the request body.
func WithJSONBody(v any) RequestOption {
	return func(r *http.Request) {
		b, _ := json.Marshal(v)
		r.Body = io.NopCloser(bytes.NewReader(b))
		r.ContentLength = int64(len(b))
		r.Header.Set("Content-Type", "application/json")
	}
}

// WithRequestHeader sets a header on the outgoing request.
func WithRequestHeader(key, value string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

// Do sends a request through the full app stack and returns a TestResponse.
func (a *TestApp) Do(method, path string, opts ...RequestOption) *TestResponse {
	req, err := http.NewRequest(method, a.srv.URL+path, nil)
	if err != nil {
		panic("basttest: failed to build request: " + err.Error())
	}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic("basttest: request failed: " + err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return &TestResponse{
		Code:    resp.StatusCode,
		Body:    body,
		Headers: resp.Header,
	}
}

// TestResponse holds the result of a TestApp.Do call.
type TestResponse struct {
	Code    int
	Body    []byte
	Headers http.Header
}

// JSON unmarshals the response body into v.
func (r *TestResponse) JSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Assert returns a fluent assertion helper bound to t.
func (r *TestResponse) Assert(t *testing.T) *Assertions {
	t.Helper()
	return &Assertions{t: t, resp: r}
}

// Assertions provides fluent assertions over a TestResponse.
type Assertions struct {
	t    *testing.T
	resp *TestResponse
}

// StatusIs asserts the response status code equals code.
func (a *Assertions) StatusIs(code int) *Assertions {
	a.t.Helper()
	if a.resp.Code != code {
		a.t.Errorf("status = %d, want %d (body: %s)", a.resp.Code, code, a.resp.Body)
	}
	return a
}

// BodyContains asserts the response body contains substr.
func (a *Assertions) BodyContains(substr string) *Assertions {
	a.t.Helper()
	if !bytes.Contains(a.resp.Body, []byte(substr)) {
		a.t.Errorf("body does not contain %q\nbody: %s", substr, a.resp.Body)
	}
	return a
}

// HeaderIs asserts the response header key equals value.
func (a *Assertions) HeaderIs(key, value string) *Assertions {
	a.t.Helper()
	got := a.resp.Headers.Get(key)
	if got != value {
		a.t.Errorf("header %q = %q, want %q", key, got, value)
	}
	return a
}