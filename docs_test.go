package bast_test

import (
	"encoding/json"
	"testing"

	"github.com/bastion-framework/bast"
)

// testDocsApp builds an app with docs enabled and a simple module registered.
func testDocsApp() *bast.App {
	type UserResponse struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type CreateUserRequest struct {
		Name  string `json:"name"  validate:"required,min=2"`
		Email string `json:"email" validate:"required,email"`
	}

	ctrl := &docsTestController{}
	mod := bast.Module{
		Prefix: "/users",
		Doc: bast.ModuleDoc{
			Name:        "Users",
			Description: "User management",
		},
		Controller: ctrl,
	}

	app := bast.New(bast.Config{
		Docs: &bast.DocsConfig{
			Enabled:     true,
			Path:        "/docs",
			JSONPath:    "/openapi.json",
			Title:       "Bast Test API",
			Version:     "1.0.0",
			Description: "Test API built with Bast",
		},
	})
	app.Register(mod)
	return app
}

type docsTestController struct{}

func (c *docsTestController) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/:id", c.GetUser, bast.WithDoc(bast.Doc{
			Summary:     "Get a user by ID",
			Description: "Returns a single user.",
			Tags:        []string{"users"},
			Params:      []bast.Param{bast.PathParam("id", "User UUID")},
		})),
		bast.POST("/", c.CreateUser, bast.WithDoc(bast.Doc{
			Summary: "Create a new user",
			Tags:    []string{"users"},
		})),
		bast.DELETE("/:id", c.DeleteUser, bast.WithDoc(bast.Doc{
			Summary:       "Delete a user",
			Deprecated:    true,
			DeprecatedMsg: "Use v2 instead",
		})),
	}
}

func (c *docsTestController) GetUser(ctx *bast.Ctx) bast.Response    { return ctx.OK(nil) }
func (c *docsTestController) CreateUser(ctx *bast.Ctx) bast.Response  { return ctx.Created(nil) }
func (c *docsTestController) DeleteUser(ctx *bast.Ctx) bast.Response  { return ctx.NoContent() }

func TestDocs_SpecContainsPaths(t *testing.T) {
	app := testDocsApp()
	spec := app.OpenAPISpec()

	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	for _, want := range []string{
		"/users/{id}",
		"/users/",
		"get",
		"post",
		"delete",
	} {
		if !containsStr(string(raw), want) {
			t.Errorf("spec missing %q\nspec: %s", want, raw)
		}
	}
}

func TestDocs_SpecInfo(t *testing.T) {
	app := testDocsApp()
	spec := app.OpenAPISpec()

	info, ok := spec["info"].(map[string]any)
	if !ok {
		t.Fatal("spec missing 'info' field")
	}
	if info["title"] != "Bast Test API" {
		t.Errorf("title = %v, want Bast Test API", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", info["version"])
	}
}

func TestDocs_SpecOpenAPIVersion(t *testing.T) {
	app := testDocsApp()
	spec := app.OpenAPISpec()

	if spec["openapi"] != "3.0.3" {
		t.Errorf("openapi = %v, want 3.0.3", spec["openapi"])
	}
}

func TestDocs_TagGroups(t *testing.T) {
	app := testDocsApp()
	spec := app.OpenAPISpec()

	raw, _ := json.Marshal(spec)
	if !containsStr(string(raw), "Users") {
		t.Errorf("spec missing module tag group 'Users'\nspec: %s", raw)
	}
}

func TestDocs_DeprecatedRoute(t *testing.T) {
	app := testDocsApp()
	spec := app.OpenAPISpec()

	raw, _ := json.Marshal(spec)
	if !containsStr(string(raw), "deprecated") {
		t.Errorf("spec should mark deprecated routes\nspec: %s", raw)
	}
}

func TestDocs_PathParamConversion(t *testing.T) {
	app := testDocsApp()
	spec := app.OpenAPISpec()

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec missing 'paths'")
	}
	if _, ok := paths["/users/{id}"]; !ok {
		t.Errorf("expected path /users/{id}, got keys: %v", pathKeys(paths))
	}
}

func pathKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestDocs_NilWhenDisabled(t *testing.T) {
	app := bast.New(bast.Config{}) // no Docs config
	spec := app.OpenAPISpec()
	if spec != nil {
		t.Error("OpenAPISpec should be nil when docs not configured")
	}
}