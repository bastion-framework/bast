package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// runNew generates a complete, working Bast Todo app in dir/<appName>/.
func runNew(appName, dir string) error {
	root := filepath.Join(dir, appName)

	dirs := []string{
		filepath.Join(root, "modules", "todos"),
		filepath.Join(root, "shared", "guards"),
		filepath.Join(root, "shared", "errors"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	data := map[string]string{"App": appName, "Version": resolveVersion()}

	files := []struct {
		path string
		tmpl string
	}{
		{filepath.Join(root, "main.go"), tmplMain},
		{filepath.Join(root, "go.mod"), tmplGoMod},
		{filepath.Join(root, "modules", "todos", "todos.module.go"), tmplTodosModule},
		{filepath.Join(root, "modules", "todos", "todos.controller.go"), tmplTodosController},
		{filepath.Join(root, "modules", "todos", "todos.service.go"), tmplTodosService},
		{filepath.Join(root, "modules", "todos", "todos.repo.go"), tmplTodosRepo},
		{filepath.Join(root, "modules", "todos", "todos.dto.go"), tmplTodosDTO},
		{filepath.Join(root, "shared", "guards", "auth.guard.go"), tmplAuthGuard},
		{filepath.Join(root, "shared", "errors", "errors.go"), tmplSharedErrors},
	}

	for _, f := range files {
		if err := writeTemplate(f.path, f.tmpl, data); err != nil {
			return err
		}
	}
	return nil
}

func writeTemplate(path, tmplStr string, data any) error {
	t, err := template.New("").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parse template for %s: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	return t.Execute(f, data)
}

// ─── Project templates ────────────────────────────────────────────────────────

var tmplMain = `package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bastion-framework/bast"
	"github.com/bastion-framework/bast/middleware"

	"{{.App}}/modules/todos"
)

func main() {
	app := bast.New(bast.Config{
		Port:         8080,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		Docs: &bast.DocsConfig{
			Enabled:     true,
			Path:        "/docs",
			JSONPath:    "/openapi.json",
			Title:       "{{.App}} API",
			Version:     "0.1.0",
			Description: "A Todo API built with Bast",
		},
	})

	// RequestID and Recover are always recommended.
	// Logger middleware is optional — the framework already logs every
	// request via the built-in [Bast] logger. Add middleware.Logger only
	// if you want a separate slog-based log stream.
	app.Use(
		middleware.RequestID,
		middleware.Recover,
		middleware.CORS(middleware.CORSConfig{
			AllowedOrigins: []string{"*"},
		}),
	)

	app.Register(
		todos.NewModule(),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(); err != nil {
			slog.Error("server error", "err", err)
		}
	}()

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.Shutdown(ctx)
}
`

var tmplGoMod = `module {{.App}}

go 1.22

require github.com/bastion-framework/bast {{.Version}}
`

var tmplTodosModule = `package todos

import "github.com/bastion-framework/bast"

func NewModule() bast.Module {
	repo       := newRepo()
	service    := newService(repo)
	controller := newController(service)

	return bast.Module{
		Prefix:     "/todos",
		Controller: controller,
		Doc: bast.ModuleDoc{
			Name:        "Todos",
			Description: "Todo list management",
		},
	}
}
`

var tmplTodosController = `package todos

import "github.com/bastion-framework/bast"

type controller struct {
	service *service
}

func newController(s *service) *controller {
	return &controller{service: s}
}

func (c *controller) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/", c.List, bast.WithDoc(bast.Doc{
			Summary: "List all todos",
			Tags:    []string{"Todos"},
			Returns: bast.Returns{
				200: bast.Body[[]Todo](),
			},
		})),
		bast.GET("/:id", c.Get, bast.WithDoc(bast.Doc{
			Summary: "Get a todo by ID",
			Tags:    []string{"Todos"},
			Params:  []bast.Param{bast.PathParam("id", "Todo ID")},
			Returns: bast.Returns{
				200: bast.Body[Todo](),
				404: bast.Body[bast.BastError](),
			},
		})),
		bast.POST("/", c.Create, bast.WithDoc(bast.Doc{
			Summary: "Create a todo",
			Tags:    []string{"Todos"},
			Body:    bast.Body[CreateTodoRequest](),
			Returns: bast.Returns{
				201: bast.Body[Todo](),
				400: bast.Body[bast.BastError](),
			},
		})),
		bast.PATCH("/:id", c.Update, bast.WithDoc(bast.Doc{
			Summary: "Update a todo",
			Tags:    []string{"Todos"},
			Params:  []bast.Param{bast.PathParam("id", "Todo ID")},
			Body:    bast.Body[UpdateTodoRequest](),
			Returns: bast.Returns{
				200: bast.Body[Todo](),
				400: bast.Body[bast.BastError](),
				404: bast.Body[bast.BastError](),
			},
		})),
		bast.DELETE("/:id", c.Delete, bast.WithDoc(bast.Doc{
			Summary: "Delete a todo",
			Tags:    []string{"Todos"},
			Params:  []bast.Param{bast.PathParam("id", "Todo ID")},
			Returns: bast.Returns{
				204: nil,
				404: bast.Body[bast.BastError](),
			},
		})),
	}
}

func (c *controller) List(ctx *bast.Ctx) bast.Response {
	items, err := c.service.List(ctx.Context())
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.OK(items)
}

func (c *controller) Get(ctx *bast.Ctx) bast.Response {
	todo, err := c.service.GetByID(ctx.Context(), ctx.Param("id"))
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.OK(todo)
}

func (c *controller) Create(ctx *bast.Ctx) bast.Response {
	var req CreateTodoRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.Error(err)
	}
	todo, err := c.service.Create(ctx.Context(), req)
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.Created(todo)
}

func (c *controller) Update(ctx *bast.Ctx) bast.Response {
	var req UpdateTodoRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.Error(err)
	}
	todo, err := c.service.Update(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.OK(todo)
}

func (c *controller) Delete(ctx *bast.Ctx) bast.Response {
	if err := c.service.Delete(ctx.Context(), ctx.Param("id")); err != nil {
		return ctx.Error(err)
	}
	return ctx.NoContent()
}
`

var tmplTodosService = `package todos

import (
	"context"

	"github.com/bastion-framework/bast"
)

type service struct {
	repo *repo
}

func newService(r *repo) *service {
	return &service{repo: r}
}

func (s *service) List(ctx context.Context) ([]Todo, error) {
	return s.repo.findAll(ctx)
}

func (s *service) GetByID(ctx context.Context, id string) (*Todo, error) {
	todo, err := s.repo.findByID(ctx, id)
	if err != nil {
		return nil, bast.ErrNotFound("TODO_NOT_FOUND", "todo "+id+" not found")
	}
	return todo, nil
}

func (s *service) Create(ctx context.Context, req CreateTodoRequest) (*Todo, error) {
	return s.repo.create(ctx, req.Title)
}

func (s *service) Update(ctx context.Context, id string, req UpdateTodoRequest) (*Todo, error) {
	todo, err := s.repo.findByID(ctx, id)
	if err != nil {
		return nil, bast.ErrNotFound("TODO_NOT_FOUND", "todo "+id+" not found")
	}
	if req.Title != nil {
		todo.Title = *req.Title
	}
	if req.Done != nil {
		todo.Done = *req.Done
	}
	return s.repo.save(ctx, todo)
}

func (s *service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.findByID(ctx, id); err != nil {
		return bast.ErrNotFound("TODO_NOT_FOUND", "todo "+id+" not found")
	}
	return s.repo.delete(ctx, id)
}
`

var tmplTodosRepo = `package todos

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type repo struct {
	mu    sync.RWMutex
	store map[string]*Todo
	seq   int
}

func newRepo() *repo {
	r := &repo{store: make(map[string]*Todo)}
	// Seed with a couple of todos so the app feels alive immediately.
	r.store["1"] = &Todo{ID: "1", Title: "Buy groceries", Done: false, CreatedAt: time.Now()}
	r.store["2"] = &Todo{ID: "2", Title: "Read the Bast docs", Done: true, CreatedAt: time.Now()}
	r.seq = 2
	return r
}

func (r *repo) findAll(_ context.Context) ([]Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Todo, 0, len(r.store))
	for _, t := range r.store {
		out = append(out, *t)
	}
	return out, nil
}

func (r *repo) findByID(_ context.Context, id string) (*Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.store[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	copy := *t
	return &copy, nil
}

func (r *repo) create(_ context.Context, title string) (*Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	id := fmt.Sprintf("%d", r.seq)
	t := &Todo{ID: id, Title: title, Done: false, CreatedAt: time.Now()}
	r.store[id] = t
	copy := *t
	return &copy, nil
}

func (r *repo) save(_ context.Context, t *Todo) (*Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[t.ID] = t
	copy := *t
	return &copy, nil
}

func (r *repo) delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
	return nil
}
`

var tmplTodosDTO = `package todos

import "time"

// Todo is the canonical todo resource.
type Todo struct {
	ID        string    ` + "`json:\"id\"`" + `
	Title     string    ` + "`json:\"title\"`" + `
	Done      bool      ` + "`json:\"done\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
}

// CreateTodoRequest is the body for POST /todos.
type CreateTodoRequest struct {
	Title string ` + "`json:\"title\" validate:\"required,min=1,max=200\"`" + `
}

// UpdateTodoRequest is the body for PATCH /todos/:id.
// All fields are optional — only provided fields are updated.
type UpdateTodoRequest struct {
	Title *string ` + "`json:\"title,omitempty\"`" + `
	Done  *bool   ` + "`json:\"done,omitempty\"`" + `
}
`

var tmplAuthGuard = `package guards

import (
	"github.com/bastion-framework/bast"
)

// AuthGuard blocks unauthenticated requests.
// Replace the stub with real JWT verification for production use.
var AuthGuard = bast.GuardFunc(func(ctx *bast.Ctx) error {
	token := ctx.Header("Authorization")
	if token == "" {
		return bast.ErrUnauthorized(bast.CodeUnauthorized, "missing Authorization header")
	}
	// TODO: verify JWT, set claims:
	// claims, err := jwt.Verify(token)
	// if err != nil { return bast.ErrUnauthorized(bast.CodeUnauthorized, "invalid token") }
	// ctx.Set("claims", claims)
	return nil
})
`

var tmplSharedErrors = `package errors

import "github.com/bastion-framework/bast"

// NotFound wraps bast.ErrNotFound with a resource-scoped code.
func NotFound(resource, id string) error {
	return bast.ErrNotFound(resource+"_NOT_FOUND", resource+" "+id+" not found")
}

// Conflict wraps bast.ErrConflict with a resource-scoped code.
func Conflict(resource string) error {
	return bast.ErrConflict(resource+"_CONFLICT", resource+" already exists")
}
`
