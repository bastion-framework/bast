package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/bastion-framework/bast"
)

// RequestID generates a unique request ID, stores it in the Ctx, and attaches
// it as X-Request-ID on the response.
func RequestID(next bast.HandlerFunc) bast.HandlerFunc {
	return func(ctx *bast.Ctx) bast.Response {
		id := newRequestID()
		ctx.Set("requestID", id)
		resp := next(ctx)
		return resp.WithHeader("X-Request-ID", id)
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}