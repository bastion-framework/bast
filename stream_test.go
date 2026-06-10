package bast_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bastion-framework/bast"
)

// --- test controllers ---

type streamEventsCtrl struct{ handler bast.StreamHandlerFunc }

func (c *streamEventsCtrl) Routes() []bast.Route {
	return []bast.Route{bast.STREAM("/events", c.handler)}
}

type streamParamCtrl struct{ handler bast.StreamHandlerFunc }

func (c *streamParamCtrl) Routes() []bast.Route {
	return []bast.Route{bast.STREAM("/:id/events", c.handler)}
}

type streamGuardedCtrl struct {
	handler bast.StreamHandlerFunc
	guards  []bast.Guard
}

func (c *streamGuardedCtrl) Routes() []bast.Route {
	return []bast.Route{
		bast.STREAM("/events", c.handler, bast.WithGuards(c.guards...)),
	}
}

// --- Guard execution ---

func TestStream_GlobalGuard_Blocks(t *testing.T) {
	called := false
	blockGuard := bast.GuardFunc(func(ctx *bast.Ctx) error {
		return bast.ErrUnauthorized(bast.CodeUnauthorized, "not allowed")
	})

	app := bast.New(bast.Config{})
	app.Guard(blockGuard)
	app.Register(bast.Module{
		Prefix: "/api",
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) { called = true },
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if called {
		t.Error("stream handler must not be called when guard blocks")
	}
}

func TestStream_ModuleGuard_Blocks(t *testing.T) {
	called := false
	blockGuard := bast.GuardFunc(func(ctx *bast.Ctx) error {
		return bast.ErrForbidden(bast.CodeForbidden, "forbidden")
	})

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Guards: []bast.Guard{blockGuard},
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) { called = true },
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if called {
		t.Error("stream handler must not be called when module guard blocks")
	}
}

func TestStream_RouteGuard_Blocks(t *testing.T) {
	called := false
	blockGuard := bast.GuardFunc(func(ctx *bast.Ctx) error {
		return bast.ErrForbidden(bast.CodeForbidden, "route blocked")
	})

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Controller: &streamGuardedCtrl{
			handler: func(sctx *bast.StreamCtx) { called = true },
			guards:  []bast.Guard{blockGuard},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if called {
		t.Error("stream handler must not be called when route guard blocks")
	}
}

func TestStream_MultipleGuards_FirstFailureShortCircuits(t *testing.T) {
	secondCalled := false
	first := bast.GuardFunc(func(ctx *bast.Ctx) error {
		return bast.ErrUnauthorized(bast.CodeUnauthorized, "first blocked")
	})
	second := bast.GuardFunc(func(ctx *bast.Ctx) error {
		secondCalled = true
		return nil
	})

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix:  "/api",
		Guards:  []bast.Guard{first, second},
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) {},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if secondCalled {
		t.Error("second guard must not run after first guard fails")
	}
}

func TestStream_GuardAllows_HandlerRuns(t *testing.T) {
	called := false
	allowGuard := bast.GuardFunc(func(ctx *bast.Ctx) error { return nil })

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Guards: []bast.Guard{allowGuard},
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) { called = true },
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if !called {
		t.Error("stream handler should be called when guard allows")
	}
}

// --- Store transfer ---

func TestStream_GuardStore_TransferredToStreamCtx(t *testing.T) {
	var gotValue string
	var gotOK bool

	setGuard := bast.GuardFunc(func(ctx *bast.Ctx) error {
		ctx.Set("userID", "user-42")
		return nil
	})

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Guards: []bast.Guard{setGuard},
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) {
				v, ok := sctx.Get("userID")
				gotOK = ok
				if s, ok2 := v.(string); ok2 {
					gotValue = s
				}
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if !gotOK {
		t.Error("sctx.Get(\"userID\") should return ok=true")
	}
	if gotValue != "user-42" {
		t.Errorf("userID = %q, want %q", gotValue, "user-42")
	}
}

func TestStream_GuardStore_MustGet(t *testing.T) {
	setGuard := bast.GuardFunc(func(ctx *bast.Ctx) error {
		ctx.Set("role", "admin")
		return nil
	})

	var gotRole string
	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Guards: []bast.Guard{setGuard},
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) {
				gotRole = sctx.MustGet("role").(string)
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if gotRole != "admin" {
		t.Errorf("role = %q, want \"admin\"", gotRole)
	}
}

// --- Path params (Option B) ---

func TestStream_PathParam_Accessible(t *testing.T) {
	var gotID string

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Controller: &streamParamCtrl{
			handler: func(sctx *bast.StreamCtx) {
				gotID = sctx.Param("id")
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user-99/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if gotID != "user-99" {
		t.Errorf("param id = %q, want \"user-99\"", gotID)
	}
}

func TestStream_PathParam_MissingKeyReturnsEmpty(t *testing.T) {
	var gotVal string

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) {
				gotVal = sctx.Param("nonexistent")
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if gotVal != "" {
		t.Errorf("missing param = %q, want empty string", gotVal)
	}
}

// --- StreamCtx store unit tests ---

func TestStreamCtx_Set_Get(t *testing.T) {
	sctx := bast.NewTestStreamCtx()
	sctx.Set("key", "value")

	v, ok := sctx.Get("key")
	if !ok {
		t.Fatal("Get should return ok=true for a set key")
	}
	if v.(string) != "value" {
		t.Errorf("Get = %v, want \"value\"", v)
	}
}

func TestStreamCtx_Get_Missing(t *testing.T) {
	sctx := bast.NewTestStreamCtx()
	_, ok := sctx.Get("missing")
	if ok {
		t.Error("Get should return ok=false for a missing key")
	}
}

func TestStreamCtx_MustGet_Panics_OnMissing(t *testing.T) {
	sctx := bast.NewTestStreamCtx()
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet should panic on missing key")
		}
	}()
	sctx.MustGet("nonexistent")
}

// --- StreamCtx param unit tests ---

func TestStreamCtx_Param(t *testing.T) {
	sctx := bast.NewTestStreamCtx()
	bast.SetStreamTestParam(sctx, "id", "42")

	if got := sctx.Param("id"); got != "42" {
		t.Errorf("Param(\"id\") = %q, want \"42\"", got)
	}
	if got := sctx.Param("missing"); got != "" {
		t.Errorf("Param(\"missing\") = %q, want empty", got)
	}
}

// --- Guard header access (guards can read request headers) ---

func TestStream_Guard_CanReadRequestHeaders(t *testing.T) {
	var gotToken string
	checkGuard := bast.GuardFunc(func(ctx *bast.Ctx) error {
		gotToken = ctx.Header("Authorization")
		return nil
	})

	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/api",
		Guards: []bast.Guard{checkGuard},
		Controller: &streamEventsCtrl{
			handler: func(sctx *bast.StreamCtx) {},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Authorization", "Bearer tok123")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if gotToken != "Bearer tok123" {
		t.Errorf("guard saw Authorization = %q, want \"Bearer tok123\"", gotToken)
	}
}
