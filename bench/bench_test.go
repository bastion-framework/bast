// Package bench_test compares bast routing+dispatch performance against
// the most widely-used Go HTTP frameworks.
//
// All net/http-based frameworks (stdlib, httprouter, gin, echo, chi,
// gorilla/mux, iris) are measured identically: httptest.ResponseRecorder +
// ServeHTTP. Results are directly comparable.
//
// Fiber uses fasthttp (different HTTP stack). It is benchmarked via app.Test()
// which serialises/parses a full HTTP/1.1 message through an in-process pipe —
// that overhead is real in testing but absent in production. Treat fiber numbers
// as directional, not a direct ns/op comparison.
//
// Run:
//
//	cd bench && go test -bench=. -benchmem -benchtime=5s
package bench_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bastion-framework/bast"
	fiberv2 "github.com/gofiber/fiber/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	gmux "github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	iris "github.com/kataras/iris/v12"
	"github.com/labstack/echo/v4"
)

// ── route corpus ──────────────────────────────────────────────────────────────

type route struct {
	method, path string
}

// githubRoutes is the canonical 26-route GitHub REST API corpus used across
// the Go HTTP benchmark community (go-http-routing-benchmark et al.).
var githubRoutes = []route{
	{"GET", "/authorizations"},
	{"GET", "/authorizations/:id"},
	{"POST", "/authorizations"},
	{"PUT", "/authorizations/clients/:client_id"},
	{"PATCH", "/authorizations/:id"},
	{"DELETE", "/authorizations/:id"},
	{"GET", "/applications/:client_id/tokens/:access_token"},
	{"DELETE", "/applications/:client_id/tokens"},
	{"DELETE", "/applications/:client_id/tokens/:access_token"},
	{"GET", "/events"},
	{"GET", "/repos/:owner/:repo/events"},
	{"GET", "/networks/:owner/:repo/events"},
	{"GET", "/orgs/:org/events"},
	{"GET", "/users/:user/received_events"},
	{"GET", "/users/:user/received_events/public"},
	{"GET", "/users/:user/events"},
	{"GET", "/users/:user/events/public"},
	{"GET", "/users/:user/events/orgs/:org"},
	{"GET", "/feeds"},
	{"GET", "/notifications"},
	{"GET", "/repos/:owner/:repo/notifications"},
	{"PUT", "/notifications"},
	{"PUT", "/repos/:owner/:repo/notifications"},
	{"GET", "/notifications/threads/:id"},
	{"PATCH", "/notifications/threads/:id"},
	{"GET", "/notifications/threads/:id/subscription"},
}

// githubReqs cycles through 8 paths that cover static, 1-param, 2-param, and
// cross-method cases from the corpus.
var githubReqs = [8]route{
	{"GET", "/authorizations"},
	{"GET", "/authorizations/1"},
	{"POST", "/authorizations"},
	{"GET", "/repos/golang/go/events"},
	{"GET", "/users/kasim/received_events"},
	{"GET", "/notifications/threads/42/subscription"},
	{"DELETE", "/applications/abc123/tokens/tok456"},
	{"PATCH", "/notifications/threads/99"},
}

var (
	static1 = []route{{"GET", "/ping"}}
	param1  = []route{{"GET", "/users/:id"}}
)

// curlyPath converts :param segments to {param} for chi, gorilla/mux, and stdlib.
func curlyPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if len(s) > 0 && s[0] == ':' {
			parts[i] = "{" + s[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// ── framework builders ────────────────────────────────────────────────────────

func newBast(routes []route) http.Handler {
	app := bast.New(bast.Config{Logger: noopLog{}})
	app.Register(bast.Module{Controller: &corpusCtrl{routes}})
	return app
}

func newStdlib(routes []route) http.Handler {
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.HandleFunc(r.method+" "+curlyPath(r.path), func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}
	return mux
}

func newHttprouter(routes []route) http.Handler {
	r := httprouter.New()
	for _, rt := range routes {
		r.Handle(rt.method, rt.path, func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
			w.WriteHeader(http.StatusOK)
		})
	}
	return r
}

func newGin(routes []route) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	for _, rt := range routes {
		r.Handle(rt.method, rt.path, func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
	}
	return r
}

func newEcho(routes []route) http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	for _, rt := range routes {
		e.Add(rt.method, rt.path, func(c echo.Context) error {
			return c.NoContent(http.StatusOK)
		})
	}
	return e
}

