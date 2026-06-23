package bast

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/bastion-framework/bast/internal/jsonx"
)

// DocsConfig configures the built-in OpenAPI documentation endpoints.
type DocsConfig struct {
	Enabled     bool
	Path        string // Swagger UI path, e.g. "/docs"
	JSONPath    string // Raw OpenAPI JSON path, e.g. "/openapi.json"
	Title       string
	Version     string
	Description string
}

// routeEntry holds a registered route together with its full path and doc for spec building.
type routeEntry struct {
	method  string
	path    string // full path with prefix, OpenAPI format ({id} not :id)
	doc     Doc
	guards  []Guard
	stream  bool
}

// specBuilder accumulates routes and module metadata during registration.
type specBuilder struct {
	routes  []routeEntry
	tags    []map[string]any          // OpenAPI tag objects
	schemes map[string]map[string]any // securityScheme name → definition
}

func newSpecBuilder() *specBuilder {
	return &specBuilder{
		schemes: make(map[string]map[string]any),
	}
}

// addRoute records a route for the spec.
func (b *specBuilder) addRoute(method, fullPattern string, r Route, modDoc ModuleDoc, guards []Guard) {
	oaPath := toOpenAPIPath(fullPattern)
	entry := routeEntry{
		method: strings.ToLower(method),
		path:   oaPath,
		doc:    r.Doc,
		guards: guards,
		stream: r.Stream != nil,
	}
	b.routes = append(b.routes, entry)

	// Collect security schemes from SecuredGuard implementations.
	for _, g := range guards {
		if sg, ok := g.(SecuredGuard); ok {
			scheme := sg.SecurityScheme()
			name := schemeKey(scheme)
			if _, exists := b.schemes[name]; !exists {
				b.schemes[name] = securitySchemeDef(scheme)
			}
		}
	}
}

// addModuleDoc records a module tag group.
func (b *specBuilder) addModuleDoc(doc ModuleDoc) {
	if doc.Name == "" {
		return
	}
	for _, t := range b.tags {
		if t["name"] == doc.Name {
			return // already added
		}
	}
	b.tags = append(b.tags, map[string]any{
		"name":        doc.Name,
		"description": doc.Description,
	})
}

// build assembles the OpenAPI 3.0.3 spec as a generic map.
// Reflection runs here — once at startup, never on the hot path.
func (b *specBuilder) build(cfg *DocsConfig) map[string]any {
	paths := make(map[string]any)

	for _, entry := range b.routes {
		pathItem, ok := paths[entry.path].(map[string]any)
		if !ok {
			pathItem = make(map[string]any)
			paths[entry.path] = pathItem
		}

		op := buildOperation(entry)
		pathItem[entry.method] = op
	}

	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       cfg.Title,
			"version":     cfg.Version,
			"description": cfg.Description,
		},
		"paths": paths,
	}

	if len(b.tags) > 0 {
		spec["tags"] = b.tags
	}

	components := make(map[string]any)
	if len(b.schemes) > 0 {
		components["securitySchemes"] = b.schemes
	}
	if len(components) > 0 {
		spec["components"] = components
	}

	return spec
}

// buildOperation assembles an OpenAPI operation object from a route entry.
func buildOperation(entry routeEntry) map[string]any {
	op := make(map[string]any)

	if entry.doc.Summary != "" {
		op["summary"] = entry.doc.Summary
	}
	if entry.doc.Description != "" {
		op["description"] = entry.doc.Description
	}
	if len(entry.doc.Tags) > 0 {
		op["tags"] = entry.doc.Tags
	}
	if entry.doc.Deprecated {
		op["deprecated"] = true
		if entry.doc.DeprecatedMsg != "" {
			op["x-deprecated-message"] = entry.doc.DeprecatedMsg
		}
	}

	// Parameters.
	var params []any
	for _, p := range entry.doc.Params {
		params = append(params, map[string]any{
			"name":        p.Name,
			"in":          p.In,
			"description": p.Description,
			"required":    p.Required,
			"schema":      map[string]any{"type": "string"},
		})
	}
	// Auto-extract path params from the path pattern if no explicit params defined.
	if len(params) == 0 {
		for _, seg := range strings.Split(entry.path, "/") {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				name := seg[1 : len(seg)-1]
				params = append(params, map[string]any{
					"name":     name,
					"in":       "path",
					"required": true,
					"schema":   map[string]any{"type": "string"},
				})
			}
		}
	}
	if len(params) > 0 {
		op["parameters"] = params
	}

	// Request body schema.
	if entry.doc.Body != nil {
		schema := reflectSchema(reflect.TypeOf(entry.doc.Body.Example))
		op["requestBody"] = map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": schema,
				},
			},
		}
	}

	// Responses.
	responses := make(map[string]any)
	for code, body := range entry.doc.Returns {
		resp := map[string]any{
			"description": http.StatusText(code),
		}
		if body != nil {
			schema := reflectSchema(reflect.TypeOf(body.Example))
			resp["content"] = map[string]any{
				"application/json": map[string]any{
					"schema": schema,
				},
			}
		}
		responses[fmt.Sprintf("%d", code)] = resp
	}
	if len(responses) > 0 {
		op["responses"] = responses
	} else {
		op["responses"] = map[string]any{
			"200": map[string]any{"description": "OK"},
		}
	}

	// Security — from SecuredGuard guards.
	var security []any
	for _, g := range entry.guards {
		if sg, ok := g.(SecuredGuard); ok {
			scheme := sg.SecurityScheme()
			security = append(security, map[string]any{
				schemeKey(scheme): []any{},
			})
		}
	}
	if len(security) > 0 {
		op["security"] = security
	}

	return op
}

