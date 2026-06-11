package bast

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// failWriter is an http.ResponseWriter whose Write always fails.
type failWriter struct{ header http.Header }

func (f *failWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}
func (f *failWriter) WriteHeader(int)         {}
func (f *failWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestStreamCtx_Flush_ReturnsErrorOnWriterFailure(t *testing.T) {
	fw := &failWriter{}
	req := httptest.NewRequest("GET", "/", nil)
	sctx := newStreamCtx(req.Context(), fw, req)

	// Put data in the buffer so bw.Flush() actually writes to the underlying writer.
	_ = sctx.bw.WriteByte('x')

	err := sctx.Flush()
	if err == nil {
		t.Fatal("Flush should return error when underlying writer fails")
	}
	if !strings.Contains(err.Error(), "broken pipe") {
		t.Errorf("Flush error = %v, want to contain 'broken pipe'", err)
	}
}

func TestStreamCtx_Flush_ReturnsNilOnSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	sctx := newStreamCtx(req.Context(), w, req)

	if err := sctx.Flush(); err != nil {
		t.Fatalf("Flush returned unexpected error: %v", err)
	}
}
