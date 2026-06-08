package middleware

import (
	"fmt"
	"log/slog"

	"github.com/bastion-framework/bast"
)

// Recover catches panics in downstream handlers, logs them, and returns a 500.
// Framework code must not panic — this catches user handler panics only.
func Recover(next bast.HandlerFunc) bast.HandlerFunc {
	return func(ctx *bast.Ctx) (resp bast.Response) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered panic in handler",
					"panic", fmt.Sprintf("%v", r),
					"path", ctx.Path(),
					"method", ctx.Method(),
				)
				resp = bast.NewErrorResponse(
					bast.ErrInternal(bast.CodeInternal, "internal server error"),
				)
			}
		}()
		return next(ctx)
	}
}