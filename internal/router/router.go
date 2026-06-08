package router

import "errors"

// ErrNotFound is returned when no route matches the path.
var ErrNotFound = errors.New("router: not found")

// MethodNotAllowedError is returned when a route exists but not for the given method.
type MethodNotAllowedError struct {
	Allow []string
}

func (e *MethodNotAllowedError) Error() string {
	return "router: method not allowed"
}

// Param is a single URL path parameter extracted during route matching.
// Stored in a pre-allocated array inside Ctx, no heap allocation on the hot path.
type Param struct {
	Key   string
	Value string
}

// Match is the result of a successful route lookup.
// Params are written into the caller-provided slice, no allocation in Match itself.
type Match struct {
	Handler any
	Params  []Param // points into the caller-provided backing array
}

// Router is a radix tree HTTP router.
// Each HTTP method has its own tree, allowing method-not-allowed detection.
type Router struct {
	trees map[string]*node
}

// New creates an empty Router.
func New() *Router {
	return &Router{trees: make(map[string]*node)}
}

// Add registers a handler for method+pattern.
// pattern must start with '/'. Panics on invalid patterns (programmer error).
func (r *Router) Add(method, pattern string, handler any) {
	if pattern == "" || pattern[0] != '/' {
		panic("router: pattern must start with '/'")
	}
	root := r.trees[method]
	if root == nil {
		root = &node{path: "/", nType: nodeStatic}
		r.trees[method] = root
	}
	root.insert(pattern[1:], method, handler)
}

// Find looks up method+path, writes extracted params into dst, and returns a Match.
// dst must be backed by pre-allocated storage (e.g. a [8]Param array in the caller).
// Appending stays within the pre-allocated capacity → 0 heap allocations on the hot path.
//
// Returns ErrNotFound if no route matches the path.
// Returns *MethodNotAllowedError if the path matches but not the method.
func (r *Router) Find(method, path string, dst []Param) (Match, error) {
	if path == "" {
		path = "/"
	}

	// Try the exact method tree first.
	if root, ok := r.trees[method]; ok {
		h, mna, params, allow := root.search(path, method, dst)
		if h != nil {
			return Match{Handler: h, Params: params}, nil
		}
		if mna {
			return Match{}, &MethodNotAllowedError{Allow: allow}
		}
	}

	// Path not found — check other method trees to distinguish 404 from 405.
	var allow []string
	for m, root := range r.trees {
		if m == method {
			continue
		}
		// Use a stack-allocated dummy buffer — we only need to know if a handler exists.
		var dummy [8]Param
		h, _, _, _ := root.search(path, m, dummy[:0])
		if h != nil {
			allow = append(allow, m)
		}
	}
	if len(allow) > 0 {
		return Match{}, &MethodNotAllowedError{Allow: allow}
	}

	return Match{}, ErrNotFound
}