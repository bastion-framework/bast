package bast

import (
	"errors"
	"log/slog"
	"net/http"
)

// ErrorHandler maps an error to a Response.
type ErrorHandler func(ctx *Ctx, err error) Response

// DefaultErrorHandler is the built-in error boundary.
// Maps BastError and ValidationError to structured JSON responses.
// Unknown errors become 500 without leaking internals.
func DefaultErrorHandler(ctx *Ctx, err error) Response {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return newRawResponse(422, "application/json", ve.JSON())
	}

	var bastErr *BastError
	if errors.As(err, &bastErr) {
		return newRawResponse(bastErr.Status, "application/json", bastErr.JSON())
	}

	slog.Error("unhandled error", "err", err, "path", ctx.Path())
	internal := &BastError{Status: 500, Code: CodeInternal, Message: "internal server error"}
	return newRawResponse(500, "application/json", internal.JSON())
}

// writeResponse writes a Response to the wire. Called once per request, after the handler returns.
func writeResponse(w http.ResponseWriter, resp Response) {
	if resp.Redirect() != "" {
		http.Redirect(w, &http.Request{}, resp.Redirect(), resp.Status())
		return
	}

	for _, h := range resp.headers {
		w.Header().Set(h.key, h.value)
	}
	for _, c := range resp.Cookies() {
		http.SetCookie(w, c)
	}
	if ct := resp.ContentType(); ct != "" {
		hh := w.Header()
		if v := hh["Content-Type"]; cap(v) > 0 {
			hh["Content-Type"] = v[:1]
			hh["Content-Type"][0] = ct
		} else {
			hh["Content-Type"] = []string{ct}
		}
	}
	if resp.Status() != 0 {
		w.WriteHeader(resp.Status())
	}
	if body := resp.Body(); len(body) > 0 {
		_, _ = w.Write(body)
	}
}