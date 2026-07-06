package bast

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedirect_RelativeTarget(t *testing.T) {
	app := newTestApp(Config{},
		GET("/go", func(c *Ctx) Response { return c.Redirect("/login", 302) }),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/go", nil))
	if w.Code != 302 || w.Header().Get("Location") != "/login" {
		t.Fatalf("got status=%d location=%q, want 302 /login", w.Code, w.Header().Get("Location"))
	}
}

func TestRedirect_CRLFStripped(t *testing.T) {
	w := httptest.NewRecorder()
	resp := newRawResponse(302, "", nil)
	resp.redirect = "/next\r\nSet-Cookie: evil=1"
	writeResponse(w, resp)
	if loc := w.Header().Get("Location"); strings.ContainsAny(loc, "\r\n") {
		t.Fatalf("CRLF not stripped from Location: %q", loc)
	}
}

func TestBindForm_PopulatesStruct(t *testing.T) {
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
		t.Fatalf("BindForm: %v", err)
	}
	if target.Name != "alice" || target.Age != 30 || !target.Admin || len(target.Tags) != 2 {
		t.Fatalf("not populated: %+v", target)
	}
}

func TestBindForm_BadIntIs400(t *testing.T) {
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
}

func TestFiles_MultipartRespectsMaxBodySize(t *testing.T) {
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
	if _, err := c.Files("f"); err == nil {
		t.Fatal("2MB upload accepted under 1KB limit")
	}
}

func TestBindJSON_OversizeBodyIs413(t *testing.T) {
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
}

func TestBindJSON_UnderLimitWorks(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"bob"}`))
	c := newTestCtx()
	c.Request = req
	c.writer = httptest.NewRecorder()
	c.maxBodySize = 1024
	var v struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&v); err != nil || v.Name != "bob" {
		t.Fatalf("under-limit body failed: err=%v v=%+v", err, v)
	}
}
