package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// cmdRun runs the app in dir. With watch, it rebuilds and restarts on any
// change to .go files, go.mod, or go.sum.
func cmdRun(dir string, watch bool) error {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return fmt.Errorf("no go.mod found — run inside a Bast project")
	}
	if watch {
		return watchAndRun(dir)
	}

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ctrl+C goes to the whole console group. Ignore it here so the app gets
	// to shut down gracefully; its exit ends Run.
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)

	err := cmd.Run()
	select {
	case <-interrupted:
		return nil // user-initiated stop is not a failure
	default:
		return err
	}
}

const (
	watchInterval = 500 * time.Millisecond
	debounce      = 200 * time.Millisecond
	stopGrace     = 3 * time.Second
)

// watchAndRun builds the app to a temp binary and restarts it on source
// changes. It builds + execs rather than `go run` because killing `go run`
// on Windows orphans the grandchild server process.
//
// The build always targets a fresh temp path and the old process is stopped
// only after the new binary compiles: a broken save keeps the last good
// process serving, and Windows cannot overwrite a running exe anyway.
func watchAndRun(dir string) error {
	base := filepath.Join(os.TempDir(), fmt.Sprintf("bast-run-%d", os.Getpid()))
	seq := 0
	nextBin := func() string {
		seq++
		p := fmt.Sprintf("%s-%d", base, seq)
		if runtime.GOOS == "windows" {
			p += ".exe"
		}
		return p
	}

	var proc *exec.Cmd
	var curBin string
	defer func() { os.Remove(curBin) }()

	stop := func() {
		if proc == nil || proc.Process == nil {
			return
		}
		_ = interruptProcess(proc)
		done := make(chan struct{})
		go func() { _ = proc.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(stopGrace):
			_ = proc.Process.Kill()
			<-done
		}
		proc = nil
	}

	buildAndSwap := func() {
		bin := nextBin()
		bcmd := exec.Command("go", "build", "-o", bin, ".")
		bcmd.Dir = dir
		bcmd.Stdout = os.Stdout
		bcmd.Stderr = os.Stderr
		if err := bcmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "bast run: build failed — keeping previous process, waiting for changes")
			return
		}
		stop()
		os.Remove(curBin)
		curBin = bin

		p := exec.Command(bin)
		p.Dir = dir
		p.Stdin = os.Stdin
		p.Stdout = os.Stdout
		p.Stderr = os.Stderr
		if err := p.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "bast run: start failed: %v\n", err)
			return
		}
		proc = p
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	fmt.Println("bast run --watch: watching for changes (Ctrl+C to stop)")
	prev, err := snapshotSources(dir)
	if err != nil {
		return err
	}
	buildAndSwap()

	tick := time.NewTicker(watchInterval)
	defer tick.Stop()
	for {
		select {
		case <-sig:
			stop()
			return nil
		case <-tick.C:
			cur, err := snapshotSources(dir)
			if err != nil || !sourcesChanged(prev, cur) {
				continue
			}
			time.Sleep(debounce) // let rapid successive saves settle
			prev, _ = snapshotSources(dir)
			fmt.Println("bast run: change detected — rebuilding")
			buildAndSwap()
		}
	}
}

// interruptProcess asks the child to shut down gracefully. Windows cannot
// deliver os.Interrupt to another process, so it gets a hard kill there.
func interruptProcess(c *exec.Cmd) error {
	if runtime.GOOS == "windows" {
		return c.Process.Kill()
	}
	return c.Process.Signal(os.Interrupt)
}

// watchIgnore lists directory names excluded from the file watcher.
var watchIgnore = map[string]bool{
	".git":         true,
	".idea":        true,
	".vscode":      true,
	"bin":          true,
	"dist":         true,
	"node_modules": true,
	"vendor":       true,
}

// snapshotSources maps every watched file (.go, go.mod, go.sum) to its modtime.
func snapshotSources(root string) (map[string]int64, error) {
	snap := make(map[string]int64)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // file vanished mid-walk; pick it up next tick
		}
		if d.IsDir() {
			if watchIgnore[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") || d.Name() == "go.mod" || d.Name() == "go.sum" {
			if info, ierr := d.Info(); ierr == nil {
				snap[p] = info.ModTime().UnixNano()
			}
		}
		return nil
	})
	return snap, err
}

func sourcesChanged(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range a {
		if b[k] != v {
			return true
		}
	}
	return false
}
