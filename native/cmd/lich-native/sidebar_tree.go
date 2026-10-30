package main

import (
	"maps"
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
// is in the block hangs from it, a level deeper each generation, in stored
// order, and a child in a checkout other than its mother's hangs from a
// checkout row of its own inside her group. under is the checkout a row above
// the block draws, "" for none; folded reports a mother's stored fold.
func sessionRows(sessions []sessionView, under string, folded func(id string) bool) []sessionRow {
	in := make(map[string]bool, len(sessions))
	for _, sv := range sessions {
		in[sv.ID] = true
	}
	children := map[string][]sessionView{}
	var roots []sessionView
	for _, sv := range sessions {
		if up := sv.OriginSessionID; in[up] && up != sv.ID {
			children[up] = append(children[up], sv)
			continue
		}
		roots = append(roots, sv)
	}
	t := tree{children: children, folded: folded}
	base := 0
	if under != "" {
		base = 1
	}
	for i, root := range roots {
		t.walk(root, base, nil, i == len(roots)-1, under != "", map[string]bool{})
	}
	return t.rows
}

// blockRows is a block's rows. A folder or the pinned block only gathers
// sessions, so while the project draws checkout rows it draws them too: each
// checkout its trees' roots run in, as a row they hang from, in the order its
// first root sits.
func blockRows(b blockView, sessions []sessionView) []sessionRow {
	folded := func(id string) bool { return b.motherFolds[id] }
	if !b.byCheckout {
		return sessionRows(sessions, b.under, folded)
	}
	in := make(map[string]sessionView, len(sessions))
	for _, sv := range sessions {
		in[sv.ID] = sv
	}
	rootPath := func(sv sessionView) string {
		seen := map[string]bool{}
		for {
			up, ok := in[sv.OriginSessionID]
			if !ok || seen[up.ID] {
				return sv.path
			}
			seen[up.ID] = true
			sv = up
		}
	}
	var paths []string
	groups := map[string][]sessionView{}
	for _, sv := range sessions {
		path := rootPath(sv)
		if _, ok := groups[path]; !ok {
			paths = append(paths, path)
		}
		groups[path] = append(groups[path], sv)
	}
	var rows []sessionRow
	for _, path := range paths {
		rows = append(rows, sessionRow{kind: rowNestedCheckout, key: "__checkout__:" + b.key + ":" + path, path: path, stub: true})
		rows = append(rows, sessionRows(groups[path], path, folded)...)
	}
	return rows
}

// tree builds sessionRows' rows depth first.
type tree struct {
	children map[string][]sessionView
	folded   func(id string) bool
	rows     []sessionRow
}

// descendants is everything under id, its children's children included.
func (t *tree) descendants(id string, seen map[string]bool) []sessionView {
	var out []sessionView
	for _, kid := range t.children[id] {
		if seen[kid.ID] {
			continue
		}
		seen[kid.ID] = true
		out = append(append(out, kid), t.descendants(kid.ID, seen)...)
	}
	return out
}

// walk adds sv at depth and, unless it is folded, its group under it. seen
// guards against an origin cycle.
func (t *tree) walk(sv sessionView, depth int, lanes []bool, tail, hideGit bool, seen map[string]bool) {
	if seen[sv.ID] {
		return
	}
	seen[sv.ID] = true
	all := t.descendants(sv.ID, maps.Clone(seen))
	row := sessionRow{kind: rowCard, key: sv.ID, sv: sv, depth: depth, lanes: lanes, tail: tail, kids: len(all), hideGit: hideGit}
	row.folded = len(all) > 0 && t.folded(sv.ID)
	if row.folded {
		row.mark = treeMark(all)
	}
	row.stub = len(all) > 0 && !row.folded
	t.rows = append(t.rows, row)
	if row.stub {
		t.group(sv, depth+1, childLanes(row), seen)
	}
}

// childLanes are the lanes of a row's children: its own, and its parent's
// line when that runs on past it.
func childLanes(r sessionRow) []bool {
	if r.depth == 0 {
		return nil
	}
	return append(slices.Clone(r.lanes), !r.tail)
}

// group lays a mother's children out at depth: the ones in her checkout
// first-come, and each other checkout as a row its children hang from, at the
// slot its first child takes.
func (t *tree) group(mother sessionView, depth int, lanes []bool, seen map[string]bool) {
	type item struct {
		path     string
		sessions []sessionView
	}
	var items []item
	for _, sv := range t.children[mother.ID] {
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
	for i, it := range items {
		tail := i == len(items)-1
		if it.path == "" {
			t.walk(it.sessions[0], depth, lanes, tail, true, seen)
			continue
		}
		row := sessionRow{kind: rowNestedCheckout, key: "__checkout__:" + mother.ID + ":" + it.path, path: it.path, depth: depth, lanes: lanes, tail: tail, stub: true}
		t.rows = append(t.rows, row)
		for j, sv := range it.sessions {
			t.walk(sv, depth+1, childLanes(row), j == len(it.sessions)-1, true, seen)
		}
	}
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
