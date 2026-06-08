package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// runGenerateModule generates the 5 files that make up a Bast module.
// dir should already be the target directory (e.g. "modules/users").
func runGenerateModule(name, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	pkg := toPackageName(name)
	title := toTitle(name)
	data := map[string]string{
		"Pkg":   pkg,
		"Title": title,
		"Name":  name,
	}

	files := []struct {
		path string
		tmpl string
	}{
		{filepath.Join(dir, name+".module.go"), tmplModule},
		{filepath.Join(dir, name+".controller.go"), tmplController},
		{filepath.Join(dir, name+".service.go"), tmplService},
		{filepath.Join(dir, name+".repo.go"), tmplRepo},
		{filepath.Join(dir, name+".dto.go"), tmplDTO},
	}

	for _, f := range files {
		if err := writeTemplate(f.path, f.tmpl, data); err != nil {
			return err
		}
	}
	return nil
}

// runGenerateGuard generates a single guard file.
// dir should already be the target directory (e.g. "shared/guards").
func runGenerateGuard(name, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	pkg := toPackageName(name)
	title := toTitle(name)
	data := map[string]string{"Pkg": pkg, "Title": title, "Name": name}
	return writeTemplate(filepath.Join(dir, name+".guard.go"), tmplGuard, data)
}

// runGenerateService generates a single shared service file.
// dir should already be the target directory (e.g. "shared/services").
func runGenerateService(name, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	pkg := toPackageName(name)
	title := toTitle(name)
	data := map[string]string{"Pkg": pkg, "Title": title, "Name": name}
	return writeTemplate(filepath.Join(dir, name+".service.go"), tmplService, data)
}

// ─── Generate templates ───────────────────────────────────────────────────────

var tmplModule = `package {{.Pkg}}

import "github.com/bastion-framework/bast"

func NewModule() bast.Module {
	repo       := newRepo()
	service    := newService(repo)
	controller := newController(service)

	return bast.Module{
		Prefix:     "/{{.Name}}",
		Controller: controller,
		Doc: bast.ModuleDoc{
			Name:        "{{.Title}}",
			Description: "{{.Title}} management",
		},
	}
}
`

var tmplController = `package {{.Pkg}}

import "github.com/bastion-framework/bast"

type controller struct {
	service *service
}

func newController(s *service) *controller {
	return &controller{service: s}
}

func (c *controller) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/", c.List),
		bast.GET("/:id", c.Get),
		bast.POST("/", c.Create),
		bast.PATCH("/:id", c.Update),
		bast.DELETE("/:id", c.Delete),
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
	item, err := c.service.GetByID(ctx.Context(), ctx.Param("id"))
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.OK(item)
}

func (c *controller) Create(ctx *bast.Ctx) bast.Response {
	var req CreateRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.Error(err)
	}
	item, err := c.service.Create(ctx.Context(), req)
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.Created(item)
}

func (c *controller) Update(ctx *bast.Ctx) bast.Response {
	var req UpdateRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.Error(err)
	}
	item, err := c.service.Update(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		return ctx.Error(err)
	}
	return ctx.OK(item)
}

func (c *controller) Delete(ctx *bast.Ctx) bast.Response {
	if err := c.service.Delete(ctx.Context(), ctx.Param("id")); err != nil {
		return ctx.Error(err)
	}
	return ctx.NoContent()
}
`

var tmplService = `package {{.Pkg}}

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

func (s *service) List(ctx context.Context) ([]*{{.Title}}, error) {
	return s.repo.findAll(ctx)
}

func (s *service) GetByID(ctx context.Context, id string) (*{{.Title}}, error) {
	item, err := s.repo.findByID(ctx, id)
	if err != nil {
		return nil, bast.ErrNotFound("{{.Title}}_NOT_FOUND", "{{.Name}} "+id+" not found")
	}
	return item, nil
}

func (s *service) Create(ctx context.Context, req CreateRequest) (*{{.Title}}, error) {
	return s.repo.create(ctx, req)
}

func (s *service) Update(ctx context.Context, id string, req UpdateRequest) (*{{.Title}}, error) {
	item, err := s.repo.findByID(ctx, id)
	if err != nil {
		return nil, bast.ErrNotFound("{{.Title}}_NOT_FOUND", "{{.Name}} "+id+" not found")
	}
	_ = req
	return s.repo.save(ctx, item)
}

func (s *service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.findByID(ctx, id); err != nil {
		return bast.ErrNotFound("{{.Title}}_NOT_FOUND", "{{.Name}} "+id+" not found")
	}
	return s.repo.delete(ctx, id)
}
`

