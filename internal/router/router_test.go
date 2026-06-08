package router_test

import (
	"testing"

	"github.com/bastion-framework/bast/internal/router"
)

// find is a test helper that wraps Find with a pre-allocated param buffer.
func find(r *router.Router, method, path string) (router.Match, error) {
	var buf [8]router.Param
	return r.Find(method, path, buf[:0])
}

// param does a linear lookup in a []Param — mirrors how Ctx.Param works.
func param(params []router.Param, key string) string {
	for i := range params {
		if params[i].Key == key {
			return params[i].Value
		}
	}
	return ""
}

func TestRouter_StaticRoute(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "handlerUsers")
	r.Add("GET", "/users/me", "handlerMe")

	m, err := find(r, "GET", "/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerUsers" {
		t.Errorf("handler = %v, want handlerUsers", m.Handler)
	}

	m, err = find(r, "GET", "/users/me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerMe" {
		t.Errorf("handler = %v, want handlerMe", m.Handler)
	}
}

func TestRouter_ParamExtraction(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users/:id", "handlerGetUser")
	r.Add("GET", "/users/:id/posts/:postID", "handlerGetPost")

	m, err := find(r, "GET", "/users/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerGetUser" {
		t.Errorf("handler = %v, want handlerGetUser", m.Handler)
	}
	if param(m.Params, "id") != "42" {
		t.Errorf("param id = %q, want 42", param(m.Params, "id"))
	}

	m, err = find(r, "GET", "/users/99/posts/7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if param(m.Params, "id") != "99" {
		t.Errorf("param id = %q, want 99", param(m.Params, "id"))
	}
	if param(m.Params, "postID") != "7" {
		t.Errorf("param postID = %q, want 7", param(m.Params, "postID"))
	}
}

func TestRouter_StaticVsParam(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users/me", "handlerMe")
	r.Add("GET", "/users/:id", "handlerID")

	m, err := find(r, "GET", "/users/me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerMe" {
		t.Errorf("static should win: handler = %v, want handlerMe", m.Handler)
	}
	if len(m.Params) != 0 {
		t.Errorf("static match should have no params, got %v", m.Params)
	}

	m, err = find(r, "GET", "/users/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerID" {
		t.Errorf("handler = %v, want handlerID", m.Handler)
	}
	if param(m.Params, "id") != "123" {
		t.Errorf("param id = %q, want 123", param(m.Params, "id"))
	}
}

func TestRouter_Wildcard(t *testing.T) {
	r := router.New()
	r.Add("GET", "/static/*filepath", "handlerStatic")

	m, err := find(r, "GET", "/static/js/app.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerStatic" {
		t.Errorf("handler = %v, want handlerStatic", m.Handler)
	}
	if param(m.Params, "filepath") != "js/app.js" {
		t.Errorf("param filepath = %q, want js/app.js", param(m.Params, "filepath"))
	}

	m, err = find(r, "GET", "/static/img/logo.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if param(m.Params, "filepath") != "img/logo.png" {
		t.Errorf("param filepath = %q, want img/logo.png", param(m.Params, "filepath"))
	}
}

func TestRouter_NotFound(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "h")

	_, err := find(r, "GET", "/missing")
	if err != router.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "hGet")
	r.Add("POST", "/users", "hPost")

	_, err := find(r, "DELETE", "/users")
	if err == nil {
		t.Fatal("expected error for wrong method, got nil")
	}
	mnaErr, ok := err.(*router.MethodNotAllowedError)
	if !ok {
		t.Fatalf("expected *router.MethodNotAllowedError, got %T: %v", err, err)
	}
	if len(mnaErr.Allow) == 0 {
		t.Error("Allow list must not be empty")
	}
	foundGet := false
	for _, m := range mnaErr.Allow {
		if m == "GET" {
			foundGet = true
		}
	}
	if !foundGet {
		t.Errorf("Allow list should contain GET, got %v", mnaErr.Allow)
	}
}

func TestRouter_TrailingSlash(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "h")

	m, err := find(r, "GET", "/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "h" {
		t.Errorf("handler = %v, want h", m.Handler)
	}

	_, err = find(r, "GET", "/users/")
	if err != router.ErrNotFound {
		t.Errorf("expected ErrNotFound for trailing slash, got %v", err)
	}
}

func TestRouter_RootRoute(t *testing.T) {
	r := router.New()
	r.Add("GET", "/", "handlerRoot")

	m, err := find(r, "GET", "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerRoot" {
		t.Errorf("handler = %v, want handlerRoot", m.Handler)
	}
}

func TestRouter_MultipleHTTPMethods(t *testing.T) {
	r := router.New()
	r.Add("GET", "/items/:id", "hGet")
	r.Add("PUT", "/items/:id", "hPut")
	r.Add("DELETE", "/items/:id", "hDel")

	for _, tc := range []struct {
		method  string
		want    string
		paramID string
	}{
		{"GET", "hGet", "1"},
		{"PUT", "hPut", "2"},
		{"DELETE", "hDel", "3"},
	} {
		m, err := find(r, tc.method, "/items/"+tc.paramID)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.method, err)
		}
		if m.Handler != tc.want {
			t.Errorf("%s: handler = %v, want %v", tc.method, m.Handler, tc.want)
		}
		if param(m.Params, "id") != tc.paramID {
			t.Errorf("%s: param id = %q, want %q", tc.method, param(m.Params, "id"), tc.paramID)
		}
	}
}

func TestRouter_DeepNestedParams(t *testing.T) {
	r := router.New()
	r.Add("GET", "/a/:b/c/:d/e/:f", "deep")

	m, err := find(r, "GET", "/a/1/c/2/e/3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "deep" {
		t.Errorf("handler = %v, want deep", m.Handler)
	}
	if param(m.Params, "b") != "1" || param(m.Params, "d") != "2" || param(m.Params, "f") != "3" {
		t.Errorf("params = %v", m.Params)
	}
}

func TestRouter_WildcardAfterStatic(t *testing.T) {
	r := router.New()
	r.Add("GET", "/files/docs", "hDocs")
	r.Add("GET", "/files/*path", "hWild")

	m, err := find(r, "GET", "/files/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "hDocs" {
		t.Errorf("static should win over wildcard: handler = %v, want hDocs", m.Handler)
	}

	m, err = find(r, "GET", "/files/images/photo.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "hWild" {
		t.Errorf("handler = %v, want hWild", m.Handler)
	}
	if param(m.Params, "path") != "images/photo.jpg" {
		t.Errorf("param path = %q, want images/photo.jpg", param(m.Params, "path"))
	}
}

// ─── Benchmarks ───────────────────────────────────────────────────────────────

func BenchmarkRouter_Static(b *testing.B) {
	r := router.New()
	r.Add("GET", "/users/me", "h")
	var buf [8]router.Param
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		r.Find("GET", "/users/me", buf[:0]) //nolint
	}
}

func BenchmarkRouter_Param(b *testing.B) {
	r := router.New()
	r.Add("GET", "/users/:id", "h")
	var buf [8]router.Param
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		r.Find("GET", "/users/42", buf[:0]) //nolint
	}
}

func BenchmarkRouter_DeepParam(b *testing.B) {
	r := router.New()
	r.Add("GET", "/a/:b/c/:d/e/:f/g/:h/i/:j", "h")
	var buf [8]router.Param
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		r.Find("GET", "/a/1/c/2/e/3/g/4/i/5", buf[:0]) //nolint
	}
}

func BenchmarkRouter_NotFound(b *testing.B) {
	r := router.New()
	r.Add("GET", "/users/:id", "h")
	var buf [8]router.Param
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		r.Find("GET", "/missing/path", buf[:0]) //nolint
	}
}