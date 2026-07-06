package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// tinyModule scaffolds a dependency-free buildable module so tests work offline.
func tinyModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module tinyapp\n\ngo 1.22\n")
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	return dir
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestModuleName_ParsesGoMod(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module github.com/acme/payments-api\n\ngo 1.22\n")
	name, err := moduleName(dir)
	if err != nil {
		t.Fatalf("moduleName: %v", err)
	}
	if name != "payments-api" {
		t.Errorf("moduleName = %q, want payments-api (base of module path)", name)
	}
}

func TestModuleName_MissingGoMod(t *testing.T) {
	if _, err := moduleName(t.TempDir()); err == nil {
		t.Fatal("moduleName should error without go.mod")
	}
}

func TestBuild_ProducesBinary(t *testing.T) {
	dir := tinyModule(t)
	out, err := cmdBuild(dir, buildOptions{})
	if err != nil {
		t.Fatalf("cmdBuild: %v", err)
	}
	want := "tinyapp"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if filepath.Base(out) != want {
		t.Errorf("binary name = %q, want %q", filepath.Base(out), want)
	}
	info, err := os.Stat(filepath.Join(dir, "bin", want))
	if err != nil {
		t.Fatalf("binary not written to bin/: %v", err)
	}
	if info.Size() == 0 {
		t.Error("binary is empty")
	}
}

func TestBuild_CustomOutput(t *testing.T) {
	dir := tinyModule(t)
	_, err := cmdBuild(dir, buildOptions{out: filepath.Join("dist", "svc.bin")})
	if err != nil {
		t.Fatalf("cmdBuild: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dist", "svc.bin")); err != nil {
		t.Fatalf("custom output path not honored: %v", err)
	}
}

func TestBuild_CrossCompileLinux(t *testing.T) {
	dir := tinyModule(t)
	out, err := cmdBuild(dir, buildOptions{goos: "linux", goarch: "amd64"})
	if err != nil {
		t.Fatalf("cross-compile: %v", err)
	}
	if filepath.Ext(out) == ".exe" {
		t.Errorf("linux binary must not have .exe suffix: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "tinyapp")); err != nil {
		t.Fatalf("linux binary not written: %v", err)
	}
}

func TestRun_ErrorsWithoutGoMod(t *testing.T) {
	if err := cmdRun(t.TempDir(), false); err == nil {
		t.Fatal("cmdRun outside a module should error, not invoke go run")
	}
}

func TestSnapshotSources_SkipsIgnoredDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module x\n")
	writeTestFile(t, dir, "main.go", "package main\n")
	writeTestFile(t, dir, filepath.Join(".git", "hook.go"), "x")
	writeTestFile(t, dir, filepath.Join("vendor", "dep.go"), "x")
	writeTestFile(t, dir, filepath.Join("bin", "gen.go"), "x")
	writeTestFile(t, dir, "notes.txt", "not watched")

	snap, err := snapshotSources(dir)
	if err != nil {
		t.Fatalf("snapshotSources: %v", err)
	}
	if len(snap) != 2 {
		t.Errorf("snapshot = %d entries %v, want 2 (main.go + go.mod)", len(snap), snap)
	}
}

func TestSnapshotSources_DetectsChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "main.go", "package main\n")
	before, _ := snapshotSources(dir)

	// Backdate then touch so the modtime definitely differs across filesystems.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "main.go"), old, old); err != nil {
		t.Fatal(err)
	}
	after, _ := snapshotSources(dir)
	if !sourcesChanged(before, after) {
		t.Error("modtime change not detected")
	}

	writeTestFile(t, dir, "extra.go", "package main\n")
	after2, _ := snapshotSources(dir)
	if !sourcesChanged(after, after2) {
		t.Error("new file not detected")
	}
}
