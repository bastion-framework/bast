package fuzz_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bastion-framework/bast"
	"github.com/bastion-framework/bast/router"
)

// ─── Router fuzz ──────────────────────────────────────────────────────────────

// FuzzRouter_Find ensures the router never panics on arbitrary path inputs.
func FuzzRouter_Find(f *testing.F) {
	// Seed corpus: valid paths the router knows about.
	f.Add("/users/42")
	f.Add("/users/me")
	f.Add("/")
	f.Add("/static/js/app.js")
	f.Add("//double-slash")
	f.Add("/users/:id")  // literal colon — not a param
	f.Add("/a/b/c/d/e/f/g/h/i/j/k")
	f.Add("")

	r := router.New()
	r.Add("GET", "/users/me", "h")
	r.Add("GET", "/users/:id", "h")
	r.Add("GET", "/static/*path", "h")
	r.Add("POST", "/users", "h")

	f.Fuzz(func(t *testing.T, path string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("router panicked on path %q: %v", path, r)
			}
		}()
		var buf [8]router.Param
		r.Find("GET", path, buf[:0]) //nolint
	})
}

// ─── Body parsing fuzz ────────────────────────────────────────────────────────

// FuzzCtx_BindJSON ensures body parsing never panics on arbitrary input.
func FuzzCtx_BindJSON(f *testing.F) {
	f.Add(`{"name":"kasim"}`)
	f.Add(`{}`)
	f.Add(`null`)
	f.Add(`[1,2,3]`)
	f.Add(`not json at all`)
	f.Add(`{"name":` + strings.Repeat("x", 1024) + `}`)
	f.Add("")

	f.Fuzz(func(t *testing.T, body string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("BindJSON panicked on body %q: %v", body, r)
			}
		}()
		ctx := bast.NewTestCtx()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		bast.InitTestCtx(ctx, httptest.NewRecorder(), r)
		var v map[string]any
		ctx.BindJSON(&v) //nolint
	})
}

// ─── Config fuzz ──────────────────────────────────────────────────────────────

// FuzzLoadConfig_StringField ensures config loading never panics on arbitrary env values.
func FuzzLoadConfig_StringField(f *testing.F) {
	f.Add("8080")
	f.Add("")
	f.Add("not-a-number")
	f.Add(strings.Repeat("x", 4096))
	f.Add("true")
	f.Add("30s")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, val string) {
		// Skip values that are invalid as env var values (null bytes).
		if strings.ContainsRune(val, 0) {
			t.Skip("null byte in env value — OS rejects it, not a framework concern")
		}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("LoadConfig panicked on value %q: %v", val, r)
			}
		}()
		t.Setenv("FUZZ_PORT", val)
		type Cfg struct {
			Port int `env:"FUZZ_PORT" default:"8080"`
		}
		bast.LoadConfig[Cfg]() //nolint
	})
}