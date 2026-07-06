package bast

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recLogger records request statuses for assertion.
type recLogger struct {
	mu       sync.Mutex
	statuses []int
}

func (l *recLogger) OnBoot(string)                          {}
func (l *recLogger) OnModuleRegistered(_, _ string)         {}
func (l *recLogger) OnRouteRegistered(_, _ string, _ []string) {}
func (l *recLogger) OnServiceExported(_, _ string)          {}
func (l *recLogger) OnServiceResolved(_, _ string)          {}
func (l *recLogger) OnListening(int)                        {}
func (l *recLogger) OnShutdown()                            {}
func (l *recLogger) OnRequest(_, _ string, s int, _ time.Duration, _ string) {
	l.mu.Lock()
	l.statuses = append(l.statuses, s)
	l.mu.Unlock()
}
func (l *recLogger) OnError(_ *Ctx, _ error) {}
func (l *recLogger) Info(string, ...any)  {}
func (l *recLogger) Warn(string, ...any)  {}
func (l *recLogger) Error(string, ...any) {}
func (l *recLogger) Debug(string, ...any) {}

func (l *recLogger) lastStatus() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.statuses) == 0 {
		return -1
	}
	return l.statuses[len(l.statuses)-1]
}

type routesCtrl struct{ routes []Route }

func (c *routesCtrl) Routes() []Route { return c.routes }

func newTestApp(cfg Config, routes ...Route) *App {
	if cfg.Logger == nil {
		cfg.Logger = &recLogger{}
	}
	app := New(cfg)
	app.Register(Module{Controller: &routesCtrl{routes}})
	return app
}



func TestBuildServer_SafeDefaults(t *testing.T) {
	app := New(Config{Logger: &recLogger{}})
	srv := app.buildServer()
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 10s default", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s default", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 1<<20 {
		t.Errorf("MaxHeaderBytes = %d, want 1MB", srv.MaxHeaderBytes)
	}
	// ReadTimeout/WriteTimeout must NOT be defaulted — a default WriteTimeout
	// would kill every SSE stream.
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Errorf("Read/WriteTimeout = %v/%v, want 0 (unset)", srv.ReadTimeout, srv.WriteTimeout)
	}
}

func TestBuildServer_ExplicitValuesRespected(t *testing.T) {
	app := New(Config{
		Logger:            &recLogger{},
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      6 * time.Second,
		IdleTimeout:       7 * time.Second,
		ReadHeaderTimeout: 8 * time.Second,
	})
	srv := app.buildServer()
	if srv.ReadTimeout != 5*time.Second || srv.WriteTimeout != 6*time.Second ||
		srv.IdleTimeout != 7*time.Second || srv.ReadHeaderTimeout != 8*time.Second {
		t.Errorf("explicit timeouts not respected: %+v", srv)
	}
}

func TestBuildServer_NegativeDisablesDefault(t *testing.T) {
	app := New(Config{
		Logger:            &recLogger{},
		ReadHeaderTimeout: -1,
		IdleTimeout:       -1,
	})
	srv := app.buildServer()
	if srv.ReadHeaderTimeout != 0 {
		t.Errorf("ReadHeaderTimeout = %v, want 0 (disabled via -1)", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 0 {
		t.Errorf("IdleTimeout = %v, want 0 (disabled via -1)", srv.IdleTimeout)
	}
}


func TestHandlerTimeout_CancelsContext(t *testing.T) {
	app := newTestApp(Config{HandlerTimeout: 20 * time.Millisecond},
		GET("/slow", func(ctx *Ctx) Response {
			select {
			case <-ctx.Context().Done():
				return ctx.Error(ErrTimeout("handler deadline exceeded"))
			case <-time.After(2 * time.Second):
				return ctx.OK(nil)
			}
		}),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/slow", nil))
	if w.Code != 408 {
		t.Errorf("status = %d, want 408 from HandlerTimeout", w.Code)
	}
}

func TestRouteTimeout_OverridesHandlerTimeout(t *testing.T) {
	app := newTestApp(Config{HandlerTimeout: 20 * time.Millisecond},
		GET("/patient", func(ctx *Ctx) Response {
			select {
			case <-ctx.Context().Done():
				return ctx.Error(ErrTimeout("deadline hit"))
			case <-time.After(60 * time.Millisecond):
				return ctx.OK(nil)
			}
		}, WithTimeout(500*time.Millisecond)),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/patient", nil))
	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (route timeout should override global)", w.Code)
	}
}

func TestShutdownContext_AddsDeadlineWhenMissing(t *testing.T) {
	ctx, cancel := shutdownContext(context.Background(), 30*time.Second)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("shutdownContext should add a deadline when the caller's context has none")
	}
}

func TestShutdownContext_KeepsCallerDeadline(t *testing.T) {
	parent, pcancel := context.WithTimeout(context.Background(), time.Hour)
	defer pcancel()
	want, _ := parent.Deadline()
	ctx, cancel := shutdownContext(parent, time.Second)
	defer cancel()
	got, ok := ctx.Deadline()
	if !ok || !got.Equal(want) {
		t.Fatalf("shutdownContext must not shorten an existing deadline: got %v want %v", got, want)
	}
}



func TestServeHTTP_HandlerPanic_Returns500(t *testing.T) {
	log := &recLogger{}
	app := newTestApp(Config{Logger: log},
		GET("/boom", func(ctx *Ctx) Response { panic("kaboom") }),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/boom", nil)) // must not crash the test
	if w.Code != 500 {
		t.Errorf("status = %d, want 500 after recovered panic", w.Code)
	}
	if !strings.Contains(w.Body.String(), "INTERNAL_ERROR") {
		t.Errorf("body should be the standard 500 envelope, got: %s", w.Body.String())
	}
}

func TestServeHTTP_AbortHandler_Repanics(t *testing.T) {
	app := newTestApp(Config{},
		GET("/abort", func(ctx *Ctx) Response { panic(http.ErrAbortHandler) }),
	)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("http.ErrAbortHandler must propagate to net/http, not be swallowed")
		}
		if err, ok := r.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("re-panicked value = %v, want http.ErrAbortHandler", r)
		}
	}()
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/abort", nil))
}

