package bast_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bastion-framework/bast"
)

// silentLogger discards everything — benchmarks must measure the framework,
// not console output.
type silentLogger struct{}

func (silentLogger) OnBoot(string)                                        {}
func (silentLogger) OnModuleRegistered(_, _ string)                       {}
func (silentLogger) OnRouteRegistered(_, _ string, _ []string)            {}
func (silentLogger) OnServiceExported(_, _ string)                        {}
func (silentLogger) OnServiceResolved(_, _ string)                        {}
func (silentLogger) OnListening(int)                                      {}
func (silentLogger) OnShutdown()                                          {}
func (silentLogger) OnRequest(_, _ string, _ int, _ time.Duration, _ string) {}
func (silentLogger) OnError(_ *bast.Ctx, _ error)                         {}
func (silentLogger) Info(string, ...any)                                  {}
func (silentLogger) Warn(string, ...any)                                  {}
func (silentLogger) Error(string, ...any)                                 {}
func (silentLogger) Debug(string, ...any)                                 {}

// benchApp builds a minimal app for request lifecycle benchmarks.
func benchApp() *bast.App {
	app := bast.New(bast.Config{Logger: silentLogger{}})

	staticCtrl := &benchStaticController{}
	paramCtrl := &benchParamController{}

	app.Register(
		bast.Module{Prefix: "/static", Controller: staticCtrl},
		bast.Module{Prefix: "/param", Controller: paramCtrl},
	)
	return app
}

type benchStaticController struct{}

func (c *benchStaticController) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/ping", func(ctx *bast.Ctx) bast.Response {
			return ctx.OK(nil)
		}),
	}
}

type benchParamController struct{}

func (c *benchParamController) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/:id", func(ctx *bast.Ctx) bast.Response {
			_ = ctx.Param("id")
			return ctx.OK(nil)
		}),
	}
}

func BenchmarkApp_StaticRoute(b *testing.B) {
	app := benchApp()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/ping", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		w.Body.Reset()
		app.ServeHTTP(w, r)
	}
}

func BenchmarkApp_ParamRoute(b *testing.B) {
	app := benchApp()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/param/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		w.Body.Reset()
		app.ServeHTTP(w, r)
	}
}

func BenchmarkApp_MiddlewareChain(b *testing.B) {
	noop := func(next bast.HandlerFunc) bast.HandlerFunc {
		return func(ctx *bast.Ctx) bast.Response { return next(ctx) }
	}
	app := bast.New(bast.Config{Logger: silentLogger{}})
	app.Use(noop, noop, noop, noop, noop)
	app.Register(bast.Module{
		Prefix:     "/mw",
		Controller: &benchStaticController{},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/mw/ping", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		w.Body.Reset()
		app.ServeHTTP(w, r)
	}
}

func BenchmarkApp_ErrorBoundary(b *testing.B) {
	app := bast.New(bast.Config{Logger: silentLogger{}})
	app.Register(bast.Module{
		Prefix:     "/err",
		Controller: &benchErrorController{},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/err/boom", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		w.Body.Reset()
		app.ServeHTTP(w, r)
	}
}

type benchErrorController struct{}

func (c *benchErrorController) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/boom", func(ctx *bast.Ctx) bast.Response {
			return ctx.Error(bast.ErrNotFound(bast.CodeNotFound, "not found"))
		}),
	}
}