package bast

import (
	"net/http"
	"strings"
)

// header is a key-value pair stored in the Response.
// Using a slice of header instead of map[string]string eliminates the map bucket
// allocation on every response construction — most responses never set extra headers.
type header struct{ key, value string }

// Response is an immutable value representing what the framework should write.
// Constructed via Ctx methods — never directly instantiated by user code.
// All methods use value receivers and return new Response values — never mutate.
//
// Internal constructors below are used by Ctx methods and basttest.
type Response struct {
	status      int
	headers     []header // nil until first WithHeader call — no alloc on construction
	body        []byte
	contentType string
	err         error
	redirect    string
	cookies     []*http.Cookie
}

// Status returns the HTTP status code.
func (r Response) Status() int { return r.status }

// Body returns the raw response body bytes.
func (r Response) Body() []byte { return r.body }

// ContentType returns the Content-Type header value.
func (r Response) ContentType() string { return r.contentType }

// Headers returns the response headers as a map. Built lazily — not called on the hot path.
// Returns nil when no headers have been set.
func (r Response) Headers() map[string]string {
	if len(r.headers) == 0 {
		return nil
	}
	m := make(map[string]string, len(r.headers))
	for _, h := range r.headers {
		m[h.key] = h.value
	}
	return m
}

// Cookies returns cookies to be set on the response.
func (r Response) Cookies() []*http.Cookie { return r.cookies }

// Redirect returns the redirect URL, if any.
func (r Response) Redirect() string { return r.redirect }

// IsError reports whether this response carries an error.
func (r Response) IsError() bool { return r.err != nil }

// Err returns the underlying error, if any.
func (r Response) Err() error { return r.err }

// WithHeader returns a new Response with an additional header set.
// Allocates a new backing slice of exactly len+1 — no capacity aliasing possible.
// Strips \r and \n from both key and value to prevent CRLF injection.
func (r Response) WithHeader(key, value string) Response {
	if strings.ContainsAny(key, "\r\n") {
		key = strings.Map(func(c rune) rune {
			if c == '\r' || c == '\n' {
				return -1
			}
			return c
		}, key)
	}
	if strings.ContainsAny(value, "\r\n") {
		value = strings.Map(func(c rune) rune {
			if c == '\r' || c == '\n' {
				return -1
			}
			return c
		}, value)
	}
	n := len(r.headers)
	h := make([]header, n+1)
	copy(h, r.headers)
	h[n] = header{key, value}
	r.headers = h
	return r
}

// WithStatus returns a new Response with a different status code.
func (r Response) WithStatus(code int) Response {
	r.status = code
	return r
}

// newRawResponse builds a response for Ctx.Raw() and internal use.
// headers is nil by default — no allocation until WithHeader is called.
func newRawResponse(status int, contentType string, body []byte) Response {
	return Response{
		status:      status,
		contentType: contentType,
		body:        body,
	}
}

// newErrorResponse builds a response carrying an error for the boundary.
func newErrorResponse(err error) Response {
	return Response{
		status: 500,
		err:    err,
	}
}

// NewRawResponse is the exported form used by basttest and response tests.
func NewRawResponse(status int, contentType string, body []byte) Response {
	return newRawResponse(status, contentType, body)
}

// NewErrorResponse is the exported form used by basttest and response tests.
func NewErrorResponse(err error) Response {
	return newErrorResponse(err)
}

// WithCookie returns a new Response with an additional cookie attached.
func (r Response) WithCookie(cookie *http.Cookie) Response {
	c := make([]*http.Cookie, len(r.cookies)+1)
	copy(c, r.cookies)
	c[len(r.cookies)] = cookie
	r.cookies = c
	return r
}
