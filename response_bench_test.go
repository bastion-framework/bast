package bast_test

import (
	"net/http/httptest"
	"testing"

	"github.com/bastion-framework/bast"
)

// --- Correctness tests for response envelope ---

func TestCtxOK_Nil_Envelope(t *testing.T) {
	ctx := bast.NewTestCtxWithPath("/")
	resp := ctx.OK(nil)
	got := string(resp.Body())
	want := `{"data":null,"meta":null}`
	if got != want {
		t.Errorf("OK(nil) body = %q, want %q", got, want)
	}
	if resp.Status() != 200 {
		t.Errorf("OK(nil) status = %d, want 200", resp.Status())
	}
}

func TestCtxOK_Data_Envelope(t *testing.T) {
	ctx := bast.NewTestCtxWithPath("/")
	resp := ctx.OK(map[string]string{"id": "42"})
	got := string(resp.Body())
	if got == "" {
		t.Fatal("OK(data) produced empty body")
	}
	if resp.ContentType() != "application/json" {
		t.Errorf("content-type = %q, want application/json", resp.ContentType())
	}
}

func TestCtxCreated_Nil_Envelope(t *testing.T) {
	ctx := bast.NewTestCtxWithPath("/")
	resp := ctx.Created(nil)
	got := string(resp.Body())
	want := `{"data":null,"meta":null}`
	if got != want {
		t.Errorf("Created(nil) body = %q, want %q", got, want)
	}
	if resp.Status() != 201 {
		t.Errorf("Created(nil) status = %d, want 201", resp.Status())
	}
}

func TestResponse_Headers_NilOnFreshResponse(t *testing.T) {
	r := bast.NewRawResponse(200, "application/json", nil)
	if h := r.Headers(); h != nil {
		t.Errorf("fresh response Headers() = %v, want nil", h)
	}
}

func TestResponse_WithHeader_BuildsCorrectMap(t *testing.T) {
	r := bast.NewRawResponse(200, "", nil)
	r = r.WithHeader("X-Trace", "abc").WithHeader("X-Request-Id", "xyz")

	h := r.Headers()
	if h["X-Trace"] != "abc" {
		t.Errorf("X-Trace = %q, want abc", h["X-Trace"])
	}
	if h["X-Request-Id"] != "xyz" {
		t.Errorf("X-Request-Id = %q, want xyz", h["X-Request-Id"])
	}
}

// --- Micro-benchmarks ---

// BenchmarkResponse_NewRaw measures raw response construction.
// Target after Fix 1+4: 0 allocs/op.
func BenchmarkResponse_NewRaw(b *testing.B) {
	body := []byte(`{"data":null,"meta":null}`)
	b.ReportAllocs()
	for b.Loop() {
		_ = bast.NewRawResponse(200, "application/json", body)
	}
}

// BenchmarkResponse_OK_Nil measures ctx.OK(nil).
// Target after Fix 2+1+4: 0 allocs/op (pre-baked envelope, no map init).
func BenchmarkResponse_OK_Nil(b *testing.B) {
	ctx := bast.NewTestCtxWithPath("/")
	b.ReportAllocs()
	for b.Loop() {
		_ = ctx.OK(nil)
	}
}

// BenchmarkResponse_OK_Data measures ctx.OK(data) — JSON marshal path.
// Target after Fix 3+1+4: reduced allocs vs baseline.
func BenchmarkResponse_OK_Data(b *testing.B) {
	ctx := bast.NewTestCtxWithPath("/")
	data := map[string]string{"id": "42", "name": "kasim"}
	b.ReportAllocs()
	for b.Loop() {
		_ = ctx.OK(data)
	}
}

// BenchmarkResponse_WithHeader measures the WithHeader copy path.
// Target after Fix 4: 1 alloc/op (one slice allocation, no map).
func BenchmarkResponse_WithHeader(b *testing.B) {
	base := bast.NewRawResponse(200, "application/json", nil)
	b.ReportAllocs()
	for b.Loop() {
		_ = base.WithHeader("X-Request-Id", "bench-id-12345")
	}
}

// BenchmarkResponse_Raw measures ctx.Raw (no marshal).
// Target after Fix 1+4: 0 allocs/op (no map, no body alloc).
func BenchmarkResponse_Raw(b *testing.B) {
	ctx := bast.NewTestCtxWithPath("/")
	body := []byte(`{"data":null,"meta":null}`)
	b.ReportAllocs()
	for b.Loop() {
		_ = ctx.Raw(200, "application/json", body)
	}
}

// BenchmarkApp_WriteResponse exercises the full ServeHTTP path with ctx.OK(nil).
// This is the production hot path benchmark.
func BenchmarkApp_WriteResponse_OKNil(b *testing.B) {
	app := bast.New(bast.Config{})
	app.Register(bast.Module{
		Prefix: "/bench",
		Controller: &benchOKNilController{},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/bench/ping", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		w.Body.Reset()
		app.ServeHTTP(w, r)
	}
}

type benchOKNilController struct{}

func (c *benchOKNilController) Routes() []bast.Route {
	return []bast.Route{
		bast.GET("/ping", func(ctx *bast.Ctx) bast.Response {
			return ctx.OK(nil)
		}),
	}
}
