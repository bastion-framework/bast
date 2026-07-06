package bast

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/bastion-framework/bast/internal/router"
)

// Config holds app-level settings.
//
// Timeout semantics: 0 means "use the safe default" where one exists,
// a negative value disables the timeout entirely.
type Config struct {
	Port int

	// ReadTimeout / WriteTimeout have no default: a default WriteTimeout would
	// kill long-lived SSE streams and a default ReadTimeout would kill slow
	// uploads. Set them if your app serves neither. Use HandlerTimeout for
	// per-request protection instead.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// IdleTimeout defaults to 120s; ReadHeaderTimeout defaults to 10s
	// (slowloris protection). Set to -1 to disable.
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration

	MaxBodySize int64

	// HandlerTimeout is the global per-request deadline applied to every
	// non-stream route without an explicit route-level Timeout. Handlers see it
	// via ctx.Context() cancellation. 0 means no global deadline.
	HandlerTimeout time.Duration

	// ShutdownTimeout bounds Shutdown when the caller's context has no
	// deadline of its own. Defaults to 30s.
	ShutdownTimeout time.Duration

	HookTimeout    time.Duration
	TrustedProxies []string
	ErrorHandler   ErrorHandler
	Validator      Validator
	Health         *HealthConfig
	Docs           *DocsConfig
	Logger         Logger
}

// App is the Bast application. Satisfies http.Handler.
type App struct {
	cfg         Config
	router      *router.Router
	middleware  []MiddlewareFunc
	guards      []Guard
	errHandler  ErrorHandler
	modules     []Module
	moduleNames []string
	proxies     []*net.IPNet
	srv         *http.Server
	specBuilder *specBuilder
	logger      Logger
}

// New creates a new Bast app with the given config.
func New(cfg Config) *App {
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = defaultMaxBodySize
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.HookTimeout == 0 {
		cfg.HookTimeout = 10 * time.Second
	}
	errHandler := cfg.ErrorHandler
	if errHandler == nil {
		errHandler = DefaultErrorHandler
	}
	logger := cfg.Logger
	if logger == nil {
		logger = NewDefaultLogger()
	}

	proxies := parseCIDRs(cfg.TrustedProxies)

	app := &App{
		cfg:         cfg,
		router:      router.New(),
		errHandler:  errHandler,
		proxies:     proxies,
		specBuilder: newSpecBuilder(),
		logger:      logger,
	}
	app.logger.OnBoot(bastVersion)
	app.registerHealthRoutes()
	app.registerDocsRoutes()
	return app
}

// Use registers global middleware. Runs for every request.
func (a *App) Use(mw ...MiddlewareFunc) *App {
	a.middleware = append(a.middleware, mw...)
	return a
}

// Guard registers global guards. Runs for every request before the handler.
func (a *App) Guard(guards ...Guard) *App {
	a.guards = append(a.guards, guards...)
	return a
}

// Register mounts one or more modules onto the app.
func (a *App) Register(modules ...Module) *App {
	for _, m := range modules {
		a.registerModule(m, "")
	}
	return a
}

func (a *App) registerModule(m Module, parentPrefix string) {
	prefix := parentPrefix + m.Prefix
	a.modules = append(a.modules, m)
	a.moduleNames = append(a.moduleNames, m.Doc.Name)
	a.specBuilder.addModuleDoc(m.Doc)

	name := m.Doc.Name
	if name == "" {
		name = m.Prefix
	}
	a.logger.OnModuleRegistered(name, prefix)

	if m.Controller != nil {
		for _, route := range m.Controller.Routes() {
			a.registerRoute(route, prefix, m.Middleware, m.Guards, m.Doc)
		}
	}

	for _, sub := range m.Modules {
		a.registerModule(sub, prefix)
	}
}

