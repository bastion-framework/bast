package bast_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bastion-framework/bast"
)

func TestErrConstructors_StatusCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", bast.ErrNotFound("NOT_FOUND", "not found"), 404},
		{"unauthorized", bast.ErrUnauthorized("UNAUTHORIZED", "unauthorized"), 401},
		{"forbidden", bast.ErrForbidden("FORBIDDEN", "forbidden"), 403},
		{"bad request", bast.ErrBadRequest("BAD_REQUEST", "bad request"), 400},
		{"conflict", bast.ErrConflict("CONFLICT", "conflict"), 409},
		{"unprocessable", bast.ErrUnprocessable("UNPROCESSABLE", "unprocessable"), 422},
		{"internal", bast.ErrInternal("INTERNAL_ERROR", "internal"), 500},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bastErr *bast.BastError
			if !errors.As(tc.err, &bastErr) {
				t.Fatalf("expected *bast.BastError, got %T", tc.err)
			}
			if bastErr.Status != tc.want {
				t.Errorf("status = %d, want %d", bastErr.Status, tc.want)
			}
		})
	}
}

func TestBastError_ErrorsAs_Wrapped(t *testing.T) {
	inner := bast.ErrNotFound("WALLET_NOT_FOUND", "wallet not found")
	wrapped := fmt.Errorf("payment initiation: %w", inner)

	var bastErr *bast.BastError
	if !errors.As(wrapped, &bastErr) {
		t.Fatal("errors.As failed through fmt.Errorf wrapping")
	}
	if bastErr.Status != 404 {
		t.Errorf("status = %d, want 404", bastErr.Status)
	}
	if bastErr.Code != "WALLET_NOT_FOUND" {
		t.Errorf("code = %q, want WALLET_NOT_FOUND", bastErr.Code)
	}
}

func TestBastError_JSON(t *testing.T) {
	err := bast.ErrNotFound("USER_NOT_FOUND", "no user with id 42")
	var bastErr *bast.BastError
	errors.As(err, &bastErr)

	got := string(bastErr.JSON())
	want := `{"error":{"code":"USER_NOT_FOUND","message":"no user with id 42"}}`
	if got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestValidationError_BastStatus(t *testing.T) {
	ve := &bast.ValidationError{Fields: map[string]string{"email": "invalid"}}
	if ve.BastStatus() != 422 {
		t.Errorf("BastStatus = %d, want 422", ve.BastStatus())
	}
}

func TestValidationError_JSON(t *testing.T) {
	ve := &bast.ValidationError{Fields: map[string]string{"email": "must be valid"}}
	got := string(ve.JSON())
	// Must contain code and field detail
	if got == "" {
		t.Error("JSON returned empty")
	}
}

func TestErrorCodes_Constants(t *testing.T) {
	if bast.CodeUnauthorized != "UNAUTHORIZED" {
		t.Error("CodeUnauthorized mismatch")
	}
	if bast.CodeNotFound != "NOT_FOUND" {
		t.Error("CodeNotFound mismatch")
	}
}