package bast

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const bastVersion = "0.1.0"

// Logger is the pluggable logging interface for the Bast framework.
// The default implementation produces colored output to stdout.
// Swap it out via Config.Logger for Zap, Zerolog, or any other backend.
type Logger interface {
	OnBoot(version string)
	OnModuleRegistered(name, prefix string)
	OnRouteRegistered(method, path string, guards []string)
	OnServiceExported(serviceType string, fromModule string)
	OnServiceResolved(serviceType string, toModule string)
	OnListening(port int)
	OnShutdown()
	OnRequest(method, path string, status int, dur time.Duration, ip string)
	OnError(ctx *Ctx, err error)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// ANSI color codes. Disabled automatically when NO_COLOR is set or output is not a tty.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
	ansiWhite   = "\033[97m"
)

const logSep = "─────────────────────────────────────────────"

// defaultLogger is the built-in Logger. Outputs [Bast]-prefixed, colored lines.
type defaultLogger struct {
	mu     sync.Mutex
	out    io.Writer
	colors bool
}

// NewDefaultLogger returns the default Bast logger writing to stdout.
func NewDefaultLogger() Logger {
	colors := os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	return &defaultLogger{out: os.Stdout, colors: colors}
}

func (l *defaultLogger) tag() string {
	if l.colors {
		return ansiYellow + "[Bast]" + ansiReset
	}
	return "[Bast]"
}

func (l *defaultLogger) colorize(s, code string) string {
	if !l.colors {
		return s
	}
	return code + s + ansiReset
}

func (l *defaultLogger) sep() string {
	return l.tag() + " " + l.colorize(logSep, ansiGray)
}

func (l *defaultLogger) print(line string) {
	l.mu.Lock()
	fmt.Fprintln(l.out, line)
	l.mu.Unlock()
}

func (l *defaultLogger) OnBoot(version string) {
	l.print(l.sep())
	l.print(l.tag() + " " + l.colorize("Bastion Framework", ansiBold) + " " + l.colorize("v"+version, ansiGreen))
	l.print(l.sep())
}

func (l *defaultLogger) OnModuleRegistered(name, prefix string) {
	line := l.tag() + " " +
		l.colorize(fmt.Sprintf("%-10s", "Module"), ansiDim) +
		l.colorize(fmt.Sprintf("%-20s", name), ansiCyan+ansiBold) +
		l.colorize(prefix, ansiGray)
	l.print(line)
}

func (l *defaultLogger) OnRouteRegistered(method, path string, guards []string) {
	methodColor := methodColor(method, l.colors)
	line := l.tag() + "   " +
		fmt.Sprintf("%-8s", methodColor) +
		l.colorize(path, ansiWhite)
	for _, g := range guards {
		line += "  " + l.colorize("["+g+"]", ansiMagenta)
	}
	l.print(line)
}

func (l *defaultLogger) OnServiceExported(serviceType, fromModule string) {
	l.print(l.tag() + "   " +
		l.colorize(fmt.Sprintf("%-8s", "Export"), ansiGray) +
		l.colorize(serviceType, ansiCyan) +
		l.colorize("  ("+fromModule+")", ansiGray))
}

func (l *defaultLogger) OnServiceResolved(serviceType, toModule string) {
	l.print(l.tag() + "   " +
		l.colorize(fmt.Sprintf("%-8s", "Import"), ansiGray) +
		l.colorize(serviceType, ansiCyan) +
		l.colorize("  ← resolved from "+toModule, ansiGray))
}

func (l *defaultLogger) OnListening(port int) {
	l.print(l.sep())
	l.print(l.tag() + " " +
		l.colorize("Listening on ", ansiBold) +
		l.colorize(fmt.Sprintf(":%d", port), ansiGreen+ansiBold))
	l.print(l.sep())
}

func (l *defaultLogger) OnShutdown() {
	l.print(l.tag() + " " + l.colorize("Shutting down", ansiYellow))
}

func (l *defaultLogger) OnRequest(method, path string, status int, dur time.Duration, ip string) {
	statusStr := fmt.Sprintf("%d", status)
	var statusColor string
	switch {
	case status >= 500:
		statusColor = ansiRed + ansiBold
	case status >= 400:
		statusColor = ansiYellow
	default:
		statusColor = ansiGreen
	}

	durStr := formatDur(dur)

	line := l.tag() + " " +
		fmt.Sprintf("%-8s", methodColor(method, l.colors)) +
		fmt.Sprintf("%-30s", l.colorize(path, ansiWhite)) +
		l.colorize(fmt.Sprintf("%-6s", statusStr), statusColor) +
		l.colorize(fmt.Sprintf("%-10s", durStr), ansiDim)
	if ip != "" {
		line += l.colorize(ip, ansiGray)
	}
	l.print(line)
}

func (l *defaultLogger) OnError(ctx *Ctx, err error) {
	l.print(l.tag() + " " +
		l.colorize("[ERROR]", ansiRed+ansiBold) + " " +
		l.colorize(ctx.Path(), ansiGray) + " " +
		l.colorize(err.Error(), ansiRed))
}

func (l *defaultLogger) Info(msg string, args ...any) {
	l.print(l.tag() + " " + l.colorize("[INFO]  ", ansiGreen) + fmt.Sprintf(msg, maybeArgs(args)...))
}

func (l *defaultLogger) Warn(msg string, args ...any) {
	l.print(l.tag() + " " + l.colorize("[WARN]  ", ansiYellow) + fmt.Sprintf(msg, maybeArgs(args)...))
}

func (l *defaultLogger) Error(msg string, args ...any) {
	l.print(l.tag() + " " + l.colorize("[ERROR] ", ansiRed) + fmt.Sprintf(msg, maybeArgs(args)...))
}

func (l *defaultLogger) Debug(msg string, args ...any) {
	l.print(l.tag() + " " + l.colorize("[DEBUG] ", ansiGray) + fmt.Sprintf(msg, maybeArgs(args)...))
}

// methodColor returns a padded, optionally colored HTTP method string.
func methodColor(method string, colors bool) string {
	if !colors {
		return method
	}
	var code string
	switch method {
	case "GET":
		code = ansiGreen
	case "POST":
		code = ansiYellow
	case "PUT":
		code = ansiBlue
	case "PATCH":
		code = ansiMagenta
	case "DELETE":
		code = ansiRed
	default:
		code = ansiWhite
	}
	return code + method + ansiReset
}

func formatDur(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fμs", float64(d.Microseconds()))
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Milliseconds()))
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func maybeArgs(args []any) []any {
	if len(args) == 0 {
		return nil
	}
	return args
}

// guardNames extracts display names from a slice of Guard values.
func guardNames(guards []Guard) []string {
	names := make([]string, 0, len(guards))
	for _, g := range guards {
		name := fmt.Sprintf("%T", g)
		// Strip package path: "pkg.Type" → "Type"
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		names = append(names, name)
	}
	return names
}