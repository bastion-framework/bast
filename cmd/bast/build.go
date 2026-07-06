package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type buildOptions struct {
	out    string // output path, relative to the project dir; default bin/<app>
	goos   string // target GOOS; default host
	goarch string // target GOARCH; default host
}

// cmdBuild produces a production binary: reproducible (-trimpath), stripped
// (-s -w), and statically linked (CGO_ENABLED=0) so it runs in scratch and
// distroless containers. Returns the output path relative to dir.
func cmdBuild(dir string, opts buildOptions) (string, error) {
	name, err := moduleName(dir)
	if err != nil {
		return "", err
	}

	goos := opts.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	out := opts.out
	if out == "" {
		bin := name
		if goos == "windows" {
			bin += ".exe"
		}
		out = filepath.Join("bin", bin)
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", out, ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if opts.goos != "" {
		cmd.Env = append(cmd.Env, "GOOS="+opts.goos)
	}
	if opts.goarch != "" {
		cmd.Env = append(cmd.Env, "GOARCH="+opts.goarch)
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build failed: %w", err)
	}
	return out, nil
}

// moduleName returns the base name of the module path in dir's go.mod.
func moduleName(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("no go.mod found — run inside a Bast project")
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if mod, ok := strings.CutPrefix(line, "module "); ok {
			return path.Base(strings.TrimSpace(mod)), nil
		}
	}
	return "", fmt.Errorf("go.mod has no module directive")
}
