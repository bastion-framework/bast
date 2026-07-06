package bast

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func ipApp(trusted ...string) *App {
	return newTestApp(Config{TrustedProxies: trusted},
		GET("/ip", func(ctx *Ctx) Response {
			return ctx.Raw(200, "text/plain", []byte(ctx.IP()))
		}),
	)
}

func serveIP(t *testing.T, app *App, remoteAddr string, headers map[string]string) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/ip", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w.Body.String()
}

func TestIP_XFF_SpoofedLeftmostIgnored(t *testing.T) {
	app := ipApp("10.0.0.0/8")
	// Client forged "6.6.6.6"; the trusted proxy appended the real client on the right.
	got := serveIP(t, app, "10.0.0.9:4433", map[string]string{
		"X-Forwarded-For": "6.6.6.6, 203.0.113.5",
	})
	if got != "203.0.113.5" {
		t.Errorf("IP() = %q, want rightmost untrusted 203.0.113.5 (leftmost is attacker-controlled)", got)
	}
}

func TestIP_XFF_SkipsTrustedHops(t *testing.T) {
	app := ipApp("10.0.0.0/8")
	// Two internal hops appended after the client — both must be skipped.
	got := serveIP(t, app, "10.0.0.9:4433", map[string]string{
		"X-Forwarded-For": "203.0.113.5, 10.0.0.7, 10.0.0.8",
	})
	if got != "203.0.113.5" {
		t.Errorf("IP() = %q, want 203.0.113.5 after skipping trusted hops", got)
	}
}

func TestIP_XFF_AllTrusted_ReturnsLeftmost(t *testing.T) {
	app := ipApp("10.0.0.0/8")
	got := serveIP(t, app, "10.0.0.9:4433", map[string]string{
		"X-Forwarded-For": "10.0.0.5, 10.0.0.6",
	})
	if got != "10.0.0.5" {
		t.Errorf("IP() = %q, want 10.0.0.5 when every hop is trusted", got)
	}
}

func TestIP_UntrustedPeer_IgnoresXFF(t *testing.T) {
	app := ipApp("10.0.0.0/8")
	got := serveIP(t, app, "203.0.113.9:1234", map[string]string{
		"X-Forwarded-For": "6.6.6.6",
	})
	if got != "203.0.113.9" {
		t.Errorf("IP() = %q, want the peer address when it is not a trusted proxy", got)
	}
}

func TestReadiness_DoesNotLeakErrorDetail(t *testing.T) {
	secret := "postgres://user:hunter2@10.1.2.3:5432 connection refused"
	app := New(Config{
		Logger: &recLogger{},
		Health: &HealthConfig{
			ReadyPath: "/ready",
			Checks: []HealthCheck{
				CustomCheck("db", func(ctx context.Context) error { return errors.New(secret) }),
			},
		},
	})

	resp := app.ReadinessHandler()(NewTestCtxWithPath("/ready"))
	body := string(resp.Body())
	if resp.Status() != 503 {
		t.Errorf("status = %d, want 503", resp.Status())
	}
	if !strings.Contains(body, "degraded") {
		t.Errorf("body should mark the check degraded, got %s", body)
	}
	for _, leak := range []string{"postgres", "hunter2", "10.1.2.3"} {
		if strings.Contains(body, leak) {
			t.Errorf("readiness body leaks %q — dependency errors must be logged, not returned: %s", leak, body)
		}
	}
}