var tmplRepo = `package {{.Pkg}}

import (
	"context"
	"fmt"
	"sync"
)

type repo struct {
	mu    sync.RWMutex
	store map[string]*{{.Title}}
	seq   int
}

func newRepo() *repo {
	return &repo{store: make(map[string]*{{.Title}})}
}

func (r *repo) findAll(_ context.Context) ([]*{{.Title}}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*{{.Title}}, 0, len(r.store))
	for _, v := range r.store {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *repo) findByID(_ context.Context, id string) (*{{.Title}}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.store[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *v
	return &cp, nil
}

func (r *repo) create(_ context.Context, req CreateRequest) (*{{.Title}}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	item := &{{.Title}}{ID: fmt.Sprintf("%d", r.seq)}
	_ = req
	r.store[item.ID] = item
	cp := *item
	return &cp, nil
}

func (r *repo) save(_ context.Context, item *{{.Title}}) (*{{.Title}}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[item.ID] = item
	cp := *item
	return &cp, nil
}

func (r *repo) delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
	return nil
}
`

var tmplDTO = `package {{.Pkg}}

// {{.Title}} is the canonical resource.
type {{.Title}} struct {
	ID string ` + "`json:\"id\"`" + `
}

// CreateRequest is the request body for POST /{{.Name}}.
type CreateRequest struct {
	// TODO: add fields
}

// UpdateRequest is the request body for PATCH /{{.Name}}/:id.
type UpdateRequest struct {
	// TODO: add fields
}
`

var tmplGuard = `package guards

import "github.com/bastion-framework/bast"

// {{.Title}}Guard — fill in your check logic below.
var {{.Title}}Guard = bast.GuardFunc(func(ctx *bast.Ctx) error {
	// Return nil to allow, return an error to block.
	// Example: check a role stored by a prior auth guard.
	// claims, ok := bast.Get[*Claims](ctx, "claims")
	// if !ok || claims.Role != "{{.Name}}" {
	//     return bast.ErrForbidden(bast.CodeForbidden, "insufficient role")
	// }
	return nil
})
`


// injectModuleIntoMain adds the import and app.Register call for a new module
// to the nearest main.go walking up from the current directory.
func injectModuleIntoMain(name string) error {
	mainPath, err := findFile("main.go")
	if err != nil {
		return fmt.Errorf("main.go not found")
	}
	modPath, err := readModulePath(filepath.Dir(mainPath))
	if err != nil {
		return fmt.Errorf("go.mod not found")
	}

	pkg := toPackageName(name)
	importPath := modPath + "/modules/" + name

	data, err := os.ReadFile(mainPath)
	if err != nil {
		return fmt.Errorf("read main.go: %w", err)
	}

	src := string(data)

	// Skip if already registered.
	if strings.Contains(src, importPath) {
		return nil
	}

	src = addImport(src, importPath)
	src = addToRegister(src, pkg+".NewModule()")

	return os.WriteFile(mainPath, []byte(src), 0644)
}

// findFile walks up from CWD looking for a file by name (up to 4 levels).
func findFile(name string) (string, error) {
	dir, _ := os.Getwd()
	for range 5 {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("%s not found", name)
}

// readModulePath extracts the module path from go.mod in dir.
func readModulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found")
}

// addImport inserts an import path before the closing ) of the import block.
func addImport(src, importPath string) string {
	importLine := "\t\"" + importPath + "\""
	if strings.Contains(src, importLine) {
		return src
	}
	// Find the import block's closing ).
	importBlock := strings.Index(src, "import (")
	if importBlock == -1 {
		return src
	}
	closing := strings.Index(src[importBlock:], "\n)")
	if closing == -1 {
		return src
	}
	pos := importBlock + closing
	return src[:pos] + "\n" + importLine + src[pos:]
}

// addToRegister inserts a NewModule() call inside app.Register(...).
func addToRegister(src, call string) string {
	if strings.Contains(src, call) {
		return src
	}
	registerIdx := strings.Index(src, "app.Register(")
	if registerIdx == -1 {
		return src
	}
	// Find the closing ) of app.Register — look for the first \n\t)
	rest := src[registerIdx:]
	closing := strings.Index(rest, "\n\t)")
	if closing == -1 {
		return src
	}
	pos := registerIdx + closing
	return src[:pos] + "\n\t\t" + call + "," + src[pos:]
}

// ─── Name helpers ─────────────────────────────────────────────────────────────

// toPackageName converts a name like "my-module" or "MyModule" to "mymodule".
func toPackageName(name string) string {
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	return strings.ToLower(name)
}

// toTitle converts "my-module" or "myModule" to "MyModule".
func toTitle(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	var b strings.Builder
	for _, w := range words {
		runes := []rune(w)
		if len(runes) > 0 {
			b.WriteRune(unicode.ToUpper(runes[0]))
			for _, r := range runes[1:] {
				b.WriteRune(r)
			}
		}
	}
	if b.Len() == 0 {
		return name
	}
	return b.String()
}

// ensure os is used
var _ = os.Create