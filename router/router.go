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

// Match is the result of a successful route lookup.
type Match struct {
	Handler any               // HandlerFunc or StreamHandlerFunc
	Params  map[string]string // extracted path parameters
}

// Router is a radix tree HTTP router.
// Each HTTP method has its own tree rooted at a shared root node that
// dispatches into per-method subtrees, allowing method-not-allowed detection.
type Router struct {
	// trees maps HTTP method → root node of that method's radix tree.
	trees map[string]*node
}

// New creates an empty Router.
func New() *Router {
	return &Router{
		trees: make(map[string]*node),
	}
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

	// Strip leading '/' — the root node owns it.
	path := pattern[1:]

	// Normalise path: strip any leading slash segments already consumed by root.
	root.insert(path, method, handler)
}

// Find looks up method+path and returns a Match or an error.
// Returns ErrNotFound if no route matches the path.
// Returns *MethodNotAllowedError if the path matches but not the method.
func (r *Router) Find(method, path string) (Match, error) {
	if path == "" {
		path = "/"
	}

	// Try the exact method tree first.
	if root, ok := r.trees[method]; ok {
		params := make(map[string]string, 8)
		h, mna, allow := root.search(path, method, params)
		if h != nil {
			return Match{Handler: h, Params: params}, nil
		}
		if mna {
			return Match{}, &MethodNotAllowedError{Allow: allow}
		}
	}

	// Path not found in the exact method tree. Check other method trees to
	// distinguish 404 from 405.
	var allow []string
	for m, root := range r.trees {
		if m == method {
			continue
		}
		params := make(map[string]string, 8)
		h, _, _ := root.search(path, m, params)
		if h != nil {
			allow = append(allow, m)
		}
	}
	if len(allow) > 0 {
		return Match{}, &MethodNotAllowedError{Allow: allow}
	}

	return Match{}, ErrNotFound
}