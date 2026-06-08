package bast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bastion-framework/bast/router"
)

// Config holds app-level settings.
type Config struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxBodySize     int64
	HandlerTimeout  time.Duration
	ShutdownTimeout time.Duration
	HookTimeout     time.Duration
	TrustedProxies  []string
	ErrorHandler    ErrorHandler
	Validator       Validator
	Health          *HealthConfig
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

	proxies := parseCIDRs(cfg.TrustedProxies)

	app := &App{
		cfg:        cfg,
		router:     router.New(),
		errHandler: errHandler,
		proxies:    proxies,
	}
	app.registerHealthRoutes()
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

	if m.Controller != nil {
		for _, route := range m.Controller.Routes() {
			a.registerRoute(route, prefix, m.Middleware, m.Guards)
		}
	}

	for _, sub := range m.Modules {
		a.registerModule(sub, prefix)
	}
}

func (a *App) registerRoute(r Route, prefix string, modMW []MiddlewareFunc, modGuards []Guard) {
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

	if r.Stream != nil {
		// Streaming route — framework steps aside after handing off.
		streamHandler := r.Stream
		a.router.Add(r.Method, fullPattern, streamHandler)
		return
	}

	// Wrap guards into a middleware that short-circuits on failure.
	handler := r.Handler
	if len(allGuards) > 0 {
		handler = guardMiddleware(allGuards, handler, a.errHandler)
	}

	// Build the pipeline at registration time.
	pipeline := buildPipeline(handler, allMW)
	a.router.Add(r.Method, fullPattern, pipeline)
}

// guardMiddleware wraps guards into a HandlerFunc chain.
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
	ctx := acquireCtx(w, r, a.cfg.MaxBodySize, a.proxies, a.cfg.Validator)
	defer releaseCtx(ctx)

	match, err := a.router.Find(r.Method, r.URL.Path)
	if err != nil {
		var mna *router.MethodNotAllowedError
		if errors.As(err, &mna) {
			allow := ""
			for i, m := range mna.Allow {
				if i > 0 {
					allow += ", "
				}
				allow += m
			}
			w.Header().Set("Allow", allow)
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

	// Set route params on Ctx.
	for k, v := range match.Params {
		ctx.params[k] = v
	}

	switch h := match.Handler.(type) {
	case StreamHandlerFunc:
		sctx := newStreamCtx(r.Context(), w, r)
		h(sctx)
	case HandlerFunc:
		resp := h(ctx)
		if resp.IsError() {
			resp = a.errHandler(ctx, resp.Err())
		}
		writeResponse(w, resp)
	default:
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

// Listen starts the HTTP server on the configured port.
func (a *App) Listen() error {
	port := a.cfg.Port
	if port == 0 {
		port = 8080
	}

	if err := a.runInitHooks(); err != nil {
		return err
	}

	a.srv = &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        a,
		ReadTimeout:    a.cfg.ReadTimeout,
		WriteTimeout:   a.cfg.WriteTimeout,
		IdleTimeout:    a.cfg.IdleTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("bast: listen: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (a *App) Shutdown(ctx context.Context) error {
	if a.srv == nil {
		return nil
	}
	if err := a.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("bast: server shutdown: %w", err)
	}
	a.runShutdownHooks(ctx)
	return nil
}

func (a *App) runInitHooks() error {
	for i, m := range a.modules {
		if h, ok := m.Controller.(Hooks); ok {
			ctx, cancel := context.WithTimeout(context.Background(), a.cfg.HookTimeout)
			err := h.OnInit(ctx)
			cancel()
			if err != nil {
				name := a.moduleNames[i]
				slog.Error("FATAL: module OnInit failed — refusing to start",
					"module", name, "err", err)
				return fmt.Errorf("bast: %s.OnInit: %w", name, err)
			}
		}
	}
	return nil
}

func (a *App) runShutdownHooks(ctx context.Context) {
	for i := len(a.modules) - 1; i >= 0; i-- {
		if h, ok := a.modules[i].Controller.(Hooks); ok {
			hookCtx, cancel := context.WithTimeout(ctx, a.cfg.HookTimeout)
			if err := h.OnShutdown(hookCtx); err != nil {
				slog.Error("shutdown hook failed",
					"module", a.moduleNames[i], "err", err)
			}
			cancel()
		}
	}
}

// parseCIDRs parses a list of CIDR strings, silently ignoring invalid entries.
func parseCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			out = append(out, network)
		}
	}
	return out
}