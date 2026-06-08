package bast_test

import (
	"net/http"
	"testing"

	"github.com/bastion-framework/bast"
)

func makeTestResponse(status int, body []byte, ct string) bast.Response {
	return bast.NewRawResponse(status, ct, body)
}

func TestResponse_WithHeader_Immutable(t *testing.T) {
	r1 := makeTestResponse(200, []byte("ok"), "application/json")
	r2 := r1.WithHeader("X-Foo", "bar")

	if _, ok := r1.Headers()["X-Foo"]; ok {
		t.Error("WithHeader mutated original response")
	}
	if r2.Headers()["X-Foo"] != "bar" {
		t.Error("WithHeader did not set header on new response")
	}
}

func TestResponse_WithStatus_Immutable(t *testing.T) {
	r1 := makeTestResponse(200, nil, "")
	r2 := r1.WithStatus(404)

	if r1.Status() != 200 {
		t.Error("WithStatus mutated original response")
	}
	if r2.Status() != 404 {
		t.Error("WithStatus did not change status on new response")
	}
}

func TestResponse_WithCookie_Immutable(t *testing.T) {
	r1 := makeTestResponse(200, nil, "")
	cookie := &http.Cookie{Name: "session", Value: "abc"}
	r2 := r1.WithCookie(cookie)

	if len(r1.Cookies()) != 0 {
		t.Error("WithCookie mutated original response")
	}
	if len(r2.Cookies()) != 1 || r2.Cookies()[0].Name != "session" {
		t.Error("WithCookie did not attach cookie to new response")
	}
}

func TestResponse_IsError(t *testing.T) {
	r := bast.NewErrorResponse(bast.ErrNotFound("NF", "not found"))
	if !r.IsError() {
		t.Error("expected IsError to be true")
	}
	if r.Err() == nil {
		t.Error("expected Err() to be non-nil")
	}
}

func TestResponse_WithHeader_Multiple(t *testing.T) {
	r := makeTestResponse(200, nil, "")
	r = r.WithHeader("X-A", "1").WithHeader("X-B", "2")

	if r.Headers()["X-A"] != "1" {
		t.Error("X-A missing")
	}
	if r.Headers()["X-B"] != "2" {
		t.Error("X-B missing")
	}
}
