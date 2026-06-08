package middleware

import (
	"log/slog"
	"time"

	"github.com/bastion-framework/bast"
)

// Logger logs method, path, status code, duration, and client IP for every request.
func Logger(next bast.HandlerFunc) bast.HandlerFunc {
	return func(ctx *bast.Ctx) bast.Response {
		start := time.Now()
		resp := next(ctx)
		dur := time.Since(start)

		args := []any{
			"method", ctx.Method(),
			"path", ctx.Path(),
			"status", resp.Status(),
			"dur", dur,
			"ip", ctx.IP(),
		}
		if resp.IsError() {
			slog.Error("request", args...)
		} else {
			slog.Info("request", args...)
		}
		return resp
	}
}