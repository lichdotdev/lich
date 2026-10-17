package main

// The Code tab's tree, the pure half of lib/git/file-tree.ts and FileTree.tsx:
// flat repo-relative paths nested into folders, narrowed by the name filter,
// and flattened into the rows on screen.

import (
	"slices"
	"strings"

	"github.com/lichdotdev/godemirror"

	"github.com/omartelo/lich/native/i18n"
)

type treeNode struct {
	name     string // what the row shows: the last segment, or a collapsed chain
	path     string // repo-relative, the row's id for folds and ReadFile
	dir      bool
	children []*treeNode
}

// buildTree nests paths into folders, folders before files and each group
// sorted case-insensitively, then collapses the chains of single folders.
func buildTree(paths []string) []*treeNode {
	var root []*treeNode
	for _, p := range paths {
		level := &root
		prefix := ""
		parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
		for i, name := range parts {
			prefix = strings.TrimPrefix(prefix+"/"+name, "/")
			dir := i < len(parts)-1
			at := slices.IndexFunc(*level, func(n *treeNode) bool { return n.name == name && n.dir == dir })
			if at < 0 {
				*level = append(*level, &treeNode{name: name, path: prefix, dir: dir})
				at = len(*level) - 1
			}
			level = &(*level)[at].children
		}
	}
	sortTree(root)
	return collapseChains(root)
}

func sortTree(nodes []*treeNode) {
	slices.SortStableFunc(nodes, func(a, b *treeNode) int {
		if a.dir != b.dir {
			if a.dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
	for _, n := range nodes {
		sortTree(n.children)
	}
}

// collapseChains merges a folder holding only one folder into one row,
// keeping the deepest path as its id: src/main/java reads as one row, not
// three levels of indentation before the first file.
func collapseChains(nodes []*treeNode) []*treeNode {
	out := make([]*treeNode, len(nodes))
	for i, n := range nodes {
		merged := *n
		for len(merged.children) == 1 && merged.children[0].dir {
			only := merged.children[0]
			merged = treeNode{name: merged.name + "/" + only.name, path: only.path, dir: true, children: only.children}
		}
		merged.children = collapseChains(merged.children)
		out[i] = &merged
	}
	return out
}

func filterPaths(paths []string, query string) []string {
	if strings.TrimSpace(query) == "" {
		return paths
	}
	var out []string
	for _, p := range paths {
		if matchesQuery(p, query) {
			out = append(out, p)
		}
	}
	return out
}

type lineDelta struct{ added, deleted int }

// diffStatsByPath keys each changed file's counts by its current path.
func diffStatsByPath(files []godemirror.FilePatch) map[string]lineDelta {
	out := make(map[string]lineDelta, len(files))
	for _, f := range files {
		out[f.Path] = lineDelta{f.Added, f.Deleted}
	}
	return out
}

// withDirStats adds every folder holding a changed file, summed, so a closed
// folder says where the diff is. It sums the tree it is given, so a filtered
// tree counts only what the filter left.
func withDirStats(tree []*treeNode, files map[string]lineDelta) map[string]lineDelta {
	out := make(map[string]lineDelta, len(files))
	for k, v := range files {
		out[k] = v
	}
	var walk func(n *treeNode) (lineDelta, bool)
	walk = func(n *treeNode) (lineDelta, bool) {
		if !n.dir {
			d, ok := files[n.path]
			return d, ok
		}
		var sum lineDelta
		found := false
		for _, c := range n.children {
			if d, ok := walk(c); ok {
				sum.added, sum.deleted, found = sum.added+d.added, sum.deleted+d.deleted, true
			}
		}
		if found {
			out[n.path] = sum
		}
		return sum, found
	}
	for _, n := range tree {
		walk(n)
	}
	return out
}

// treeRow is one row on screen.
type treeRow struct {
	node  *treeNode
	depth int
	open  bool
}

// visibleRows flattens the tree to the rows on screen: a folder's children
// only while it is open.
func visibleRows(tree []*treeNode, isOpen func(path string) bool) []treeRow {
	var rows []treeRow
	var walk func(nodes []*treeNode, depth int)
	walk = func(nodes []*treeNode, depth int) {
		for _, n := range nodes {
			open := n.dir && isOpen(n.path)
			rows = append(rows, treeRow{node: n, depth: depth, open: open})
			if open {
				walk(n.children, depth+1)
			}
		}
	}
	walk(tree, 0)
	return rows
}

// treeFootnote is what a listing that is not the whole folder owes the
// reader: the directories the walk stepped over, then whether it was cut.
func treeFootnote(cut bool, hidden []string) string {
	var parts []string
	if len(hidden) > 0 {
		parts = append(parts, i18n.T("git.fileTree.hidden", i18n.Args{"names": strings.Join(hidden, ", ")}))
	}
	if cut {
		parts = append(parts, i18n.T("git.fileTree.cut"))
	}
	return strings.Join(parts, " ")
}
