package router

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
)

// ErrNotFound is returned when no route matches the path.
var ErrNotFound = errors.New("router: not found")

// MethodNotAllowedError is returned when the path matches but not the method.
type MethodNotAllowedError struct {
	Allow []string
}

func (e *MethodNotAllowedError) Error() string { return "router: method not allowed" }

type Param struct {
	Key   string
	Value string
}

type Match struct {
	Handler any
	Params  []Param
}

const numMethods = 9

func methodIdx(m string) int {
	switch m {
	case "GET":
		return 0
	case "POST":
		return 1
	case "PUT":
		return 2
	case "PATCH":
		return 3
	case "DELETE":
		return 4
	case "HEAD":
		return 5
	case "OPTIONS":
		return 6
	case "CONNECT":
		return 7
	case "TRACE":
		return 8
	default:
		return -1
	}
}

func methodName(i int) string {
	switch i {
	case 0:
		return "GET"
	case 1:
		return "POST"
	case 2:
		return "PUT"
	case 3:
		return "PATCH"
	case 4:
		return "DELETE"
	case 5:
		return "HEAD"
	case 6:
		return "OPTIONS"
	case 7:
		return "CONNECT"
	case 8:
		return "TRACE"
	}
	return ""
}

// Router is a segment-aware radix tree router. Each node is one full URL segment;
// at boot the mutable tree is BFS-frozen into a flat arena for cache-local lookup.
// Add during startup only; the first Find freezes all trees permanently.
//
// Concurrency contract: all Add calls must happen-before the first Find (the
// usual case — registration in main, then Listen). Add is not safe to call
// concurrently with Find; the sealed check panics on late Adds but cannot
// detect a true race.
type Router struct {
	mutable [numMethods]*mutableNode // build phase only; nil after freeze
	frozen  [numMethods][]flatNode   // immutable after freeze
	once    sync.Once
	sealed  atomic.Bool
}

func New() *Router { return &Router{} }

// Add registers handler for method+pattern. Panics on structural violations or
// if called after the router has been frozen by the first Find.
func (r *Router) Add(method, pattern string, handler any) {
	if r.sealed.Load() {
		panic("router: Add called after the router was frozen by the first Find")
	}

	if pattern == "" || pattern[0] != '/' {
		panic("router: pattern must start with '/': " + pattern)
	}

	mi := methodIdx(method)
	if mi < 0 {
		panic("router: unrecognised HTTP method: " + method)
	}

	var segs []string
	if pattern != "/" {
		segs = strings.Split(pattern[1:], "/")
		for i, seg := range segs {
			if seg == "" && i < len(segs)-1 {
				panic("router: pattern contains an embedded empty segment (double slash): " + pattern)
			}
			if len(seg) > 0 && seg[0] == '*' && i != len(segs)-1 {
				panic("router: wildcard segment must be the last: " + pattern)
			}
		}
	}

	if r.mutable[mi] == nil {
		r.mutable[mi] = &mutableNode{kind: kindStatic}
	}
	r.mutable[mi].insert(segs, 0, handler)
}

// Find looks up method+path and writes extracted params into dst.
// The first call freezes all trees; Find is safe for concurrent use after that.
func (r *Router) Find(method, path string, dst []Param) (Match, error) {
	r.once.Do(func() {
		for i := range numMethods {
			if r.mutable[i] != nil {
				r.frozen[i] = freeze(r.mutable[i])
				r.mutable[i] = nil
			}
		}
		r.sealed.Store(true)
	})

	if path == "" {
		path = "/"
	}

	mi := methodIdx(method)
	if mi >= 0 {
		if nodes := r.frozen[mi]; len(nodes) > 0 {
			if h, params := search(nodes, path, dst); h != nil {
				return Match{Handler: h, Params: params}, nil
			}
		}
	}

	var allow []string
	var dummy [8]Param
	for i := range numMethods {
		if i == mi {
			continue
		}
		if nodes := r.frozen[i]; len(nodes) > 0 {
			if h, _ := search(nodes, path, dummy[:0]); h != nil {
				allow = append(allow, methodName(i))
			}
		}
	}
	if len(allow) > 0 {
		return Match{}, &MethodNotAllowedError{Allow: allow}
	}
	return Match{}, ErrNotFound
}

func search(nodes []flatNode, path string, dst []Param) (any, []Param) {
	cur := 0
	pos := 1 // skip the leading '/'

loop:
	for pos <= len(path) {
		if pos == len(path) {
			if pos == 1 { // path == "/" exactly
				return nodes[0].handler, dst
			}
			return nodes[cur].tsHandler, dst
		}

		rel := strings.IndexByte(path[pos:], '/')
		var seg string
		var next int
		if rel < 0 {
			seg = path[pos:]
			next = len(path) + 1 // clean end; next > len(path) exits the loop
		} else {
			seg = path[pos : pos+rel]
			next = pos + rel + 1
		}

		n := &nodes[cur]

		for i := uint8(0); i < n.staticCount; i++ {
			if c := &nodes[int(n.childBase)+int(i)]; c.segment == seg {
				cur = int(n.childBase) + int(i)
				pos = next
				continue loop
			}
		}

		if n.hasParam == 1 {
			pi := int(n.childBase) + int(n.staticCount)
			dst = append(dst, Param{Key: nodes[pi].param, Value: seg})
			cur = pi
			pos = next
			continue
		}

		if n.hasWild == 1 {
			wi := int(n.childBase) + int(n.staticCount) + int(n.hasParam)
			dst = append(dst, Param{Key: nodes[wi].param, Value: path[pos:]})
			return nodes[wi].handler, dst
		}

		return nil, dst
	}

	return nodes[cur].handler, dst
}