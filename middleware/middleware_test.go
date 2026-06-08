package middleware_test

import (
	"strings"
	"testing"

	"github.com/bastion-framework/bast"
	"github.com/bastion-framework/bast/basttest"
	"github.com/bastion-framework/bast/middleware"
)

// --- RequestID ---

func TestRequestID_SetsHeader(t *testing.T) {
	handler := middleware.RequestID(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx()
	resp := handler(ctx)

	if resp.Headers()["X-Request-ID"] == "" {
		t.Error("X-Request-ID header not set on response")
	}
}

func TestRequestID_SetsStore(t *testing.T) {
	var capturedID string
	handler := middleware.RequestID(func(ctx *bast.Ctx) bast.Response {
		id, ok := bast.Get[string](ctx, "requestID")
		if !ok {
			t.Error("requestID not found in store")
		}
		capturedID = id
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx()
	handler(ctx)

	if capturedID == "" {
		t.Error("requestID was empty string")
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	var ids []string
	handler := middleware.RequestID(func(ctx *bast.Ctx) bast.Response {
		id, _ := bast.Get[string](ctx, "requestID")
		ids = append(ids, id)
		return ctx.OK(nil)
	})

	for range 5 {
		handler(basttest.NewCtx())
	}

	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate requestID: %q", id)
		}
		seen[id] = true
	}
}

// --- Recover ---

func TestRecover_CatchesPanic(t *testing.T) {
	handler := middleware.Recover(func(ctx *bast.Ctx) bast.Response {
		panic("something went wrong")
	})

	ctx := basttest.NewCtx()
	resp := handler(ctx)

	if resp.Status() != 500 {
		t.Errorf("status = %d, want 500", resp.Status())
	}
}

func TestRecover_PassthroughOnNoPanic(t *testing.T) {
	handler := middleware.Recover(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(map[string]string{"ok": "true"})
	})

	ctx := basttest.NewCtx()
	resp := handler(ctx)

	if resp.Status() != 200 {
		t.Errorf("status = %d, want 200", resp.Status())
	}
}

// --- Logger ---

func TestLogger_PassesThrough(t *testing.T) {
	handler := middleware.Logger(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx()
	resp := handler(ctx)

	if resp.Status() != 200 {
		t.Errorf("status = %d, want 200", resp.Status())
	}
}

// --- CORS ---

func TestCORS_AllowedOrigin(t *testing.T) {
	handler := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx(basttest.WithHeader("Origin", "https://example.com"))
	resp := handler(ctx)

	if resp.Headers()["Access-Control-Allow-Origin"] != "https://example.com" {
		t.Errorf("ACAO header = %q, want https://example.com",
			resp.Headers()["Access-Control-Allow-Origin"])
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	handler := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"*"},
	})(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx(basttest.WithHeader("Origin", "https://anything.com"))
	resp := handler(ctx)

	if resp.Headers()["Access-Control-Allow-Origin"] != "*" {
		t.Errorf("ACAO header = %q, want *", resp.Headers()["Access-Control-Allow-Origin"])
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	handler := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx(basttest.WithHeader("Origin", "https://evil.com"))
	resp := handler(ctx)

	if v := resp.Headers()["Access-Control-Allow-Origin"]; v != "" {
		t.Errorf("ACAO header should be empty for disallowed origin, got %q", v)
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})(func(ctx *bast.Ctx) bast.Response {
		return ctx.OK(nil)
	})

	ctx := basttest.NewCtx(
		basttest.WithMethod("OPTIONS"),
		basttest.WithHeader("Origin", "https://example.com"),
		basttest.WithHeader("Access-Control-Request-Method", "POST"),
	)
	resp := handler(ctx)

	if resp.Status() != 204 {
		t.Errorf("preflight status = %d, want 204", resp.Status())
	}
	if !strings.Contains(resp.Headers()["Access-Control-Allow-Methods"], "POST") {
		t.Errorf("ACAM header missing POST: %q", resp.Headers()["Access-Control-Allow-Methods"])
	}
}