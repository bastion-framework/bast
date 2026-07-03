package bast

import (
	"bufio"
	"context"
	"fmt"
	"net/http"

	"github.com/bastion-framework/bast/internal/router"
)

// StreamCtx is the context for streaming handlers.
//
// Unlike *Ctx it is intentionally NOT pooled. Stream connections are long-lived
// (seconds to hours), so their lifetime is unbounded and cannot be managed by a
// sync.Pool: the pool would either hold the slot indefinitely (defeating the pool)
// or return the object while the connection is still active (use-after-free). One
// allocation per connection is negligible compared to the cost of keeping the
// connection open. GC handles reclamation naturally when Done fires and the handler
// returns.
//
// It embeds context.Context directly — safe to pass anywhere, for any duration.
type StreamCtx struct {
	context.Context
	Request *http.Request
	writer  http.ResponseWriter
	flusher http.Flusher
	bw      *bufio.Writer
	params  []router.Param
	store   map[string]any // lazily allocated on first Set

	status     int  // status reported to the request logger
	headerSent bool // true once the status line is (implicitly) on the wire
}

// newStreamCtx creates a StreamCtx for a streaming connection.
func newStreamCtx(ctx context.Context, w http.ResponseWriter, r *http.Request) *StreamCtx {
	sc := &StreamCtx{
		Context: ctx,
		Request: r,
		writer:  w,
		status:  http.StatusOK,
	}
	if f, ok := w.(http.Flusher); ok {
		sc.flusher = f
	}
	sc.bw = bufio.NewWriter(w)
	return sc
}

// SetHeader sets a response header. Must be called before first Write or Send.
func (s *StreamCtx) SetHeader(key, value string) {
	s.writer.Header().Set(key, value)
}

// Status sends the status line immediately. Call before any Write, Send, or
// Flush; once data is on the wire the status is fixed at 200 and Status is a no-op.
func (s *StreamCtx) Status(code int) {
	if s.headerSent {
		return
	}
	s.status = code
	s.headerSent = true
	s.writer.WriteHeader(code)
}

// Send writes a Server-Sent Event to the client.
func (s *StreamCtx) Send(event, data string) error {
	if _, err := fmt.Fprintf(s.bw, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return fmt.Errorf("bast: stream send: %w", err)
	}
	return nil
}

// Write writes raw bytes to the client.
func (s *StreamCtx) Write(p []byte) (int, error) {
	n, err := s.bw.Write(p)
	if err != nil {
		return n, fmt.Errorf("bast: stream write: %w", err)
	}
	return n, nil
}

// Flush flushes buffered data to the client immediately.
// Returns an error if the underlying write fails (e.g. client disconnected).
func (s *StreamCtx) Flush() error {
	if err := s.bw.Flush(); err != nil {
		return fmt.Errorf("bast: stream flush: %w", err)
	}
	s.headerSent = true // first flush implicitly sends the 200 status line
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

// Closed returns a channel that is closed when the client disconnects.
// Equivalent to StreamCtx.Done() via the embedded context.
func (s *StreamCtx) Closed() <-chan struct{} {
	return s.Done()
}

// Param returns a URL path parameter by name.
// e.g. STREAM("/:id/events", ...) → sctx.Param("id") == "42" for /42/events
func (s *StreamCtx) Param(key string) string {
	for i := range s.params {
		if s.params[i].Key == key {
			return s.params[i].Value
		}
	}
	return ""
}

// Get retrieves a value from the request-scoped store.
// Values are populated by guards that ran before the stream handler.
func (s *StreamCtx) Get(key string) (any, bool) {
	v, ok := s.store[key]
	return v, ok
}

// Set stores a value in the request-scoped store.
func (s *StreamCtx) Set(key string, val any) {
	if s.store == nil {
		s.store = make(map[string]any, 8)
	}
	s.store[key] = val
}

// MustGet retrieves a value and panics if not found.
// Use only when a guard guarantees the value was set.
func (s *StreamCtx) MustGet(key string) any {
	v, ok := s.store[key]
	if !ok {
		panic(fmt.Sprintf("bast: MustGet: key %q not found in store", key))
	}
	return v
}

// --- Test helpers (exported for basttest only) ---

// NewTestStreamCtx creates a StreamCtx outside the pool for unit testing stream handlers.
// Never pool or reuse a test StreamCtx.
func NewTestStreamCtx() *StreamCtx {
	return &StreamCtx{
		Context: context.Background(),
		status:  http.StatusOK,
	}
}

// SetStreamTestParam sets a route path parameter on a test StreamCtx. For basttest only.
func SetStreamTestParam(s *StreamCtx, key, value string) {
	s.params = append(s.params, router.Param{Key: key, Value: value})
}

// SetStreamTestStore sets a value in the store of a test StreamCtx. For basttest only.
func SetStreamTestStore(s *StreamCtx, key string, val any) {
	s.Set(key, val)
}