func newChi(routes []route) http.Handler {
	r := chi.NewRouter()
	for _, rt := range routes {
		r.Method(rt.method, curlyPath(rt.path), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	}
	return r
}

func newGorilla(routes []route) http.Handler {
	r := gmux.NewRouter()
	for _, rt := range routes {
		r.HandleFunc(curlyPath(rt.path), func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).Methods(rt.method)
	}
	return r
}

func newIris(routes []route) http.Handler {
	app := iris.New()
	app.Logger().SetLevel("disable")
	for _, rt := range routes {
		app.Handle(rt.method, rt.path, func(ctx iris.Context) {
			ctx.StatusCode(http.StatusOK)
		})
	}
	if err := app.Build(); err != nil {
		panic(err)
	}
	return app
}

func newFiber(routes []route) *fiberv2.App {
	app := fiberv2.New(fiberv2.Config{DisableStartupMessage: true})
	for _, rt := range routes {
		app.Add(rt.method, rt.path, func(c *fiberv2.Ctx) error {
			return c.SendStatus(fiberv2.StatusOK)
		})
	}
	return app
}

// ── benchmark harness ─────────────────────────────────────────────────────────

// runHTTP measures a net/http-based handler dispatching a single fixed request.
func runHTTP(b *testing.B, h http.Handler, req *http.Request) {
	b.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req) // warm up: bast freezes router on first call
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		h.ServeHTTP(w, req)
	}
}

// runHTTPCorpus cycles through the 8 representative GitHub requests.
func runHTTPCorpus(b *testing.B, h http.Handler) {
	b.Helper()
	ws := make([]*httptest.ResponseRecorder, len(githubReqs))
	reqs := make([]*http.Request, len(githubReqs))
	for i, gr := range githubReqs {
		ws[i] = httptest.NewRecorder()
		reqs[i] = httptest.NewRequest(gr.method, gr.path, nil)
	}
	h.ServeHTTP(ws[0], reqs[0]) // warm up
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := i % len(githubReqs)
		ws[n].Body.Reset()
		h.ServeHTTP(ws[n], reqs[n])
	}
}

// ── Static: GET /ping ─────────────────────────────────────────────────────────

func BenchmarkStatic(b *testing.B) {
	req := httptest.NewRequest("GET", "/ping", nil)
	b.Run("bast", func(b *testing.B) { runHTTP(b, newBast(static1), req) })
	b.Run("stdlib", func(b *testing.B) { runHTTP(b, newStdlib(static1), req) })
	b.Run("httprouter", func(b *testing.B) { runHTTP(b, newHttprouter(static1), req) })
	b.Run("gin", func(b *testing.B) { runHTTP(b, newGin(static1), req) })
	b.Run("echo", func(b *testing.B) { runHTTP(b, newEcho(static1), req) })
	b.Run("chi", func(b *testing.B) { runHTTP(b, newChi(static1), req) })
	b.Run("gorilla", func(b *testing.B) { runHTTP(b, newGorilla(static1), req) })
	b.Run("iris", func(b *testing.B) { runHTTP(b, newIris(static1), req) })
}

// ── Param: GET /users/:id ─────────────────────────────────────────────────────

