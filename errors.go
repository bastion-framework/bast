package bast

import (
	"fmt"
	"sync"

	"github.com/bastion-framework/bast/internal/jsonx"
)

const (
	CodeBadRequest         = "BAD_REQUEST"
	CodeInvalidBody        = "INVALID_BODY"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeTimeout            = "REQUEST_TIMEOUT"
	CodeConflict           = "CONFLICT"
	CodePayloadTooLarge    = "PAYLOAD_TOO_LARGE"
	CodeUnprocessable      = "UNPROCESSABLE_ENTITY"
	CodeValidation         = "VALIDATION_FAILED"
	CodeTooManyRequests    = "TOO_MANY_REQUESTS"
	CodeInternal           = "INTERNAL_ERROR"
	CodeNotImplemented     = "NOT_IMPLEMENTED"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeGatewayTimeout     = "GATEWAY_TIMEOUT"
)

// BastError is the typed error Bast understands. Use the constructors.
type BastError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *BastError) Error() string {
	return fmt.Sprintf("bast: %s: %s", e.Code, e.Message)
}

// bastErrBody is the inner object written to JSON for BastError responses.
// Pooled: avoids two anonymous-struct heap allocations per error response.
type bastErrBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type bastErrEnvelope struct {
	Error bastErrBody `json:"error"`
}

var bastErrPool = sync.Pool{New: func() any { return new(bastErrEnvelope) }}

// JSON serializes BastError into the standard error envelope.
func (e *BastError) JSON() []byte {
	env := bastErrPool.Get().(*bastErrEnvelope)
	env.Error.Code = e.Code
	env.Error.Message = e.Message
	b, _ := jsonx.Marshal(env)
	bastErrPool.Put(env)
	return b
}

// ValidationError is returned by ctx.Bind() when struct validation fails.
// The error boundary detects it and produces field-level 422 responses automatically.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "bast: validation failed" }

// BastStatus satisfies a convention checked by the error boundary.
func (e *ValidationError) BastStatus() int { return 422 }

// validationErrEnvelope is the JSON shape for validation error responses.
// Pooled: avoids anonymous-struct allocations per error response.
type validationErrBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

type validationErrEnvelope struct {
	Error validationErrBody `json:"error"`
}

var validationErrPool = sync.Pool{New: func() any { return new(validationErrEnvelope) }}

// JSON serializes ValidationError into the validation error envelope.
func (e *ValidationError) JSON() []byte {
	env := validationErrPool.Get().(*validationErrEnvelope)
	env.Error.Code = CodeValidation
	env.Error.Message = "request validation failed"
	env.Error.Fields = e.Fields
	b, _ := jsonx.Marshal(env)
	env.Error.Fields = nil
	validationErrPool.Put(env)
	return b
}

func ErrBadRequest(code, message string) error {
	return &BastError{Status: 400, Code: code, Message: message}
}

func ErrInvalidBody(message string) error {
	return &BastError{Status: 400, Code: CodeInvalidBody, Message: message}
}

func ErrUnauthorized(code, message string) error {
	return &BastError{Status: 401, Code: code, Message: message}
}

func ErrForbidden(code, message string) error {
	return &BastError{Status: 403, Code: code, Message: message}
}

func ErrNotFound(code, message string) error {
	return &BastError{Status: 404, Code: code, Message: message}
}

func ErrMethodNotAllowed(message string) error {
	return &BastError{Status: 405, Code: CodeMethodNotAllowed, Message: message}
}

func ErrTimeout(message string) error {
	return &BastError{Status: 408, Code: CodeTimeout, Message: message}
}

func ErrConflict(code, message string) error {
	return &BastError{Status: 409, Code: code, Message: message}
}

func ErrPayloadTooLarge(message string) error {
	return &BastError{Status: 413, Code: CodePayloadTooLarge, Message: message}
}

func ErrUnprocessable(code, message string) error {
	return &BastError{Status: 422, Code: code, Message: message}
}

func ErrTooManyRequests(message string) error {
	return &BastError{Status: 429, Code: CodeTooManyRequests, Message: message}
}

func ErrInternal(code, message string) error {
	return &BastError{Status: 500, Code: code, Message: message}
}

func ErrNotImplemented(message string) error {
	return &BastError{Status: 501, Code: CodeNotImplemented, Message: message}
}

func ErrServiceUnavailable(message string) error {
	return &BastError{Status: 503, Code: CodeServiceUnavailable, Message: message}
}

func ErrGatewayTimeout(message string) error {
	return &BastError{Status: 504, Code: CodeGatewayTimeout, Message: message}
}