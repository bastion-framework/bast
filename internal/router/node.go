package router

import "unsafe"

// compile-time: flatNode must fit in two 64-byte cache lines.
var _ [128 - unsafe.Sizeof(flatNode{})]byte

type nodeKind uint8

const (
	kindStatic   nodeKind = iota
	kindParam
	kindWildcard
)

// mutableNode is the build-phase tree node, discarded after freeze.
type mutableNode struct {
	segment   string
	kind      nodeKind
	param     string         // capture name for param/wildcard nodes
	handler   any            // handler for exact path (no trailing slash)
	tsHandler any            // handler for path + trailing slash (e.g. /users/)
	static    []*mutableNode // static-segment children, insertion-ordered
	paramC    *mutableNode   // at most one param child per node
	wildC     *mutableNode   // at most one wildcard child per node
}

func (n *mutableNode) insert(segs []string, si int, handler any) {
	if si == len(segs) {
		n.handler = handler
		return
	}
	seg := segs[si]

	// Empty segment arises from a trailing slash in the pattern (e.g. /users/).
	// Store it as the trailing-slash handler of this node so that a URL ending
	// with '/' can match it independently of the same path without the slash.
	if seg == "" {
		n.tsHandler = handler
		return
	}

	switch {
	case seg[0] == '*':
		name := seg[1:]
		if n.wildC == nil {
			n.wildC = &mutableNode{segment: seg, kind: kindWildcard, param: name}
		} else if n.wildC.param != name {
			panic("router: conflicting wildcard names at same position: " + n.wildC.param + " vs " + name)
		}
		n.wildC.handler = handler

	case seg[0] == ':':
		name := seg[1:]
		if n.paramC == nil {
			n.paramC = &mutableNode{segment: seg, kind: kindParam, param: name}
		} else if n.paramC.param != name {
			panic("router: conflicting param names at same position: " + n.paramC.param + " vs " + name)
		}
		n.paramC.insert(segs, si+1, handler)

	default:
		for _, c := range n.static {
			if c.segment == seg {
				c.insert(segs, si+1, handler)
				return
			}
		}
		child := &mutableNode{segment: seg, kind: kindStatic}
		n.static = append(n.static, child)
		child.insert(segs, si+1, handler)
	}
}

// flatNode is the frozen, arena-allocated search node (72 bytes, two cache lines).
//
// Hot fields (cache line 1, bytes 0–55): segment, param, handler, childBase,
// staticCount, hasParam, hasWild, kind — fetched on every node visit.
// tsHandler (cache line 2, bytes 56–71) is only fetched when the URL ends with '/'.
//
// Children of node i are always contiguous:
//   static:   nodes[childBase : childBase+staticCount]
//   param:    nodes[childBase+staticCount]          (if hasParam==1)
//   wildcard: nodes[childBase+staticCount+hasParam] (if hasWild==1)
type flatNode struct {
	segment     string
	param       string
	handler     any
	childBase   uint32
	staticCount uint8
	hasParam    uint8
	hasWild     uint8
	kind        nodeKind
	tsHandler   any
}

// freeze converts the mutable insertion tree into a flat []flatNode via BFS.
// BFS guarantees each node's children occupy contiguous indices before any
// child is processed, keeping the static scan cache-line friendly.
func freeze(root *mutableNode) []flatNode {
	if root == nil {
		return nil
	}

	total := countNodes(root)
	nodes := make([]flatNode, total)

	type entry struct {
		mn  *mutableNode
		idx int
	}
	queue := make([]entry, 0, total)
	queue = append(queue, entry{root, 0})
	nextIdx := 1

	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]
		mn := e.mn

		sc := len(mn.static)
		hp := mn.paramC != nil
		hw := mn.wildC != nil
		cb := nextIdx
		nextIdx += sc + btoi(hp) + btoi(hw)

		nodes[e.idx] = flatNode{
			segment:     mn.segment,
			param:       mn.param,
			handler:     mn.handler,
			tsHandler:   mn.tsHandler,
			childBase:   uint32(cb),
			staticCount: uint8(sc),
			hasParam:    btoU8(hp),
			hasWild:     btoU8(hw),
			kind:        mn.kind,
		}

		for i, c := range mn.static {
			queue = append(queue, entry{c, cb + i})
		}
		if hp {
			queue = append(queue, entry{mn.paramC, cb + sc})
		}
		if hw {
			queue = append(queue, entry{mn.wildC, cb + sc + btoi(hp)})
		}
	}
	return nodes
}

func countNodes(n *mutableNode) int {
	if n == nil {
		return 0
	}
	c := 1
	for _, s := range n.static {
		c += countNodes(s)
	}
	return c + countNodes(n.paramC) + countNodes(n.wildC)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func btoU8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}