func TestServeHTTP_StreamPanic_Recovered(t *testing.T) {
	log := &recLogger{}
	app := newTestApp(Config{Logger: log},
		STREAM("/events", func(s *StreamCtx) { panic("stream kaboom") }),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/events", nil)) // must not crash
	if w.Code != 500 {
		t.Errorf("status = %d, want 500 when stream panics before writing", w.Code)
	}
	if log.lastStatus() != 500 {
		t.Errorf("logged status = %d, want 500", log.lastStatus())
	}
}



func TestNew_InvalidTrustedProxy_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New must panic on an invalid TrustedProxies CIDR — silent trust changes are a security bug")
		}
	}()
	New(Config{Logger: &recLogger{}, TrustedProxies: []string{"10.0.0.0/8", "not-a-cidr"}})
}

func TestNew_ValidTrustedProxies_OK(t *testing.T) {
	app := New(Config{Logger: &recLogger{}, TrustedProxies: []string{"10.0.0.0/8", "::1/128"}})
	if len(app.proxies) != 2 {
		t.Errorf("proxies = %d, want 2", len(app.proxies))
	}
}



func TestStream_StatusLogged_Default200(t *testing.T) {
	log := &recLogger{}
	app := newTestApp(Config{Logger: log},
		STREAM("/ok", func(s *StreamCtx) {
			_ = s.Send("tick", "1")
			_ = s.Flush()
		}),
	)
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ok", nil))
	if log.lastStatus() != 200 {
		t.Errorf("logged status = %d, want 200", log.lastStatus())
	}
}

func TestStream_StatusLogged_Custom(t *testing.T) {
	log := &recLogger{}
	app := newTestApp(Config{Logger: log},
		STREAM("/gone", func(s *StreamCtx) {
			s.Status(http.StatusResetContent)
		}),
	)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/gone", nil))
	if w.Code != http.StatusResetContent {
		t.Errorf("wire status = %d, want 205", w.Code)
	}
	if log.lastStatus() != http.StatusResetContent {
		t.Errorf("logged status = %d, want 205", log.lastStatus())
	}
}



func TestCtx_LazyStore_GetOnNil(t *testing.T) {
	c := &Ctx{}
	if _, ok := c.Get("missing"); ok {
		t.Error("Get on nil store must return false, not panic")
	}
	c.Set("k", "v")
	if v, _ := c.Get("k"); v != "v" {
		t.Errorf("Set/Get after lazy init: got %v", v)
	}
}

func TestStreamCtx_LazyStore(t *testing.T) {
	s := &StreamCtx{Context: context.Background()}
	if _, ok := s.Get("missing"); ok {
		t.Error("Get on nil store must return false")
	}
	s.Set("k", "v")
	if v, _ := s.Get("k"); v != "v" {
		t.Errorf("Set/Get after lazy init: got %v", v)
	}
}


func TestReadBody_PooledBuffer_NoCrossContamination(t *testing.T) {
	for i, body := range []string{"first-request-body", "x", "third one, longer than the second"} {
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		c := acquireCtx(httptest.NewRecorder(), req, defaultMaxBodySize, nil, nil)
		got, err := c.RawBody()
		if err != nil {
			t.Fatalf("req %d: RawBody: %v", i, err)
		}
		if string(got) != body {
			t.Errorf("req %d: body = %q, want %q", i, got, body)
		}
		releaseCtx(c)
	}
}