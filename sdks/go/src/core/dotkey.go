package core

import "strings"

// DotNode is an intermediate tree node used to merge dot-key attrs
// (e.g. "user.id", "user.name") into nested JSON objects.
type DotNode struct {
	// Value holds the leaf attr value (Kind, Value) when Children is nil.
	Value *LeafValue
	// Children maps sub-key → child node.
	Children map[string]*DotNode
}

// LeafValue carries the typed value of a leaf node.
type LeafValue struct {
	Kind  uint8 // matches loxa.Kind
	Value any
}

// ExpandDotKeys takes a flat list of (key, kind, value) tuples and converts
// all dot-separated keys into a nested tree. Non-dot keys are left as-is.
//
// Returns two slices:
//   - plain: attrs whose keys have no dot (or dot expansion is disabled)
//   - groups: map[topKey]*DotNode for merged nested objects
func ExpandDotKeys(keys []string, kinds []uint8, values []any) (
	plainKeys []string, plainKinds []uint8, plainValues []any,
	groupKeys []string, groupRoots []*DotNode,
) {
	// Temporary map: top-level key → root DotNode
	roots := make(map[string]*DotNode)
	var rootOrder []string // preserve insertion order

	for i, key := range keys {
		dot := strings.IndexByte(key, '.')
		if dot < 0 {
			// No dot: plain attr
			plainKeys = append(plainKeys, key)
			plainKinds = append(plainKinds, kinds[i])
			plainValues = append(plainValues, values[i])
			continue
		}

		// Split "user.id.extra" → ["user", "id", "extra"]
		parts := strings.Split(key, ".")
		top := parts[0]

		root, exists := roots[top]
		if !exists {
			root = &DotNode{Children: make(map[string]*DotNode)}
			roots[top] = root
			rootOrder = append(rootOrder, top)
		}

		// Drill down, creating intermediate nodes.
		cur := root
		for _, part := range parts[1 : len(parts)-1] {
			child, ok := cur.Children[part]
			if !ok {
				child = &DotNode{Children: make(map[string]*DotNode)}
				cur.Children[part] = child
			}
			cur = child
		}

		leaf := parts[len(parts)-1]
		cur.Children[leaf] = &DotNode{
			Value: &LeafValue{Kind: kinds[i], Value: values[i]},
		}
	}

	for _, k := range rootOrder {
		groupKeys = append(groupKeys, k)
		groupRoots = append(groupRoots, roots[k])
	}
	return
}
