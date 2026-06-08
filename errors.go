package bast

import (
	"encoding/json"
	"fmt"
)

// Error code constants for common cases — avoids string typos.
const (
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeValidation   = "VALIDATION_FAILED"
	CodeConflict     = "CONFLICT"
	CodeInternal     = "INTERNAL_ERROR"
)

// BastError is the typed error Bast understands.
// Use the constructors — never instantiate directly.
type BastError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *BastError) Error() string {
	return fmt.Sprintf("bast: %s: %s", e.Code, e.Message)
}

// JSON serializes BastError into the standard error envelope.
func (e *BastError) JSON() []byte {
	b, _ := json.Marshal(struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: e.Code, Message: e.Message}})
	return b
}

// ValidationError is returned by ctx.Bind() when validation fails.
// The error boundary detects it and produces field-level 422 responses automatically.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "bast: validation failed"
}

// BastStatus satisfies a convention checked by the error boundary.
func (e *ValidationError) BastStatus() int { return 422 }

// JSON serializes ValidationError into the validation error envelope.
func (e *ValidationError) JSON() []byte {
	type fieldError struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	}
	b, _ := json.Marshal(struct {
		Error fieldError `json:"error"`
	}{Error: fieldError{
		Code:    CodeValidation,
		Message: "request validation failed",
		Fields:  e.Fields,
	}})
	return b
}

// Error constructors — all take (code, message string).

// ErrNotFound returns a 404 BastError.
func ErrNotFound(code, message string) error {
	return &BastError{Status: 404, Code: code, Message: message}
}

// ErrUnauthorized returns a 401 BastError.
func ErrUnauthorized(code, message string) error {
	return &BastError{Status: 401, Code: code, Message: message}
}

// ErrForbidden returns a 403 BastError.
func ErrForbidden(code, message string) error {
	return &BastError{Status: 403, Code: code, Message: message}
}

// ErrBadRequest returns a 400 BastError.
func ErrBadRequest(code, message string) error {
	return &BastError{Status: 400, Code: code, Message: message}
}

// ErrConflict returns a 409 BastError.
func ErrConflict(code, message string) error {
	return &BastError{Status: 409, Code: code, Message: message}
}

// ErrUnprocessable returns a 422 BastError.
func ErrUnprocessable(code, message string) error {
	return &BastError{Status: 422, Code: code, Message: message}
}

// ErrInternal returns a 500 BastError. Never leaks internals to the client.
func ErrInternal(code, message string) error {
	return &BastError{Status: 500, Code: code, Message: message}
}