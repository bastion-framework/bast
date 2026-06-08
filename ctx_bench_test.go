package bast_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bastion-framework/bast"
)

func BenchmarkCtx_AcquireRelease(b *testing.B) {
	// Pre-create request/recorder outside the loop — we are measuring only
	// the pool acquire/release, not request construction.
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/users/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		ctx := bast.BenchAcquireCtx(w, r)
		bast.BenchReleaseCtx(ctx)
	}
}

func BenchmarkCtx_ParamLookup(b *testing.B) {
	ctx := bast.NewTestCtx()
	bast.SetTestParam(ctx, "id", "42")
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = ctx.Param("id")
	}
}

func BenchmarkCtx_Bind(b *testing.B) {
	body := `{"name":"kasim","email":"k@suds.ug"}`
	b.ReportAllocs()
	for b.Loop() {
		ctx := bast.NewTestCtx()
		r, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		bast.InitTestCtx(ctx, httptest.NewRecorder(), r)
		var v struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		ctx.BindJSON(&v) //nolint
	}
}