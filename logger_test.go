package bast_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bastion-framework/bast"
)

// captureLogger records calls without any color codes for easy assertion.
type captureLogger struct {
	buf bytes.Buffer
}

func (l *captureLogger) OnBoot(v string)                                           { l.buf.WriteString("[boot] v" + v + "\n") }
func (l *captureLogger) OnModuleRegistered(name, prefix string)                    { l.buf.WriteString("[module] " + name + " " + prefix + "\n") }
func (l *captureLogger) OnRouteRegistered(method, path string, guards []string)    { l.buf.WriteString("[route] " + method + " " + path + "\n") }
func (l *captureLogger) OnServiceExported(svc, from string)                        {}
func (l *captureLogger) OnServiceResolved(svc, to string)                          {}
func (l *captureLogger) OnListening(port int)                                      { l.buf.WriteString("[listen]\n") }
func (l *captureLogger) OnShutdown()                                               { l.buf.WriteString("[shutdown]\n") }
func (l *captureLogger) OnRequest(m, p string, s int, d time.Duration, ip string) { l.buf.WriteString("[req] " + m + " " + p + "\n") }
func (l *captureLogger) OnError(ctx *bast.Ctx, err error)                          { l.buf.WriteString("[err] " + err.Error() + "\n") }
func (l *captureLogger) Info(msg string, args ...any)                              {}
func (l *captureLogger) Warn(msg string, args ...any)                              {}
func (l *captureLogger) Error(msg string, args ...any)                             {}
func (l *captureLogger) Debug(msg string, args ...any)                             {}

func serveTest(app *bast.App, method, path string) int {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, nil)
	app.ServeHTTP(w, r)
	return w.Code
}

func TestLogger_OnBoot_CalledOnNew(t *testing.T) {
	cl := &captureLogger{}
	bast.New(bast.Config{Logger: cl})
	if !strings.Contains(cl.buf.String(), "[boot]") {
		t.Errorf("expected OnBoot, got: %s", cl.buf.String())
	}
}

func TestLogger_OnModuleRegistered(t *testing.T) {
	cl := &captureLogger{}
	app := bast.New(bast.Config{Logger: cl})
	app.Register(bast.Module{
		Prefix:     "/users",
		Doc:        bast.ModuleDoc{Name: "Users"},
		Controller: &logTestCtrl{},
	})
	if !strings.Contains(cl.buf.String(), "[module] Users /users") {
		t.Errorf("expected OnModuleRegistered, got: %s", cl.buf.String())
	}
}

func TestLogger_OnRouteRegistered(t *testing.T) {
	cl := &captureLogger{}
	app := bast.New(bast.Config{Logger: cl})
	app.Register(bast.Module{Prefix: "/items", Controller: &logTestCtrl{}})
	out := cl.buf.String()
	if !strings.Contains(out, "[route] GET /items/") {
		t.Errorf("expected GET route, got: %s", out)
	}
	if !strings.Contains(out, "[route] POST /items/") {
		t.Errorf("expected POST route, got: %s", out)
	}
}

func TestLogger_OnRequest(t *testing.T) {
	cl := &captureLogger{}
	app := bast.New(bast.Config{Logger: cl})
	app.Register(bast.Module{Prefix: "/ping", Controller: &logTestCtrl{}})
	cl.buf.Reset()

	serveTest(app, http.MethodGet, "/ping/")

	if !strings.Contains(cl.buf.String(), "[req] GET /ping/") {
		t.Errorf("expected OnRequest, got: %s", cl.buf.String())
	}
}

func TestLogger_OnError(t *testing.T) {
	cl := &captureLogger{}
	app := bast.New(bast.Config{Logger: cl})
	app.Register(bast.Module{Prefix: "/boom", Controller: &logErrCtrl{}})
	cl.buf.Reset()

	serveTest(app, http.MethodGet, "/boom/")

	if !strings.Contains(cl.buf.String(), "[err]") {
		t.Errorf("expected OnError, got: %s", cl.buf.String())
	}
}

func TestLogger_DefaultLogger_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	l := bast.NewDefaultLogger()
	if l == nil {
		t.Error("expected non-nil logger")
	}
}

func TestLogger_OnRequest_404(t *testing.T) {
	cl := &captureLogger{}
	app := bast.New(bast.Config{Logger: cl})
	cl.buf.Reset()

	serveTest(app, http.MethodGet, "/missing")

	if !strings.Contains(cl.buf.String(), "[req] GET /missing") {
		t.Errorf("expected 404 request log, got: %s", cl.buf.String())
	}
}

// ── controllers used by logger tests ─────────────────────────────────────────

type logTestCtrl struct{}

func (c *logTestCtrl) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/", func(ctx *bast.Ctx) bast.Response { return ctx.OK(nil) }),
		bast.POST("/", func(ctx *bast.Ctx) bast.Response { return ctx.Created(nil) }),
	}
}

type logErrCtrl struct{}

func (c *logErrCtrl) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/", func(ctx *bast.Ctx) bast.Response {
			return ctx.Error(bast.ErrNotFound(bast.CodeNotFound, "not found"))
		}),
	}
}