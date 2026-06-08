package bast

// MiddlewareFunc wraps a HandlerFunc to form a pipeline.
// Call next(ctx) to pass control to the next middleware or handler.
// Do not call next to short-circuit (e.g. auth failure) — return a Response directly.
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// buildPipeline composes middleware and a final handler into a single HandlerFunc.
// Built at registration time — the pipeline is one function call at request time.
// Wrap in reverse so first middleware in slice runs first.
func buildPipeline(handler HandlerFunc, middleware []MiddlewareFunc) HandlerFunc {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}