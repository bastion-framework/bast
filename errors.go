package bast

import (
	"encoding/json"
	"fmt"
)

// ── Error codes ───────────────────────────────────────────────────────────────
// Machine-readable codes included in every error response.
// Use these constants instead of raw strings to avoid typos.

const (
	// 4xx — client errors
	CodeBadRequest        = "BAD_REQUEST"         // 400 generic bad request
	CodeInvalidBody       = "INVALID_BODY"         // 400 malformed JSON / unreadable body
	CodeUnauthorized      = "UNAUTHORIZED"         // 401
	CodeForbidden         = "FORBIDDEN"            // 403
	CodeNotFound          = "NOT_FOUND"            // 404
	CodeMethodNotAllowed  = "METHOD_NOT_ALLOWED"   // 405
	CodeTimeout           = "REQUEST_TIMEOUT"      // 408
	CodeConflict          = "CONFLICT"             // 409
	CodePayloadTooLarge   = "PAYLOAD_TOO_LARGE"    // 413
	CodeUnprocessable     = "UNPROCESSABLE_ENTITY" // 422
	CodeValidation        = "VALIDATION_FAILED"    // 422 field-level validation
	CodeTooManyRequests   = "TOO_MANY_REQUESTS"    // 429

	// 5xx — server errors
	CodeInternal          = "INTERNAL_ERROR"       // 500
	CodeNotImplemented    = "NOT_IMPLEMENTED"      // 501
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE" // 503
	CodeGatewayTimeout    = "GATEWAY_TIMEOUT"      // 504
)

// ── BastError ─────────────────────────────────────────────────────────────────

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

// ── ValidationError ───────────────────────────────────────────────────────────

// ValidationError is returned by ctx.Bind() when struct validation fails.
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

// ── Error constructors ────────────────────────────────────────────────────────
// All constructors take (code, message string).
// Use the Code* constants for the code argument to avoid typos.

// ErrBadRequest returns a 400 — generic malformed request.
func ErrBadRequest(code, message string) error {
	return &BastError{Status: 400, Code: code, Message: message}
}

// ErrInvalidBody returns a 400 — request body could not be parsed.
func ErrInvalidBody(message string) error {
	return &BastError{Status: 400, Code: CodeInvalidBody, Message: message}
}

// ErrUnauthorized returns a 401 — missing or invalid credentials.
func ErrUnauthorized(code, message string) error {
	return &BastError{Status: 401, Code: code, Message: message}
}

// ErrForbidden returns a 403 — authenticated but not permitted.
func ErrForbidden(code, message string) error {
	return &BastError{Status: 403, Code: code, Message: message}
}

// ErrNotFound returns a 404 — resource does not exist.
func ErrNotFound(code, message string) error {
	return &BastError{Status: 404, Code: code, Message: message}
}

// ErrMethodNotAllowed returns a 405 — HTTP method not supported for this route.
func ErrMethodNotAllowed(message string) error {
	return &BastError{Status: 405, Code: CodeMethodNotAllowed, Message: message}
}

// ErrTimeout returns a 408 — request took too long.
func ErrTimeout(message string) error {
	return &BastError{Status: 408, Code: CodeTimeout, Message: message}
}

// ErrConflict returns a 409 — state conflict (e.g. duplicate resource).
func ErrConflict(code, message string) error {
	return &BastError{Status: 409, Code: code, Message: message}
}

// ErrPayloadTooLarge returns a 413 — request body exceeds the size limit.
func ErrPayloadTooLarge(message string) error {
	return &BastError{Status: 413, Code: CodePayloadTooLarge, Message: message}
}

// ErrUnprocessable returns a 422 — request is well-formed but semantically invalid.
func ErrUnprocessable(code, message string) error {
	return &BastError{Status: 422, Code: code, Message: message}
}

// ErrTooManyRequests returns a 429 — client has exceeded its rate limit.
func ErrTooManyRequests(message string) error {
	return &BastError{Status: 429, Code: CodeTooManyRequests, Message: message}
}

// ErrInternal returns a 500 — server fault; never leaks internals to the client.
func ErrInternal(code, message string) error {
	return &BastError{Status: 500, Code: code, Message: message}
}

// ErrNotImplemented returns a 501 — endpoint exists but is not yet implemented.
func ErrNotImplemented(message string) error {
	return &BastError{Status: 501, Code: CodeNotImplemented, Message: message}
}

// ErrServiceUnavailable returns a 503 — server is temporarily unable to handle requests.
func ErrServiceUnavailable(message string) error {
	return &BastError{Status: 503, Code: CodeServiceUnavailable, Message: message}
}

// ErrGatewayTimeout returns a 504 — upstream dependency did not respond in time.
func ErrGatewayTimeout(message string) error {
	return &BastError{Status: 504, Code: CodeGatewayTimeout, Message: message}
}