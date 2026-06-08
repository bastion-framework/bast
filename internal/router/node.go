package router

import "strings"

type nodeType uint8

const (
	nodeStatic   nodeType = iota
	nodeParam
	nodeWildcard
)

// node is a single node in the radix tree.
// Children are ordered: static first, then param, then wildcard.
type node struct {
	path     string
	nType    nodeType
	name     string
	children []*node
	handlers map[string]any
}

// addChild inserts a child maintaining static-first ordering.
func (n *node) addChild(child *node) {
	if child.nType == nodeStatic {
		insertAt := len(n.children)
		for i, c := range n.children {
			if c.nType != nodeStatic {
				insertAt = i
				break
			}
		}
		n.children = append(n.children, nil)
		copy(n.children[insertAt+1:], n.children[insertAt:])
		n.children[insertAt] = child
	} else {
		n.children = append(n.children, child)
	}
}

// insert adds path+method+handler into the subtree rooted at n.
func (n *node) insert(path, method string, handler any) {
	if path == "" {
		if n.handlers == nil {
			n.handlers = make(map[string]any, 4)
		}
		n.handlers[method] = handler
		return
	}

	if path[0] == '*' {
		name := path[1:]
		for _, c := range n.children {
			if c.nType == nodeWildcard && c.name == name {
				if c.handlers == nil {
					c.handlers = make(map[string]any, 4)
				}
				c.handlers[method] = handler
				return
			}
		}
		wc := &node{path: path, nType: nodeWildcard, name: name,
			handlers: map[string]any{method: handler}}
		n.addChild(wc)
		return
	}

	if path[0] == ':' {
		end := strings.IndexByte(path, '/')
		var segName, rest string
		if end == -1 {
			segName = path[1:]
		} else {
			segName = path[1:end]
			rest = path[end+1:]
		}
		for _, c := range n.children {
			if c.nType == nodeParam && c.name == segName {
				c.insert(rest, method, handler)
				return
			}
		}
		pc := &node{path: ":" + segName, nType: nodeParam, name: segName}
		n.addChild(pc)
		pc.insert(rest, method, handler)
		return
	}

	// Static: find longest common prefix with existing static children.
	for _, c := range n.children {
		if c.nType != nodeStatic {
			continue
		}
		lcp := commonPrefix(path, c.path)
		if lcp == 0 {
			continue
		}
		if lcp == len(c.path) {
			c.insert(path[lcp:], method, handler)
			return
		}
		// Split c at lcp.
		suffix := &node{path: c.path[lcp:], nType: nodeStatic,
			children: c.children, handlers: c.handlers}
		c.path = c.path[:lcp]
		c.children = []*node{suffix}
		c.handlers = nil
		c.insert(path[lcp:], method, handler)
		return
	}

	staticEnd := indexParamOrWildcard(path)
	if staticEnd == -1 {
		staticEnd = len(path)
	}
	if staticEnd == 0 {
		staticEnd = len(path)
	}
	child := &node{path: path[:staticEnd], nType: nodeStatic}
	n.addChild(child)
	child.insert(path[staticEnd:], method, handler)
}

// search traverses the subtree matching path.
// params is a slice backed by the caller's pre-allocated array.
// Backtracking restores params to savedLen — no heap allocation.
// Returns: handler, isMethodNotAllowed, updatedParams, allowedMethods.
func (n *node) search(path, method string, params []Param) (any, bool, []Param, []string) {
	savedLen := len(params)

	switch n.nType {
	case nodeStatic:
		if !strings.HasPrefix(path, n.path) {
			return nil, false, params, nil
		}
		path = path[len(n.path):]

	case nodeParam:
		end := strings.IndexByte(path, '/')
		var val string
		if end == -1 {
			val = path
			path = ""
		} else {
			val = path[:end]
			path = path[end+1:]
		}
		if val == "" {
			return nil, false, params, nil
		}
		params = append(params, Param{Key: n.name, Value: val})

	case nodeWildcard:
		params = append(params, Param{Key: n.name, Value: path})
		path = ""
	}

	if path == "" {
		if h, ok := n.handlers[method]; ok {
			return h, false, params, nil
		}
		if len(n.handlers) > 0 {
			allow := make([]string, 0, len(n.handlers))
			for m := range n.handlers {
				allow = append(allow, m)
			}
			return nil, true, params, allow
		}
		return nil, false, params[:savedLen], nil
	}

	for _, c := range n.children {
		h, mna, resultParams, allow := c.search(path, method, params)
		if h != nil || mna {
			return h, mna, resultParams, allow
		}
		// Failed branch — restore params to before this child was tried.
		params = params[:len(params)]
	}

	// No child matched — restore params to entry length.
	return nil, false, params[:savedLen], nil
}

func commonPrefix(a, b string) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	for i := range max {
		if a[i] != b[i] {
			return i
		}
	}
	return max
}

func indexParamOrWildcard(s string) int {
	for i := range len(s) {
		if s[i] == ':' || s[i] == '*' {
			return i
		}
	}
	return -1
}