func (a *App) registerRoute(r Route, prefix string, modMW []MiddlewareFunc, modGuards []Guard, modDoc ModuleDoc) {
	fullPattern := prefix + r.Pattern

	// Build the full middleware chain: global → module → route.
	allMW := make([]MiddlewareFunc, 0, len(a.middleware)+len(modMW)+len(r.Middleware))
	allMW = append(allMW, a.middleware...)
	allMW = append(allMW, modMW...)
	allMW = append(allMW, r.Middleware...)

	// Build guard list: global → module → route.
	allGuards := make([]Guard, 0, len(a.guards)+len(modGuards)+len(r.Guards))
	allGuards = append(allGuards, a.guards...)
	allGuards = append(allGuards, modGuards...)
	allGuards = append(allGuards, r.Guards...)

	a.specBuilder.addRoute(r.Method, fullPattern, r, modDoc, allGuards)
	a.logger.OnRouteRegistered(r.Method, fullPattern, guardNames(allGuards))

	if r.Stream != nil {
		sp := streamPipeline{guards: allGuards, handler: r.Stream}
		a.router.Add(r.Method, fullPattern, sp)
		return
	}

	handler := r.Handler
	if len(allGuards) > 0 {
		handler = guardMiddleware(allGuards, handler, a.errHandler)
	}

	pipeline := buildPipeline(handler, allMW)

	// Per-route body size limit — overrides the global setting for this route.
	if r.MaxBodySize > 0 {
		maxBody := r.MaxBodySize
		inner := pipeline
		pipeline = func(ctx *Ctx) Response {
			ctx.maxBodySize = maxBody
			return inner(ctx)
		}
	}

	// Per-route timeout — wraps the context with a deadline so services
	// that respect ctx.Context() are cancelled when time is up.
	// Falls back to the global HandlerTimeout when the route sets none.
	timeout := r.Timeout
	if timeout == 0 {
		timeout = a.cfg.HandlerTimeout
	}
	if timeout > 0 {
		inner := pipeline
		pipeline = func(ctx *Ctx) Response {
			tctx, cancel := context.WithTimeout(ctx.Context(), timeout)
			defer cancel()
			ctx.detachedCtx = tctx
			return inner(ctx)
		}
	}

	a.router.Add(r.Method, fullPattern, pipeline)
}

// streamPipeline pairs a StreamHandlerFunc with its guard chain.
// Stored in the router in place of a bare StreamHandlerFunc so that
// ServeHTTP can run guards synchronously before handing off to the handler.
type streamPipeline struct {
	guards  []Guard
	handler StreamHandlerFunc
}

// guardMiddleware wraps guards into a HandlerFunc that short-circuits on failure.
func guardMiddleware(guards []Guard, next HandlerFunc, errHandler ErrorHandler) HandlerFunc {
	return func(ctx *Ctx) Response {
		for _, g := range guards {
			if err := g.Check(ctx); err != nil {
				return errHandler(ctx, err)
			}
		}
		return next(ctx)
	}
}

// ServeHTTP satisfies http.Handler.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := acquireCtx(w, r, a.cfg.MaxBodySize, a.proxies, a.cfg.Validator)
	defer releaseCtx(ctx)

	match, err := a.router.Find(r.Method, r.URL.Path, ctx.params)
	if err != nil {
		var mna *router.MethodNotAllowedError
		if errors.As(err, &mna) {
			allow := strings.Join(mna.Allow, ", ")

			// Auto-OPTIONS: the path exists but no OPTIONS route is registered.
			// Run it through the global middleware chain — CORS preflight arrives
			// as OPTIONS and must reach the CORS middleware, which is global.
			if r.Method == http.MethodOptions {
				handler := func(*Ctx) Response {
					return newRawResponse(http.StatusNoContent, "", nil).WithHeader("Allow", allow)
				}
				resp := a.invoke(buildPipeline(handler, a.middleware), ctx)
				writeResponse(w, resp)
				a.logger.OnRequest(r.Method, r.URL.Path, resp.Status(), time.Since(start), ctx.IP())
				return
			}

			w.Header().Set("Allow", allow)
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
			a.logger.OnRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed, time.Since(start), "")
			return
		}
		http.Error(w, "404 not found", http.StatusNotFound)
		a.logger.OnRequest(r.Method, r.URL.Path, http.StatusNotFound, time.Since(start), "")
		return
	}
	ctx.params = match.Params

	var status int
	switch h := match.Handler.(type) {
	case streamPipeline:
		status = a.serveStream(w, r, ctx, h)
	case HandlerFunc:
		resp := a.invoke(h, ctx)
		if resp.IsError() {
			a.logger.OnError(ctx, resp.Err())
			resp = a.errHandler(ctx, resp.Err())
		}
		writeResponse(w, resp)
		status = resp.Status()
	default:
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		status = http.StatusInternalServerError
	}

	a.logger.OnRequest(r.Method, r.URL.Path, status, time.Since(start), ctx.IP())
}

// invoke runs a handler with built-in panic recovery. A panicking handler
// yields a standard 500 instead of a dead connection. http.ErrAbortHandler
// propagates untouched — it is net/http's sanctioned way to abort a response.
func (a *App) invoke(h HandlerFunc, ctx *Ctx) (resp Response) {
	defer func() {
		if rec := recover(); rec != nil {
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			a.logger.Error("panic recovered: %v method=%s path=%s\n%s",
				rec, ctx.Method(), ctx.Path(), debug.Stack())
			resp = a.errHandler(ctx, &BastError{
				Status: http.StatusInternalServerError,
				Code:   CodeInternal, Message: "internal server error",
			})
		}
	}()
	return h(ctx)
}

