package router_test

import (
	"testing"

	"github.com/bastion-framework/bast/router"
)

// Match is the result returned by router.Find for test assertions.
type Match = router.Match

func TestRouter_StaticRoute(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "handlerUsers")
	r.Add("GET", "/users/me", "handlerMe")

	m, err := r.Find("GET", "/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerUsers" {
		t.Errorf("handler = %v, want handlerUsers", m.Handler)
	}

	m, err = r.Find("GET", "/users/me")
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

	m, err := r.Find("GET", "/users/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerGetUser" {
		t.Errorf("handler = %v, want handlerGetUser", m.Handler)
	}
	if m.Params["id"] != "42" {
		t.Errorf("param id = %q, want 42", m.Params["id"])
	}

	m, err = r.Find("GET", "/users/99/posts/7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Params["id"] != "99" {
		t.Errorf("param id = %q, want 99", m.Params["id"])
	}
	if m.Params["postID"] != "7" {
		t.Errorf("param postID = %q, want 7", m.Params["postID"])
	}
}

func TestRouter_StaticVsParam(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users/me", "handlerMe")
	r.Add("GET", "/users/:id", "handlerID")

	m, err := r.Find("GET", "/users/me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerMe" {
		t.Errorf("static should win: handler = %v, want handlerMe", m.Handler)
	}
	if len(m.Params) != 0 {
		t.Errorf("static match should have no params, got %v", m.Params)
	}

	m, err = r.Find("GET", "/users/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerID" {
		t.Errorf("handler = %v, want handlerID", m.Handler)
	}
	if m.Params["id"] != "123" {
		t.Errorf("param id = %q, want 123", m.Params["id"])
	}
}

func TestRouter_Wildcard(t *testing.T) {
	r := router.New()
	r.Add("GET", "/static/*filepath", "handlerStatic")

	m, err := r.Find("GET", "/static/js/app.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "handlerStatic" {
		t.Errorf("handler = %v, want handlerStatic", m.Handler)
	}
	if m.Params["filepath"] != "js/app.js" {
		t.Errorf("param filepath = %q, want js/app.js", m.Params["filepath"])
	}

	m, err = r.Find("GET", "/static/img/logo.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Params["filepath"] != "img/logo.png" {
		t.Errorf("param filepath = %q, want img/logo.png", m.Params["filepath"])
	}
}

func TestRouter_NotFound(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "h")

	_, err := r.Find("GET", "/missing")
	if err != router.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	r := router.New()
	r.Add("GET", "/users", "hGet")
	r.Add("POST", "/users", "hPost")

	_, err := r.Find("DELETE", "/users")
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

	// Exact match works.
	m, err := r.Find("GET", "/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "h" {
		t.Errorf("handler = %v, want h", m.Handler)
	}

	// Trailing slash on a non-trailing-slash route → not found.
	_, err = r.Find("GET", "/users/")
	if err != router.ErrNotFound {
		t.Errorf("expected ErrNotFound for trailing slash, got %v", err)
	}
}

func TestRouter_RootRoute(t *testing.T) {
	r := router.New()
	r.Add("GET", "/", "handlerRoot")

	m, err := r.Find("GET", "/")
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
		m, err := r.Find(tc.method, "/items/"+tc.paramID)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.method, err)
		}
		if m.Handler != tc.want {
			t.Errorf("%s: handler = %v, want %v", tc.method, m.Handler, tc.want)
		}
		if m.Params["id"] != tc.paramID {
			t.Errorf("%s: param id = %q, want %q", tc.method, m.Params["id"], tc.paramID)
		}
	}
}

func TestRouter_DeepNestedParams(t *testing.T) {
	r := router.New()
	r.Add("GET", "/a/:b/c/:d/e/:f", "deep")

	m, err := r.Find("GET", "/a/1/c/2/e/3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "deep" {
		t.Errorf("handler = %v, want deep", m.Handler)
	}
	if m.Params["b"] != "1" || m.Params["d"] != "2" || m.Params["f"] != "3" {
		t.Errorf("params = %v", m.Params)
	}
}

func TestRouter_WildcardAfterStatic(t *testing.T) {
	r := router.New()
	r.Add("GET", "/files/docs", "hDocs")
	r.Add("GET", "/files/*path", "hWild")

	m, err := r.Find("GET", "/files/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "hDocs" {
		t.Errorf("static should win over wildcard: handler = %v, want hDocs", m.Handler)
	}

	m, err = r.Find("GET", "/files/images/photo.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Handler != "hWild" {
		t.Errorf("handler = %v, want hWild", m.Handler)
	}
	if m.Params["path"] != "images/photo.jpg" {
		t.Errorf("param path = %q, want images/photo.jpg", m.Params["path"])
	}
}