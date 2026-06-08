package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_CreatesProjectStructure(t *testing.T) {
	dir := t.TempDir()
	if err := runNew("testapp", dir); err != nil {
		t.Fatalf("runNew: %v", err)
	}

	mustExist(t, dir, "testapp/main.go")
	mustExist(t, dir, "testapp/go.mod")
	mustExist(t, dir, "testapp/modules/todos/todos.module.go")
	mustExist(t, dir, "testapp/modules/todos/todos.controller.go")
	mustExist(t, dir, "testapp/modules/todos/todos.service.go")
	mustExist(t, dir, "testapp/modules/todos/todos.repo.go")
	mustExist(t, dir, "testapp/modules/todos/todos.dto.go")
	mustExist(t, dir, "testapp/shared/guards/auth.guard.go")
	mustExist(t, dir, "testapp/shared/errors/errors.go")
}

func TestNew_MainGoImportsBast(t *testing.T) {
	dir := t.TempDir()
	_ = runNew("myapp", dir)

	content := readFile(t, filepath.Join(dir, "myapp", "main.go"))
	if !strings.Contains(content, "github.com/bastion-framework/bast") {
		t.Error("main.go should import bast")
	}
	if !strings.Contains(content, "bast.New") {
		t.Error("main.go should call bast.New")
	}
	if !strings.Contains(content, "app.Listen") {
		t.Error("main.go should call app.Listen")
	}
}

func TestNew_GoModContainsAppName(t *testing.T) {
	dir := t.TempDir()
	_ = runNew("payments-api", dir)

	content := readFile(t, filepath.Join(dir, "payments-api", "go.mod"))
	if !strings.Contains(content, "payments-api") {
		t.Errorf("go.mod should contain module name, got:\n%s", content)
	}
}

func TestGenerateModule_CreatesAllFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "modules", "payments")
	if err := runGenerateModule("payments", dir); err != nil {
		t.Fatalf("runGenerateModule: %v", err)
	}

	mustExist(t, dir, "payments.module.go")
	mustExist(t, dir, "payments.controller.go")
	mustExist(t, dir, "payments.service.go")
	mustExist(t, dir, "payments.repo.go")
	mustExist(t, dir, "payments.dto.go")
}

func TestGenerateModule_ModuleFileHasCorrectPackage(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "modules", "orders")
	_ = runGenerateModule("orders", dir)

	content := readFile(t, filepath.Join(dir, "orders.module.go"))
	if !strings.Contains(content, "package orders") {
		t.Errorf("module file should declare package orders, got:\n%s", content)
	}
	if !strings.Contains(content, "bast.Module") {
		t.Errorf("module file should reference bast.Module, got:\n%s", content)
	}
}

func TestGenerateModule_ControllerImplementsInterface(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "modules", "users")
	_ = runGenerateModule("users", dir)

	content := readFile(t, filepath.Join(dir, "users.controller.go"))
	if !strings.Contains(content, "func") || !strings.Contains(content, "Routes()") {
		t.Errorf("controller should implement Routes(), got:\n%s", content)
	}
}

func TestGenerateGuard_CreatesFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "shared", "guards")
	if err := runGenerateGuard("admin", dir); err != nil {
		t.Fatalf("runGenerateGuard: %v", err)
	}

	mustExist(t, dir, "admin.guard.go")
	content := readFile(t, filepath.Join(dir, "admin.guard.go"))
	if !strings.Contains(content, "bast.Guard") && !strings.Contains(content, "bast.GuardFunc") {
		t.Errorf("guard file should reference bast.Guard or GuardFunc, got:\n%s", content)
	}
}

func TestGenerateService_CreatesFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "shared", "services")
	if err := runGenerateService("token", dir); err != nil {
		t.Fatalf("runGenerateService: %v", err)
	}

	mustExist(t, dir, "token.service.go")
	content := readFile(t, filepath.Join(dir, "token.service.go"))
	if !strings.Contains(content, "Service") {
		t.Errorf("service file should contain Service type, got:\n%s", content)
	}
}

func TestGenerateModule_InjectsIntoMain(t *testing.T) {
	root := t.TempDir()

	// Write a minimal go.mod and main.go simulating a real project.
	writeFile(t, filepath.Join(root, "go.mod"), "module myapp\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "main.go"), `package main

import (
	"github.com/bastion-framework/bast"
	"myapp/modules/todos"
)

func main() {
	app := bast.New(bast.Config{})
	app.Register(
		todos.NewModule(),
	)
	app.Listen()
}
`)

	// Change to root so findFile walks from there.
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(root)

	dir := filepath.Join(root, "modules", "payments")
	if err := runGenerateModule("payments", dir); err != nil {
		t.Fatalf("runGenerateModule: %v", err)
	}
	if err := injectModuleIntoMain("payments"); err != nil {
		t.Fatalf("injectModuleIntoMain: %v", err)
	}

	content := readFile(t, filepath.Join(root, "main.go"))

	if !strings.Contains(content, `"myapp/modules/payments"`) {
		t.Errorf("import not added to main.go:\n%s", content)
	}
	if !strings.Contains(content, "payments.NewModule()") {
		t.Errorf("NewModule() not added to app.Register:\n%s", content)
	}
	// Existing todos import and registration must be preserved.
	if !strings.Contains(content, `"myapp/modules/todos"`) {
		t.Errorf("existing todos import removed from main.go:\n%s", content)
	}
	if !strings.Contains(content, "todos.NewModule()") {
		t.Errorf("existing todos registration removed from main.go:\n%s", content)
	}
}

// helpers

// mustExist checks that a file exists. If rel is provided, it is joined with base.
// If base already is the full path to the file's directory, pass "" for rel.
func mustExist(t *testing.T, base, rel string) {
	t.Helper()
	var full string
	if rel == "" {
		full = base
	} else {
		full = filepath.Join(base, filepath.FromSlash(rel))
	}
	if _, err := os.Stat(full); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", full)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
