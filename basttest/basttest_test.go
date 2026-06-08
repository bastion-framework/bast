package basttest_test

import (
	"testing"

	"github.com/bastion-framework/bast"
	"github.com/bastion-framework/bast/basttest"
)

// --- Unit tests via NewCtx ---

func TestNewCtx_Defaults(t *testing.T) {
	ctx := basttest.NewCtx()
	if ctx.Method() != "GET" {
		t.Errorf("default method = %q, want GET", ctx.Method())
	}
	if ctx.Path() != "/" {
		t.Errorf("default path = %q, want /", ctx.Path())
	}
}

func TestNewCtx_WithParam(t *testing.T) {
	ctx := basttest.NewCtx(
		basttest.WithParam("id", "42"),
		basttest.WithParam("org", "bast"),
	)
	if ctx.Param("id") != "42" {
		t.Errorf("param id = %q, want 42", ctx.Param("id"))
	}
	if ctx.Param("org") != "bast" {
		t.Errorf("param org = %q, want bast", ctx.Param("org"))
	}
}

func TestNewCtx_WithQuery(t *testing.T) {
	ctx := basttest.NewCtx(basttest.WithQuery("page", "3"))
	if ctx.Query("page") != "3" {
		t.Errorf("query page = %q, want 3", ctx.Query("page"))
	}
}

func TestNewCtx_WithHeader(t *testing.T) {
	ctx := basttest.NewCtx(basttest.WithHeader("Authorization", "Bearer tok"))
	if ctx.Header("Authorization") != "Bearer tok" {
		t.Errorf("header Authorization = %q, want Bearer tok", ctx.Header("Authorization"))
	}
}

func TestNewCtx_WithBody(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	ctx := basttest.NewCtx(basttest.WithBody(payload{Name: "kasim"}))
	var got payload
	if err := ctx.BindJSON(&got); err != nil {
		t.Fatalf("BindJSON: %v", err)
	}
	if got.Name != "kasim" {
		t.Errorf("name = %q, want kasim", got.Name)
	}
}

func TestNewCtx_WithStore(t *testing.T) {
	type Claims struct{ UserID string }
	ctx := basttest.NewCtx(basttest.WithStore("claims", &Claims{UserID: "u1"}))

	claims, ok := bast.Get[*Claims](ctx, "claims")
	if !ok {
		t.Fatal("claims not found in store")
	}
	if claims.UserID != "u1" {
		t.Errorf("userID = %q, want u1", claims.UserID)
	}
}

func TestNewCtx_WithMethod(t *testing.T) {
	ctx := basttest.NewCtx(basttest.WithMethod("POST"))
	if ctx.Method() != "POST" {
		t.Errorf("method = %q, want POST", ctx.Method())
	}
}

func TestNewCtx_WithPath(t *testing.T) {
	ctx := basttest.NewCtx(basttest.WithPath("/users/42"))
	if ctx.Path() != "/users/42" {
		t.Errorf("path = %q, want /users/42", ctx.Path())
	}
}

func TestNewCtx_HandlerUnitTest(t *testing.T) {
	// Simulates a unit test of a handler with a mocked store value.
	type Claims struct{ Role string }

	handler := func(ctx *bast.Ctx) bast.Response {
		claims, ok := bast.Get[*Claims](ctx, "claims")
		if !ok {
			return ctx.Error(bast.ErrForbidden(bast.CodeForbidden, "no claims"))
		}
		if claims.Role != "admin" {
			return ctx.Error(bast.ErrForbidden(bast.CodeForbidden, "insufficient role"))
		}
		return ctx.OK(map[string]string{"status": "ok"})
	}

	t.Run("admin passes", func(t *testing.T) {
		ctx := basttest.NewCtx(basttest.WithStore("claims", &Claims{Role: "admin"}))
		resp := handler(ctx)
		if resp.Status() != 200 {
			t.Errorf("status = %d, want 200", resp.Status())
		}
	})

	t.Run("non-admin blocked", func(t *testing.T) {
		ctx := basttest.NewCtx(basttest.WithStore("claims", &Claims{Role: "user"}))
		resp := handler(ctx)
		if !resp.IsError() {
			t.Error("expected error response")
		}
	})
}

// --- Integration tests via NewApp ---

func makeEchoModule() bast.Module {
	ctrl := &echoController{}
	return bast.Module{
		Prefix:     "/echo",
		Controller: ctrl,
	}
}

type echoController struct{}

func (c *echoController) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/:msg", c.Echo),
		bast.POST("/body", c.Body),
	}
}

func (c *echoController) Echo(ctx *bast.Ctx) bast.Response {
	return ctx.OK(map[string]string{"msg": ctx.Param("msg")})
}

func (c *echoController) Body(ctx *bast.Ctx) bast.Response {
	var payload map[string]string
	if err := ctx.BindJSON(&payload); err != nil {
		return ctx.Error(bast.ErrBadRequest(bast.CodeValidation, "bad body"))
	}
	return ctx.Created(payload)
}

func TestNewApp_GetRoute(t *testing.T) {
	app := basttest.NewApp(makeEchoModule())
	defer app.Close()

	app.Do("GET", "/echo/hello").
		Assert(t).
		StatusIs(200).
		BodyContains("hello")
}

func TestNewApp_PostRoute(t *testing.T) {
	app := basttest.NewApp(makeEchoModule())
	defer app.Close()

	app.Do("POST", "/echo/body",
		basttest.WithJSONBody(map[string]string{"key": "val"}),
	).Assert(t).
		StatusIs(201).
		BodyContains("val")
}

func TestNewApp_NotFound(t *testing.T) {
	app := basttest.NewApp(makeEchoModule())
	defer app.Close()

	app.Do("GET", "/missing").Assert(t).StatusIs(404)
}

func TestNewApp_MethodNotAllowed(t *testing.T) {
	app := basttest.NewApp(makeEchoModule())
	defer app.Close()

	app.Do("DELETE", "/echo/hello").Assert(t).StatusIs(405)
}