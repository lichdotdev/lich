package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/backend"
)

// motherKeyPrefix keys a session's fold among the block keys, which a path or
// a folder name cannot collide with.
const motherKeyPrefix = "__session__:"

func motherKey(id string) string { return motherKeyPrefix + id }

// rootsOf maps every session to the oldest ancestor its origin chain reaches
// among sessions: an origin closed or in another project ends the chain, so a
// child outlives its mother as a root of its own.
func rootsOf(sessions []backend.Session) map[string]string {
	origin := make(map[string]string, len(sessions))
	for _, s := range sessions {
		origin[s.ID] = s.OriginSessionID
	}
	roots := make(map[string]string, len(sessions))
	for _, s := range sessions {
		root, seen := s.ID, map[string]bool{s.ID: true}
		for {
			up, ok := origin[root]
			if !ok || up == "" || seen[up] {
				break
			}
			if _, open := origin[up]; !open {
				break
			}
			seen[up] = true
			root = up
		}
		roots[s.ID] = root
	}
	return roots
}

// placedBy is the session whose placement decides s's block: s itself when it
// was pinned or filed on its own, else the root of its tree, so a child sits
// with its mother whatever checkout it runs in.
func placedBy(s backend.Session, byID map[string]backend.Session, roots map[string]string) backend.Session {
	if s.Pinned || s.Folder != "" {
		return s
	}
	if root, ok := byID[roots[s.ID]]; ok {
		return root
	}
	return s
}

// sessionRowKind is what a tree row draws.
type sessionRowKind int

const (
	rowCard sessionRowKind = iota
	// rowNestedCheckout is a checkout inside a mother's group: a child running in
	// another checkout than its mother's hangs from it.
	rowNestedCheckout
)

// sessionRow is one line of a block's tree.
type sessionRow struct {
	kind sessionRowKind
	key  string
	sv   sessionView // rowCard
	path string      // rowNestedCheckout: the checkout
	// depth is the row's level; its elbow leaves the ring of level depth-1.
	depth int
	// lanes says, per level under depth-1, whether that level's line runs on
	// past this row to a later sibling.
	lanes []bool
	tail  bool // the last row under its parent: the elbow stops here
	// stub is a line from the row's ring down to its first child.
	stub   bool
	kids   int // sessions under a mother, drawn or folded
	folded bool
	mark   string // a folded mother's markWait / markDone
	// hideGit is a session row under a checkout row, which already shows the
	// pull request and the changes.
	hideGit bool
}

// sessionRows lays a block's sessions out as trees: every session whose origin
// is in the block hangs from it, flattened one level under its root in stored
// order, and a child in a checkout other than its root's hangs from a
// checkout row of its own. under is the checkout a row above the block draws,
// "" for none; folded reports a mother's stored fold.
func sessionRows(sessions []sessionView, under string, folded func(id string) bool) []sessionRow {
	in := make(map[string]bool, len(sessions))
	for _, sv := range sessions {
		in[sv.ID] = true
	}
	parent := func(sv sessionView) string {
		if in[sv.OriginSessionID] && sv.OriginSessionID != sv.ID {
			return sv.OriginSessionID
		}
		return ""
	}
	byID := make(map[string]sessionView, len(sessions))
	for _, sv := range sessions {
		byID[sv.ID] = sv
	}
	rootOf := func(sv sessionView) string {
		seen := map[string]bool{}
		for parent(sv) != "" && !seen[sv.ID] {
			seen[sv.ID] = true
			sv = byID[parent(sv)]
		}
		return sv.ID
	}
	var roots []sessionView
	descendants := map[string][]sessionView{}
	for _, sv := range sessions {
		root := rootOf(sv)
		if root == sv.ID {
			roots = append(roots, sv)
			continue
		}
		descendants[root] = append(descendants[root], sv)
	}
	base := 0
	if under != "" {
		base = 1
	}
	var rows []sessionRow
	for i, root := range roots {
		kids := descendants[root.ID]
		row := sessionRow{kind: rowCard, key: root.ID, sv: root, depth: base, tail: i == len(roots)-1, kids: len(kids), hideGit: under != ""}
		row.folded = len(kids) > 0 && folded(root.ID)
		if row.folded {
			row.mark = treeMark(kids)
		}
		row.stub = len(kids) > 0 && !row.folded
		rows = append(rows, row)
		if row.stub {
			lanes := []bool{}
			if base > 0 {
				lanes = append(lanes, !row.tail)
			}
			rows = append(rows, childRows(root, kids, base+1, lanes)...)
		}
	}
	return rows
}

// childRows is a mother's group: children in her checkout first-come, and
// each other checkout as a row its children hang from, at the slot its first
// child takes.
func childRows(mother sessionView, kids []sessionView, depth int, lanes []bool) []sessionRow {
	type item struct {
		path     string
		sessions []sessionView
	}
	var items []item
	for _, sv := range kids {
		if sv.path == mother.path {
			items = append(items, item{sessions: []sessionView{sv}})
			continue
		}
		if i := slices.IndexFunc(items, func(it item) bool { return it.path == sv.path }); i >= 0 {
			items[i].sessions = append(items[i].sessions, sv)
			continue
		}
		items = append(items, item{path: sv.path, sessions: []sessionView{sv}})
	}
	var rows []sessionRow
	for i, it := range items {
		tail := i == len(items)-1
		if it.path == "" {
			rows = append(rows, sessionRow{kind: rowCard, key: it.sessions[0].ID, sv: it.sessions[0], depth: depth, lanes: lanes, tail: tail, hideGit: true})
			continue
		}
		rows = append(rows, sessionRow{kind: rowNestedCheckout, key: "__checkout__:" + mother.ID + ":" + it.path, path: it.path, depth: depth, lanes: lanes, tail: tail, stub: true})
		inner := append(slices.Clone(lanes), !tail)
		for j, sv := range it.sessions {
			rows = append(rows, sessionRow{kind: rowCard, key: sv.ID, sv: sv, depth: depth + 1, lanes: inner, tail: j == len(it.sessions)-1, hideGit: true})
		}
	}
	return rows
}

// treeMark is what a folded mother says about her hidden children: one
// waiting on the user outranks a finished turn nobody read.
func treeMark(kids []sessionView) string {
	mark := markNone
	for _, sv := range kids {
		if sv.status == statusWaiting {
			return markWait
		}
		if sv.unread {
			mark = markDone
		}
	}
	return mark
}
