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
	// rowBrowser is a session's running browser, the first row under it.
	rowBrowser
	// rowEditor is a checkout's file in the stage editor, the last row under
	// the checkout.
	rowEditor
	// rowCheckout is the label over the roots of a block that share a
	// checkout, when there are two or more of them.
	rowCheckout
)

// sessionRow is one line of a block's tree.
type sessionRow struct {
	kind   sessionRowKind
	key    string
	sv     sessionView // rowCard, and rowBrowser's session
	path   string      // rowEditor: the checkout
	editor editorNode  // rowEditor
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
	// branchSaid is a card whose branch a row above already shows: its
	// checkout's label, or its mother in the same checkout.
	branchSaid bool
	// motion is the row's share of a run opening or closing it, nil at rest.
	motion *rowMotion
}

// sessionRows lays a block's sessions out as trees: every session whose origin
// is in the block hangs from it, a level deeper each generation, in stored
// order, with its browser and the editor it holds under it. folded reports a
// mother's stored fold.
func sessionRows(sessions []sessionView, folded func(id string) bool, editors map[string]editorNode) []sessionRow {
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
	t := tree{children: children, folded: folded, editors: editors}
	roots = byCheckout(roots)
	shared := map[string]int{}
	for _, root := range roots {
		shared[root.path]++
	}
	for i, root := range roots {
		grouped := shared[root.path] > 1
		if grouped && (i == 0 || roots[i-1].path != root.path) {
			t.rows = append(t.rows, sessionRow{kind: rowCheckout, key: checkoutRowPrefix + root.path, sv: root, path: root.path})
		}
		t.walk(root, 0, nil, i == len(roots)-1, grouped, map[string]bool{})
	}
	return t.rows
}

const checkoutRowPrefix = "__checkout__:"

// byCheckout is roots with each checkout's sessions brought together where
// the first of them stands, in stored order otherwise.
func byCheckout(roots []sessionView) []sessionView {
	at := map[string]int{}
	var groups [][]sessionView
	for _, sv := range roots {
		i, ok := at[sv.path]
		if !ok {
			i = len(groups)
			at[sv.path] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], sv)
	}
	return slices.Concat(groups...)
}

const editorRowPrefix = "__editor__:"

// editorRow is checkout's editor node, the last row under it.
func editorRow(key, checkout string, n editorNode, depth int, lanes []bool) sessionRow {
	return sessionRow{kind: rowEditor, key: key, path: checkout, editor: n, depth: depth, lanes: lanes, tail: true}
}

// blockRows is a block's rows: its sessions, each with its browser and its
// editor under it.
func blockRows(b blockView, sessions []sessionView) []sessionRow {
	folded := func(id string) bool { return b.motherFolds[id] }
	return withoutCarriedBrowser(sessionRows(sessions, folded, b.editors), b.carried)
}

// withoutCarriedBrowser is rows without the browser of the session in the
// air, so its card moves alone in the list as a mother folded over her
// children does; the browser opens under it again where it lands.
func withoutCarriedBrowser(rows []sessionRow, carried string) []sessionRow {
	at := slices.IndexFunc(rows, func(r sessionRow) bool { return r.kind == rowBrowser && carried != "" && r.sv.ID == carried })
	if at < 0 {
		return rows
	}
	if at > 0 && rows[at].tail {
		rows[at-1].stub = false
	}
	return slices.Delete(rows, at, at+1)
}

// tree builds sessionRows' rows depth first.
type tree struct {
	children map[string][]sessionView
	folded   func(id string) bool
	editors  map[string]editorNode
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

// walk adds sv at depth and, unless it is folded, its browser and its group
// under it. branchSaid is a row above showing sv's branch already. seen
// guards against an origin cycle.
func (t *tree) walk(sv sessionView, depth int, lanes []bool, tail, branchSaid bool, seen map[string]bool) {
	if seen[sv.ID] {
		return
	}
	seen[sv.ID] = true
	all := t.descendants(sv.ID, maps.Clone(seen))
	row := sessionRow{kind: rowCard, key: sv.ID, sv: sv, depth: depth, lanes: lanes, tail: tail, kids: len(all),
		folded: len(all) > 0 && t.folded(sv.ID), branchSaid: branchSaid}
	if row.folded {
		row.mark = treeMark(all)
	}
	open := len(all) > 0 && !row.folded
	browser := sv.browser && !row.folded
	editor, hasEditor := t.editors[sv.ID]
	hasEditor = hasEditor && !row.folded
	row.stub = open || browser || hasEditor
	t.rows = append(t.rows, row)
	if browser {
		t.rows = append(t.rows, sessionRow{kind: rowBrowser, key: browserRowKey(sv.ID), sv: sv, depth: depth + 1, lanes: childLanes(row), tail: !open && !hasEditor})
	}
	if hasEditor {
		t.rows = append(t.rows, editorRow(editorRowPrefix+sv.ID, sv.path, editor, depth+1, childLanes(row)))
	}
	if open {
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

// group lays a mother's children out at depth, in stored order.
func (t *tree) group(mother sessionView, depth int, lanes []bool, seen map[string]bool) {
	kids := t.children[mother.ID]
	for i, sv := range kids {
		t.walk(sv, depth, lanes, i == len(kids)-1, sv.path == mother.path, seen)
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