// reflectSchema generates a JSON schema map from a Go type.
// Only called at startup — reflection is permitted here.
func reflectSchema(t reflect.Type) map[string]any {
	if t == nil {
		return map[string]any{"type": "object"}
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Slice:
		return map[string]any{
			"type":  "array",
			"items": reflectSchema(t.Elem()),
		}
	case reflect.Struct:
		props := make(map[string]any)
		var required []string
		for i := range t.NumField() {
			f := t.Field(i)
			name := jsonName(f)
			if name == "-" {
				continue
			}
			props[name] = reflectSchema(f.Type)
			if strings.Contains(f.Tag.Get("validate"), "required") {
				required = append(required, name)
			}
		}
		schema := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	default:
		return map[string]any{"type": "object"}
	}
}

// jsonName returns the JSON field name from struct tags.
func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return strings.ToLower(f.Name)
	}
	if idx := strings.Index(tag, ","); idx != -1 {
		tag = tag[:idx]
	}
	if tag == "" {
		return strings.ToLower(f.Name)
	}
	return tag
}

// toOpenAPIPath converts a Bast path pattern to OpenAPI format.
// /users/:id/posts/:postID → /users/{id}/posts/{postID}
func toOpenAPIPath(pattern string) string {
	segments := strings.Split(pattern, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			segments[i] = "{" + seg[1:] + "}"
		} else if strings.HasPrefix(seg, "*") {
			segments[i] = "{" + seg[1:] + "}"
		}
	}
	return strings.Join(segments, "/")
}

func schemeKey(s SecurityScheme) string {
	if s.Scheme != "" {
		return s.Scheme + "Auth"
	}
	return s.Type + "Auth"
}

func securitySchemeDef(s SecurityScheme) map[string]any {
	def := map[string]any{
		"type":        s.Type,
		"description": s.Description,
	}
	if s.Scheme != "" {
		def["scheme"] = s.Scheme
	}
	if s.BearerFormat != "" {
		def["bearerFormat"] = s.BearerFormat
	}
	return def
}

// OpenAPISpec returns the built OpenAPI spec, or nil if docs are not configured.
// The spec is built once when this method is first called — safe to call multiple times.
func (a *App) OpenAPISpec() map[string]any {
	if a.cfg.Docs == nil || !a.cfg.Docs.Enabled {
		return nil
	}
	return a.specBuilder.build(a.cfg.Docs)
}

// registerDocsRoutes mounts the OpenAPI JSON and Swagger UI endpoints if configured.
func (a *App) registerDocsRoutes() {
	if a.cfg.Docs == nil || !a.cfg.Docs.Enabled {
		return
	}
	d := a.cfg.Docs

	if d.JSONPath != "" {
		a.router.Add("GET", d.JSONPath, HandlerFunc(func(ctx *Ctx) Response {
			spec := a.OpenAPISpec()
			b, _ := jsonx.Marshal(spec)
			return newRawResponse(200, "application/json", b)
		}))
	}

	if d.Path != "" {
		a.router.Add("GET", d.Path, HandlerFunc(func(ctx *Ctx) Response {
			html := swaggerUIHTML(d.JSONPath, d.Title)
			return newRawResponse(200, "text/html; charset=utf-8", []byte(html))
		}))
	}
}

// swaggerUIHTML returns a minimal Swagger UI page backed by the spec at specURL.
func swaggerUIHTML(specURL, title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>%s</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  SwaggerUIBundle({
    url: "%s",
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
    layout: "BaseLayout"
  })
</script>
</body>
</html>`, title, specURL)
}