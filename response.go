package bast

import "net/http"

// Response is an immutable value representing what the framework should write.
// Constructed via Ctx methods — never directly instantiated by user code.
// All methods use value receivers and return new Response values — never mutate.
//
// Internal constructors below are used by Ctx methods and basttest.
type Response struct {
	status      int
	headers     map[string]string
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

// Headers returns the response headers map.
func (r Response) Headers() map[string]string { return r.headers }

// Cookies returns cookies to be set on the response.
func (r Response) Cookies() []*http.Cookie { return r.cookies }

// Redirect returns the redirect URL, if any.
func (r Response) Redirect() string { return r.redirect }

// IsError reports whether this response carries an error.
func (r Response) IsError() bool { return r.err != nil }

// Err returns the underlying error, if any.
func (r Response) Err() error { return r.err }

// WithHeader returns a new Response with an additional header set.
func (r Response) WithHeader(key, value string) Response {
	h := make(map[string]string, len(r.headers)+1)
	for k, v := range r.headers {
		h[k] = v
	}
	h[key] = value
	r.headers = h
	return r
}

// WithStatus returns a new Response with a different status code.
func (r Response) WithStatus(code int) Response {
	r.status = code
	return r
}

// newRawResponse builds a response for Ctx.Raw() and internal use.
func newRawResponse(status int, contentType string, body []byte) Response {
	return Response{
		status:      status,
		contentType: contentType,
		body:        body,
		headers:     make(map[string]string),
	}
}

// newErrorResponse builds a response carrying an error for the boundary.
func newErrorResponse(err error) Response {
	return Response{
		status:  500,
		headers: make(map[string]string),
		err:     err,
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