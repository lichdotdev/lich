// Package filetree draws a tree of paths: flat slash-separated paths nested
// into folders and flattened into the rows on screen.
package filetree

import (
	"slices"
	"strings"
)

type Node struct {
	Name     string // what the row shows: the last segment, or a collapsed chain
	Path     string // repo-relative, the row's id for folds and ReadFile
	Dir      bool
	Children []*Node
}

// Build nests paths into folders, folders before files and each group
// sorted case-insensitively, then collapses the chains of single folders.
func Build(paths []string) []*Node {
	var root []*Node
	indexes := map[*Node]map[treeChildKey]*Node{}
	rootIndex := make(map[treeChildKey]*Node)
	for _, p := range paths {
		level := &root
		index := rootIndex
		prefix := ""
		parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
		for i, name := range parts {
			prefix = strings.TrimPrefix(prefix+"/"+name, "/")
			dir := i < len(parts)-1
			key := treeChildKey{name: name, dir: dir}
			node := index[key]
			if node == nil {
				node = &Node{Name: name, Path: prefix, Dir: dir}
				*level = append(*level, node)
				index[key] = node
			}
			level = &node.Children
			if i+1 < len(parts) {
				if indexes[node] == nil {
					indexes[node] = make(map[treeChildKey]*Node)
				}
				index = indexes[node]
			}
		}
	}
	sortTree(root)
	return collapseChains(root)
}

type treeChildKey struct {
	name string
	dir  bool
}

func sortTree(nodes []*Node) {
	slices.SortStableFunc(nodes, func(a, b *Node) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	for _, n := range nodes {
		sortTree(n.Children)
	}
}

// collapseChains merges a folder holding only one folder into one row,
// keeping the deepest path as its id: src/main/java reads as one row, not
// three levels of indentation before the first file.
func collapseChains(nodes []*Node) []*Node {
	out := make([]*Node, len(nodes))
	for i, n := range nodes {
		merged := *n
		for len(merged.Children) == 1 && merged.Children[0].Dir {
			only := merged.Children[0]
			merged = Node{Name: merged.Name + "/" + only.Name, Path: only.Path, Dir: true, Children: only.Children}
		}
		merged.Children = collapseChains(merged.Children)
		out[i] = &merged
	}
	return out
}

type Delta struct{ Added, Deleted int }

// WithDirStats adds every folder holding a changed file, summed, so a closed
// folder says where the diff is. It sums the tree it is given, so a filtered
// tree counts only what the filter left.
func WithDirStats(tree []*Node, files map[string]Delta) map[string]Delta {
	out := make(map[string]Delta, len(files))
	for k, v := range files {
		out[k] = v
	}
	var walk func(n *Node) (Delta, bool)
	walk = func(n *Node) (Delta, bool) {
		if !n.Dir {
			d, ok := files[n.Path]
			return d, ok
		}
		var sum Delta
		found := false
		for _, c := range n.Children {
			if d, ok := walk(c); ok {
				sum.Added, sum.Deleted, found = sum.Added+d.Added, sum.Deleted+d.Deleted, true
			}
		}
		if found {
			out[n.Path] = sum
		}
		return sum, found
	}
	for _, n := range tree {
		walk(n)
	}
	return out
}

// Row is one row on screen.
type Row struct {
	Node  *Node
	Depth int
	Open  bool
}

// VisibleRows flattens the tree to the rows on screen: a folder's children
// only while it is open.
func VisibleRows(tree []*Node, isOpen func(path string) bool) []Row {
	var rows []Row
	var walk func(nodes []*Node, depth int)
	walk = func(nodes []*Node, depth int) {
		for _, n := range nodes {
			open := n.Dir && isOpen(n.Path)
			rows = append(rows, Row{Node: n, Depth: depth, Open: open})
			if open {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(tree, 0)
	return rows
}