func BenchmarkParam(b *testing.B) {
	req := httptest.NewRequest("GET", "/users/42", nil)
	b.Run("bast", func(b *testing.B) { runHTTP(b, newBast(param1), req) })
	b.Run("stdlib", func(b *testing.B) { runHTTP(b, newStdlib(param1), req) })
	b.Run("httprouter", func(b *testing.B) { runHTTP(b, newHttprouter(param1), req) })
	b.Run("gin", func(b *testing.B) { runHTTP(b, newGin(param1), req) })
	b.Run("echo", func(b *testing.B) { runHTTP(b, newEcho(param1), req) })
	b.Run("chi", func(b *testing.B) { runHTTP(b, newChi(param1), req) })
	b.Run("gorilla", func(b *testing.B) { runHTTP(b, newGorilla(param1), req) })
	b.Run("iris", func(b *testing.B) { runHTTP(b, newIris(param1), req) })
}

// ── GitHub API corpus: 26 routes, 8 representative requests ──────────────────

func BenchmarkGitHub(b *testing.B) {
	b.Run("bast", func(b *testing.B) { runHTTPCorpus(b, newBast(githubRoutes)) })
	b.Run("stdlib", func(b *testing.B) { runHTTPCorpus(b, newStdlib(githubRoutes)) })
	b.Run("httprouter", func(b *testing.B) { runHTTPCorpus(b, newHttprouter(githubRoutes)) })
	b.Run("gin", func(b *testing.B) { runHTTPCorpus(b, newGin(githubRoutes)) })
	b.Run("echo", func(b *testing.B) { runHTTPCorpus(b, newEcho(githubRoutes)) })
	b.Run("chi", func(b *testing.B) { runHTTPCorpus(b, newChi(githubRoutes)) })
	b.Run("gorilla", func(b *testing.B) { runHTTPCorpus(b, newGorilla(githubRoutes)) })
	b.Run("iris", func(b *testing.B) { runHTTPCorpus(b, newIris(githubRoutes)) })
}

// ── Fiber (fasthttp) — different HTTP stack, noted ───────────────────────────
// Benchmarked via app.Test() which pipes a full HTTP/1.1 message in-process.
// This overhead is absent in production fiber deployments.

func BenchmarkFiber_Static(b *testing.B) {
	app := newFiber(static1)
	req := httptest.NewRequest("GET", "/ping", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req, -1)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkFiber_Param(b *testing.B) {
	app := newFiber(param1)
	req := httptest.NewRequest("GET", "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req, -1)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkFiber_GitHub(b *testing.B) {
	app := newFiber(githubRoutes)
	reqs := make([]*http.Request, len(githubReqs))
	for i, gr := range githubReqs {
		reqs[i] = httptest.NewRequest(gr.method, gr.path, nil)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(reqs[i%len(reqs)], -1)
		if err == nil {
			resp.Body.Close()
		}
	}
}

// ── bast corpus controller ────────────────────────────────────────────────────

type corpusCtrl struct{ routes []route }

func (c *corpusCtrl) Routes() []bast.Route {
	noop := func(ctx *bast.Ctx) bast.Response { return ctx.OK(nil) }
	out := make([]bast.Route, len(c.routes))
	for i, r := range c.routes {
		out[i] = bast.Route{Method: r.method, Pattern: r.path, Handler: noop}
	}
	return out
}

// ── noop logger for bast ──────────────────────────────────────────────────────

type noopLog struct{}

func (noopLog) OnBoot(_ string)                                               {}
func (noopLog) OnModuleRegistered(_, _ string)                                {}
func (noopLog) OnRouteRegistered(_ string, _ string, _ []string)              {}
func (noopLog) OnServiceExported(_, _ string)                                 {}
func (noopLog) OnServiceResolved(_, _ string)                                 {}
func (noopLog) OnListening(_ int)                                              {}
func (noopLog) OnShutdown()                                                   {}
func (noopLog) OnRequest(_ string, _ string, _ int, _ time.Duration, _ string) {}
func (noopLog) OnError(_ *bast.Ctx, _ error)                                  {}
func (noopLog) Info(_ string, _ ...any)                                        {}
func (noopLog) Warn(_ string, _ ...any)                                        {}
func (noopLog) Error(_ string, _ ...any)                                       {}
func (noopLog) Debug(_ string, _ ...any)                                       {}