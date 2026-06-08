package bast_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bastion-framework/bast"
)

// benchApp builds a minimal app for request lifecycle benchmarks.
func benchApp() *bast.App {
	app := bast.New(bast.Config{})

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
	app := bast.New(bast.Config{})
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
	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/err",
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