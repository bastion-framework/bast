package bast

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression tests for the redirect & body-limit & BindForm fixes.
type rc struct{ routes []Route }

func (c *rc) Routes() []Route { return c.routes }

// HIGH-1: relative redirect must NOT panic and must emit a 302 + Location.
func TestFix_RelativeRedirect(t *testing.T) {
	app := New(Config{})
	app.Register(Module{Prefix: "/r", Controller: &rc{[]Route{
		GET("/go", func(c *Ctx) Response { return c.Redirect("/login", 302) }),
	}}})
	req := httptest.NewRequest("GET", "/r/go", nil)
	w := httptest.NewRecorder()
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("still panics: %v", rec)
		}
	}()
	app.ServeHTTP(w, req)
	if w.Code != 302 || w.Header().Get("Location") != "/login" {
		t.Fatalf("got status=%d location=%q, want 302 /login", w.Code, w.Header().Get("Location"))
	}
	t.Logf("OK: status=%d location=%q", w.Code, w.Header().Get("Location"))
}

// CRLF in a redirect target must be stripped (no response splitting).
func TestFix_RedirectCRLFStripped(t *testing.T) {
	w := httptest.NewRecorder()
	resp := newRawResponse(302, "", nil)
	resp.redirect = "/next\r\nSet-Cookie: evil=1"
	writeResponse(w, resp)
	loc := w.Header().Get("Location")
	if strings.ContainsAny(loc, "\r\n") {
		t.Fatalf("CRLF not stripped: %q", loc)
	}
	t.Logf("OK: sanitized location=%q; injected Set-Cookie=%q", loc, w.Header().Get("Set-Cookie"))
}

// HIGH-2: BindForm must populate the struct.
func TestFix_BindFormPopulates(t *testing.T) {
	req := httptest.NewRequest("POST", "/?tag=a&tag=b", strings.NewReader("name=alice&age=30&admin=true&ttl=5s"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := newTestCtx()
	c.Request = req
	var target struct {
		Name  string   `form:"name"`
		Age   int      `form:"age"`
		Admin bool     `form:"admin"`
		Tags  []string `form:"tag"`
		Skip  string   `form:"-"`
	}
	if err := c.BindForm(&target); err != nil {
		t.Fatalf("BindForm err: %v", err)
	}
	if target.Name != "alice" || target.Age != 30 || !target.Admin || len(target.Tags) != 2 {
		t.Fatalf("not populated: %+v", target)
	}
	t.Logf("OK: %+v", target)
}

func TestFix_BindFormBadIntIs400(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("age=notanumber"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := newTestCtx()
	c.Request = req
	var target struct {
		Age int `form:"age"`
	}
	err := c.BindForm(&target)
	be, ok := err.(*BastError)
	if !ok || be.Status != 400 {
		t.Fatalf("want 400 BastError, got %v", err)
	}
	t.Logf("OK: %d %s", be.Status, be.Code)
}

// MEDIUM-4 + 413: multipart upload over the limit must be rejected as 413.
func TestFix_MultipartRespectsLimit(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("f", "big.bin")
	fw.Write(bytes.Repeat([]byte("A"), 2*1024*1024))
	mw.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c := newTestCtx()
	c.Request = req
	c.maxBodySize = 1024
	_, err := c.Files("f")
	if err == nil {
		t.Fatalf("2MB upload accepted under 1KB limit")
	}
	t.Logf("OK: over-limit upload rejected: %v", err)
}

// 413 for oversized JSON instead of a confusing 400.
func TestFix_OversizeJSONIs413(t *testing.T) {
	big := `{"name":"` + strings.Repeat("x", 5000) + `"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(big))
	c := newTestCtx()
	c.Request = req
	c.writer = httptest.NewRecorder()
	c.maxBodySize = 64
	var v struct {
		Name string `json:"name"`
	}
	err := c.BindJSON(&v)
	be, ok := err.(*BastError)
	if !ok || be.Status != 413 {
		t.Fatalf("want 413, got %v", err)
	}
	t.Logf("OK: %d %s", be.Status, be.Code)
}

// Body under the limit still works.
func TestFix_UnderLimitStillWorks(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"bob"}`))
	c := newTestCtx()
	c.Request = req
	c.writer = httptest.NewRecorder()
	c.maxBodySize = 1024
	var v struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&v); err != nil || v.Name != "bob" {
		t.Fatalf("under-limit failed: err=%v v=%+v", err, v)
	}
	t.Logf("OK: %+v", v)
}