// serveStream runs guards then hands the connection to the stream handler.
// Panics are recovered: if nothing was written yet the client gets a 500,
// otherwise the connection just closes (headers are already on the wire).
func (a *App) serveStream(w http.ResponseWriter, r *http.Request, ctx *Ctx, sp streamPipeline) (status int) {
	for _, g := range sp.guards {
		if err := g.Check(ctx); err != nil {
			resp := a.errHandler(ctx, err)
			writeResponse(w, resp)
			return resp.Status()
		}
	}

	sctx := newStreamCtx(r.Context(), w, r)
	if len(ctx.params) > 0 {
		sctx.params = make([]router.Param, len(ctx.params))
		copy(sctx.params, ctx.params)
	}
	if len(ctx.store) > 0 {
		sctx.store = make(map[string]any, len(ctx.store))
		for k, v := range ctx.store {
			sctx.store[k] = v
		}
	}

	defer func() {
		if rec := recover(); rec != nil {
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			a.logger.Error("panic recovered in stream: %v method=%s path=%s\n%s",
				rec, r.Method, r.URL.Path, debug.Stack())
			if !sctx.headerSent {
				w.WriteHeader(http.StatusInternalServerError)
				sctx.status = http.StatusInternalServerError
			}
			status = sctx.status
		}
	}()
	sp.handler(sctx)
	return sctx.status
}

// Listen starts the HTTP server on the configured port.
// Uses net.Listen so OnReady fires after the port is bound, before first connections.
func (a *App) Listen() error {
	port := a.cfg.Port
	if port == 0 {
		port = 8080
	}

	if err := a.runInitHooks(); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("bast: listen: %w", err)
	}

	a.logger.OnListening(port)
	a.runReadyHooks()

	a.srv = a.buildServer()

	if err := a.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("bast: serve: %w", err)
	}
	return nil
}

// buildServer constructs the http.Server with safe defaults applied.
func (a *App) buildServer() *http.Server {
	return &http.Server{
		Handler:           a,
		ReadTimeout:       timeoutOrDefault(a.cfg.ReadTimeout, 0),
		WriteTimeout:      timeoutOrDefault(a.cfg.WriteTimeout, 0),
		IdleTimeout:       timeoutOrDefault(a.cfg.IdleTimeout, 120*time.Second),
		ReadHeaderTimeout: timeoutOrDefault(a.cfg.ReadHeaderTimeout, 10*time.Second),
		MaxHeaderBytes:    1 << 20,
	}
}

// timeoutOrDefault resolves the Config timeout convention:
// 0 → default, negative → disabled (0 in http.Server terms).
func timeoutOrDefault(v, def time.Duration) time.Duration {
	switch {
	case v < 0:
		return 0
	case v == 0:
		return def
	default:
		return v
	}
}

// Shutdown gracefully shuts down the server.
// If ctx carries no deadline, Config.ShutdownTimeout is applied.
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.OnShutdown()
	if a.srv == nil {
		return nil
	}
	sctx, cancel := shutdownContext(ctx, a.cfg.ShutdownTimeout)
	defer cancel()
	if err := a.srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("bast: server shutdown: %w", err)
	}
	a.runShutdownHooks(sctx)
	return nil
}

// shutdownContext adds a deadline to ctx unless the caller already set one.
func shutdownContext(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok || d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func (a *App) runInitHooks() error {
	for i, m := range a.modules {
		if h, ok := m.Controller.(Hooks); ok {
			ctx, cancel := context.WithTimeout(context.Background(), a.cfg.HookTimeout)
			err := h.OnInit(ctx)
			cancel()
			if err != nil {
				name := a.moduleNames[i]
				a.logger.Error("FATAL: %s.OnInit failed — refusing to start: %v", name, err)
				return fmt.Errorf("bast: %s.OnInit: %w", name, err)
			}
		}
	}
	return nil
}

func (a *App) runReadyHooks() {
	for _, m := range a.modules {
		if h, ok := m.Controller.(Hooks); ok {
			h.OnReady()
		}
	}
}

func (a *App) runShutdownHooks(ctx context.Context) {
	for i := len(a.modules) - 1; i >= 0; i-- {
		if h, ok := a.modules[i].Controller.(Hooks); ok {
			hookCtx, cancel := context.WithTimeout(ctx, a.cfg.HookTimeout)
			if err := h.OnShutdown(hookCtx); err != nil {
				a.logger.Error("shutdown hook failed for %s: %v", a.moduleNames[i], err)
			}
			cancel()
		}
	}
}

// parseCIDRs parses the TrustedProxies list. An invalid entry panics:
// silently dropping one would change which proxy headers are trusted,
// and a security control must fail loudly at startup, not degrade quietly.
func parseCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("bast: invalid TrustedProxies CIDR " + strconv.Quote(cidr) + ": " + err.Error())
		}
		out = append(out, network)
	}
	return out
}