func TestStreamCtx_Send_MultilineDataCannotInjectFrames(t *testing.T) {
	app := newTestApp(Config{},
		STREAM("/sse", func(s *StreamCtx) {
			_ = s.Send("tick", "hello\nevent: forged\ndata: injected")
			_ = s.Flush()
		}),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/sse", nil))
	body := w.Body.String()

	if strings.Contains(body, "\nevent: forged\n") {
		t.Errorf("payload newline forged a new event frame:\n%s", body)
	}
	// Every payload line must be carried as its own data: line.
	want := "event: tick\ndata: hello\ndata: event: forged\ndata: data: injected\n\n"
	if body != want {
		t.Errorf("Send framing:\ngot  %q\nwant %q", body, want)
	}
}

func TestStreamCtx_Send_EventNameStripped(t *testing.T) {
	app := newTestApp(Config{},
		STREAM("/sse", func(s *StreamCtx) {
			_ = s.Send("tick\ndata: forged", "x")
			_ = s.Flush()
		}),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/sse", nil))
	if got, want := w.Body.String(), "event: tickdata: forged\ndata: x\n\n"; got != want {
		t.Errorf("event name must have CR/LF stripped:\ngot  %q\nwant %q", got, want)
	}
}

func TestWithValue_DoesNotAliasStore(t *testing.T) {
	c := newTestCtx()
	c.Set("shared", "orig")
	cp := c.WithValue("k", "v")

	cp.Set("copy-only", true)
	if _, ok := c.Get("copy-only"); ok {
		t.Error("Set on the WithValue copy leaked into the original's pooled store")
	}
	if v, _ := cp.Get("shared"); v != "orig" {
		t.Errorf("copy should inherit existing store values, got %v", v)
	}
	c.Set("orig-only", true)
	if _, ok := cp.Get("orig-only"); ok {
		t.Error("Set on the original leaked into the copy")
	}
}

func TestWithValue_ParamsPointIntoOwnStorage(t *testing.T) {
	c := newTestCtx()
	SetTestParam(c, "id", "42")
	cp := c.WithValue("k", "v")

	if got := cp.Param("id"); got != "42" {
		t.Fatalf("copy lost params: Param(id) = %q", got)
	}
	// Mutating the original's storage must not affect the copy.
	c.paramStorage[0].Value = "mutated"
	if got := cp.Param("id"); got != "42" {
		t.Errorf("copy params alias the original's paramStorage: got %q after mutation", got)
	}
}

func TestDocs_CustomAssetsBaseURL(t *testing.T) {
	app := New(Config{
		Logger: &recLogger{},
		Docs: &DocsConfig{
			Enabled:       true,
			Path:          "/docs",
			JSONPath:      "/openapi.json",
			Title:         "T",
			AssetsBaseURL: "/static/swagger",
		},
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/docs", nil))
	body := w.Body.String()
	if !strings.Contains(body, `"/static/swagger/swagger-ui.css"`) {
		t.Errorf("custom AssetsBaseURL not used for css:\n%s", body)
	}
	if strings.Contains(body, "unpkg.com") {
		t.Errorf("custom AssetsBaseURL still references unpkg:\n%s", body)
	}
}

func TestAutoOptions_PathExists_Returns204WithAllow(t *testing.T) {
	app := newTestApp(Config{},
		GET("/thing", func(ctx *Ctx) Response { return ctx.OK(nil) }),
		POST("/thing", func(ctx *Ctx) Response { return ctx.OK(nil) }),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/thing", nil))
	if w.Code != 204 {
		t.Fatalf("status = %d, want 204 for auto-OPTIONS", w.Code)
	}
	allow := w.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") {
		t.Errorf("Allow = %q, want GET and POST", allow)
	}
}

func TestAutoOptions_RunsGlobalMiddleware(t *testing.T) {
	// CORS is registered as global middleware; it must see auto-OPTIONS requests
	// or browser preflight breaks for every route without an explicit OPTIONS handler.
	mw := func(next HandlerFunc) HandlerFunc {
		return func(ctx *Ctx) Response {
			return next(ctx).WithHeader("X-Global-MW", "ran")
		}
	}
	app := New(Config{Logger: &recLogger{}})
	app.Use(mw)
	app.Register(Module{Controller: &routesCtrl{[]Route{
		GET("/thing", func(ctx *Ctx) Response { return ctx.OK(nil) }),
	}}})

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/thing", nil))
	if w.Header().Get("X-Global-MW") != "ran" {
		t.Error("global middleware did not run on the auto-OPTIONS path")
	}
}

func TestAutoOptions_UnknownPathStays404(t *testing.T) {
	app := newTestApp(Config{},
		GET("/thing", func(ctx *Ctx) Response { return ctx.OK(nil) }),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/nope", nil))
	if w.Code != 404 {
		t.Errorf("status = %d, want 404 for OPTIONS on unregistered path", w.Code)
	}
}

func TestAutoOptions_ExplicitRouteWins(t *testing.T) {
	app := newTestApp(Config{},
		GET("/thing", func(ctx *Ctx) Response { return ctx.OK(nil) }),
		Route{Method: "OPTIONS", Pattern: "/thing", Handler: func(ctx *Ctx) Response {
			return ctx.Raw(200, "text/plain", []byte("custom"))
		}},
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/thing", nil))
	if w.Code != 200 || w.Body.String() != "custom" {
		t.Errorf("explicit OPTIONS route must take precedence, got %d %q", w.Code, w.Body.String())
	}
}

func TestDocs_DefaultAssetsBaseURL(t *testing.T) {
	app := New(Config{
		Logger: &recLogger{},
		Docs:   &DocsConfig{Enabled: true, Path: "/docs", JSONPath: "/openapi.json"},
	})
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/docs", nil))
	if !strings.Contains(w.Body.String(), "unpkg.com/swagger-ui-dist") {
		t.Errorf("default should remain the unpkg CDN:\n%s", w.Body.String())
	}
}
