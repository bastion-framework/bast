package router

import "strings"

type nodeType uint8

const (
	nodeStatic   nodeType = iota // literal path segment
	nodeParam                    // :name — matches one segment
	nodeWildcard                 // *name — matches rest of path
)

// node is a single node in the radix tree.
// Children are ordered: static children first, then at most one param child,
// then at most one wildcard child. This ordering enforces static > param > wildcard priority.
type node struct {
	path     string
	nType    nodeType
	name     string // param or wildcard name without : or *
	children []*node
	handlers map[string]any // method → HandlerFunc or StreamHandlerFunc
}

// addChild inserts a child, maintaining static-first ordering.
func (n *node) addChild(child *node) {
	if child.nType == nodeStatic {
		// Static children go before param/wildcard children.
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

	// Wildcard segment.
	if path[0] == '*' {
		name := path[1:]
		for _, c := range n.children {
			if c.nType == nodeWildcard {
				if c.name == name {
					if c.handlers == nil {
						c.handlers = make(map[string]any, 4)
					}
					c.handlers[method] = handler
					return
				}
			}
		}
		wc := &node{
			path:     path,
			nType:    nodeWildcard,
			name:     name,
			handlers: map[string]any{method: handler},
		}
		n.addChild(wc)
		return
	}

	// Param segment.
	if path[0] == ':' {
		end := strings.IndexByte(path, '/')
		var segName string
		var rest string
		if end == -1 {
			segName = path[1:]
			rest = ""
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
		pc := &node{
			path:  ":" + segName,
			nType: nodeParam,
			name:  segName,
		}
		n.addChild(pc)
		pc.insert(rest, method, handler)
		return
	}

	// Static segment: find longest common prefix with existing static children.
	for _, c := range n.children {
		if c.nType != nodeStatic {
			continue
		}
		lcp := commonPrefix(path, c.path)
		if lcp == 0 {
			continue
		}

		if lcp == len(c.path) {
			// Existing child is fully matched — continue into it.
			c.insert(path[lcp:], method, handler)
			return
		}

		// Split c at lcp.
		// c.path = lcp + suffix. Create a new node for suffix.
		suffix := &node{
			path:     c.path[lcp:],
			nType:    nodeStatic,
			children: c.children,
			handlers: c.handlers,
		}
		c.path = c.path[:lcp]
		c.children = []*node{suffix}
		c.handlers = nil

		// Insert remaining path into (now-split) c.
		c.insert(path[lcp:], method, handler)
		return
	}

	// No matching child — create a new static child.
	// But first, split path on the first param or wildcard marker.
	staticEnd := indexParamOrWildcard(path)
	if staticEnd == 0 {
		// Starts with : or * — handled above, should not reach here.
		// Defensive: insert as-is.
		child := &node{path: path, nType: nodeStatic}
		n.addChild(child)
		child.insert("", method, handler)
		return
	}
	if staticEnd == -1 {
		staticEnd = len(path)
	}

	staticPart := path[:staticEnd]
	rest := path[staticEnd:]

	child := &node{path: staticPart, nType: nodeStatic}
	n.addChild(child)
	child.insert(rest, method, handler)
}

// search traverses the subtree rooted at n, matching path.
// params accumulates captured path parameters.
func (n *node) search(path, method string, params map[string]string) (any, bool, []string) {
	switch n.nType {
	case nodeStatic:
		if !strings.HasPrefix(path, n.path) {
			return nil, false, nil
		}
		path = path[len(n.path):]

	case nodeParam:
		// Consume up to the next '/' or end of path.
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
			return nil, false, nil
		}
		params[n.name] = val

	case nodeWildcard:
		// Consume the rest.
		params[n.name] = path
		path = ""
	}

	if path == "" {
		// We have consumed the full path. Check for a handler.
		if h, ok := n.handlers[method]; ok {
			return h, false, nil
		}
		// Check if other methods are registered → MethodNotAllowed.
		if len(n.handlers) > 0 {
			allow := make([]string, 0, len(n.handlers))
			for m := range n.handlers {
				allow = append(allow, m)
			}
			return nil, true, allow
		}
		return nil, false, nil
	}

	// Search children in order (static first — guaranteed by addChild).
	for _, c := range n.children {
		h, mna, allow := c.search(path, method, params)
		if h != nil || mna {
			return h, mna, allow
		}
		// Clean up any params added by a failed branch.
		if c.nType == nodeParam {
			delete(params, c.name)
		} else if c.nType == nodeWildcard {
			delete(params, c.name)
		}
	}

	return nil, false, nil
}

// commonPrefix returns the length of the longest common prefix of a and b.
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

// indexParamOrWildcard returns the index of the first ':' or '*' in s,
// or -1 if neither is present.
func indexParamOrWildcard(s string) int {
	for i := range len(s) {
		if s[i] == ':' || s[i] == '*' {
			return i
		}
	}
	return -1
}