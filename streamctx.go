package bast

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
)

// StreamCtx is the context for streaming handlers.
// Unlike *Ctx, it is NOT pooled — allocated per connection, GC'd when done.
// It embeds context.Context directly — safe to pass anywhere, for any duration.
type StreamCtx struct {
	context.Context
	Request *http.Request
	writer  http.ResponseWriter
	flusher http.Flusher
	bw      *bufio.Writer
}

// newStreamCtx creates a StreamCtx for a streaming connection.
func newStreamCtx(ctx context.Context, w http.ResponseWriter, r *http.Request) *StreamCtx {
	sc := &StreamCtx{
		Context: ctx,
		Request: r,
		writer:  w,
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
func (s *StreamCtx) Flush() {
	_ = s.bw.Flush()
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// Closed returns a channel that is closed when the client disconnects.
// Equivalent to StreamCtx.Done() via the embedded context.
func (s *StreamCtx) Closed() <-chan struct{} {
	return s.Done()
}