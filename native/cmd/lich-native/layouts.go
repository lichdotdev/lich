package main

import (
	"encoding/json"
	"fmt"
	"slices"
)

// A layout is a tab of panes the user splits as they like: a binary tree
// whose leaves are sessions, from any open project. It belongs to the window,
// not to a project, and a session sits in at most one pane of one layout,
// since its PTY has one size.

// layoutsKey is the backend setting the layouts persist under.
const layoutsKey = "lich.native.layouts"

// splitAxis is how a node divides its area: side by side or stacked.
type splitAxis string

const (
	splitRow    splitAxis = "row"
	splitColumn splitAxis = "column"
)

// paneEdge is where a session lands against a pane: one of its four edges,
// or its middle, which replaces the session there.
type paneEdge int

const (
	edgeLeft paneEdge = iota
	edgeRight
	edgeTop
	edgeBottom
	edgeCenter
)

// The ratio a divider can be dragged to; the pixel floor is the stage's
// (paneMinWidth), this only keeps a stored ratio from collapsing a side.
const (
	ratioMin = 0.05
	ratioMax = 0.95
)

// paneNode is a leaf when Session is set, else a split of First and Second
// along Axis, First taking Ratio of the area.
type paneNode struct {
	Session string    `json:"session,omitempty"`
	Axis    splitAxis `json:"axis,omitempty"`
	Ratio   float64   `json:"ratio,omitempty"`
	First   *paneNode `json:"first,omitempty"`
	Second  *paneNode `json:"second,omitempty"`
}

func (n *paneNode) leaf() bool { return n.Session != "" }

// layoutTab is one layout: its tree, the session it last had in focus and
// whether that pane is zoomed over the others.
type layoutTab struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Root   *paneNode `json:"root"`
	Focus  string    `json:"focus"`
	Zoomed bool      `json:"zoomed,omitempty"`
}

// sessions are the tab's sessions in reading order, left to right and top to
// bottom.
func (t *layoutTab) sessions() []string {
	var out []string
	var walk func(n *paneNode)
	walk = func(n *paneNode) {
		if n.leaf() {
			out = append(out, n.Session)
			return
		}
		walk(n.First)
		walk(n.Second)
	}
	walk(t.Root)
	return out
}

func (t *layoutTab) holds(id string) bool { return slices.Contains(t.sessions(), id) }

// split puts session against target's edge, halving target's pane; the
// middle replaces target instead. It reports false when target is not in the
// tab.
func (t *layoutTab) split(target, session string, edge paneEdge) bool {
	n := findLeaf(t.Root, target)
	if n == nil {
		return false
	}
	if edge == edgeCenter {
		n.Session = session
		return true
	}
	old, added := &paneNode{Session: target}, &paneNode{Session: session}
	axis := splitRow
	if edge == edgeTop || edge == edgeBottom {
		axis = splitColumn
	}
	*n = paneNode{Axis: axis, Ratio: 0.5, First: old, Second: added}
	if edge == edgeLeft || edge == edgeTop {
		n.First, n.Second = added, old
	}
	return true
}

// remove takes session's pane out, its sibling taking the parent's area. It
// reports whether the tab still holds any pane.
func (t *layoutTab) remove(session string) bool {
	if t.Root.leaf() {
		if t.Root.Session == session {
			t.Root = nil
			return false
		}
		return true
	}
	var walk func(n *paneNode) bool
	walk = func(n *paneNode) bool {
		if n.leaf() {
			return false
		}
		for _, pair := range [][2]*paneNode{{n.First, n.Second}, {n.Second, n.First}} {
			if pair[0].leaf() && pair[0].Session == session {
				*n = *pair[1]
				return true
			}
		}
		return walk(n.First) || walk(n.Second)
	}
	walk(t.Root)
	if t.Focus == session {
		t.Focus = t.sessions()[0]
	}
	return true
}

// move puts source's pane against target's edge, taking it out of where it
// was; the middle swaps the two. It reports false when either is not in the
// tab or they are the same pane.
func (t *layoutTab) move(source, target string, edge paneEdge) bool {
	if source == target || findLeaf(t.Root, source) == nil || findLeaf(t.Root, target) == nil {
		return false
	}
	if edge == edgeCenter {
		findLeaf(t.Root, source).Session, findLeaf(t.Root, target).Session = "\x00swap", source
		findLeaf(t.Root, "\x00swap").Session = target
		return true
	}
	t.remove(source)
	t.split(target, source, edge)
	t.Focus = source
	return true
}

// rotate turns the split holding session's pane from side by side to stacked
// or back, and reports false for a pane alone in the tab.
func (t *layoutTab) rotate(session string) bool {
	parent := parentOf(t.Root, session)
	if parent == nil {
		return false
	}
	if parent.Axis == splitRow {
		parent.Axis = splitColumn
	} else {
		parent.Axis = splitRow
	}
	return true
}

// stackUnder puts ids under target's pane, the column of them all sharing
// target's area evenly. It reports false when target is not in the tab.
func (t *layoutTab) stackUnder(target string, ids []string) bool {
	n := findLeaf(t.Root, target)
	if n == nil {
		return false
	}
	*n = *stackColumn(append([]string{target}, ids...))
	return true
}

// columnBeside halves target's pane, a column of ids taking its right half.
// It reports false when target is not in the tab.
func (t *layoutTab) columnBeside(target string, ids []string) bool {
	n := findLeaf(t.Root, target)
	if n == nil {
		return false
	}
	*n = paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Session: target}, Second: stackColumn(ids)}
	return true
}

// stackColumn is ids stacked top to bottom, each taking an even share.
func stackColumn(ids []string) *paneNode {
	if len(ids) == 1 {
		return &paneNode{Session: ids[0]}
	}
	return &paneNode{Axis: splitColumn, Ratio: 1 / float64(len(ids)), First: &paneNode{Session: ids[0]}, Second: stackColumn(ids[1:])}
}

// parentOf is the split whose side is session's leaf, nil for none.
func parentOf(n *paneNode, session string) *paneNode {
	if n == nil || n.leaf() {
		return nil
	}
	for _, side := range []*paneNode{n.First, n.Second} {
		if side.leaf() && side.Session == session {
			return n
		}
	}
	if p := parentOf(n.First, session); p != nil {
		return p
	}
	return parentOf(n.Second, session)
}

func findLeaf(n *paneNode, session string) *paneNode {
	if n == nil {
		return nil
	}
	if n.leaf() {
		if n.Session == session {
			return n
		}
		return nil
	}
	if f := findLeaf(n.First, session); f != nil {
		return f
	}
	return findLeaf(n.Second, session)
}

// minSize is the smallest area the tree fits in, given the smallest pane and
// the gap between two panes.
func minSize(n *paneNode, paneW, paneH, gap int) (w, h int) {
	if n.leaf() {
		return paneW, paneH
	}
	fw, fh := minSize(n.First, paneW, paneH, gap)
	sw, sh := minSize(n.Second, paneW, paneH, gap)
	if n.Axis == splitRow {
		return fw + gap + sw, max(fh, sh)
	}
	return max(fw, sw), fh + gap + sh
}

func clampRatio(r float64) float64 { return min(ratioMax, max(ratioMin, r)) }

// layoutTabs is every layout of the window, in strip order.
type layoutTabs struct {
	tabs []layoutTab
	// seq numbers the ids handed out; restored past the highest stored.
	seq int
}

// of is the index of the tab holding session, -1 for none.
func (ls *layoutTabs) of(session string) int {
	return slices.IndexFunc(ls.tabs, func(t layoutTab) bool { return t.holds(session) })
}

func (ls *layoutTabs) byID(id string) *layoutTab {
	if i := slices.IndexFunc(ls.tabs, func(t layoutTab) bool { return t.ID == id }); i >= 0 {
		return &ls.tabs[i]
	}
	return nil
}

// open makes a layout of first and second, second against first's edge,
// named name, and answers its id. Neither may be in a layout already: moving
// one out is the caller's to ask about.
func (ls *layoutTabs) open(name, first, second string, edge paneEdge) string {
	ls.seq++
	t := layoutTab{ID: fmt.Sprintf("layout-%d", ls.seq), Name: name, Root: &paneNode{Session: first}, Focus: second}
	t.split(first, second, edge)
	ls.tabs = append(ls.tabs, t)
	return t.ID
}

// openBeside makes a layout of left with a column of ids at its right,
// named name and focused on left, and answers its id. As with open, none of
// them may be in a layout already.
func (ls *layoutTabs) openBeside(name, left string, column []string) string {
	ls.seq++
	t := layoutTab{ID: fmt.Sprintf("layout-%d", ls.seq), Name: name, Root: &paneNode{Session: left}, Focus: left}
	t.columnBeside(left, column)
	ls.tabs = append(ls.tabs, t)
	return t.ID
}

// take removes session from whichever layout holds it, reporting whether
// one did. A layout left with a single pane is no layout any more: it goes,
// and left is the session that pane showed, "" while the layout stays.
func (ls *layoutTabs) take(session string) (left string, ok bool) {
	i := ls.of(session)
	if i < 0 {
		return "", false
	}
	t := &ls.tabs[i]
	t.remove(session)
	if t.Root.leaf() {
		left = t.Root.Session
		ls.tabs = slices.Delete(ls.tabs, i, i+1)
	}
	return left, true
}

func (ls *layoutTabs) close(id string) {
	ls.tabs = slices.DeleteFunc(ls.tabs, func(t layoutTab) bool { return t.ID == id })
}

// prune drops every pane whose session is not live, which is how a session
// closed while the window was away leaves the layouts it was in.
func (ls *layoutTabs) prune(live func(id string) bool) bool {
	changed := false
	for _, t := range slices.Clone(ls.tabs) {
		for _, id := range t.sessions() {
			if !live(id) {
				_, took := ls.take(id)
				changed = took || changed
			}
		}
	}
	return changed
}

func (ls *layoutTabs) marshal() (string, error) {
	b, err := json.Marshal(ls.tabs)
	return string(b), err
}

// parseLayouts reads the stored layouts. A session in two panes keeps the
// first; a tree with a broken node is an error, since repairing it would
// guess which half the user meant.
func parseLayouts(raw string) (layoutTabs, error) {
	var ls layoutTabs
	if raw == "" {
		return ls, nil
	}
	if err := json.Unmarshal([]byte(raw), &ls.tabs); err != nil {
		return layoutTabs{}, fmt.Errorf("%s: %w", layoutsKey, err)
	}
	seen := map[string]bool{}
	for i := range ls.tabs {
		t := &ls.tabs[i]
		if err := validNode(t.Root); err != nil {
			return layoutTabs{}, fmt.Errorf("%s: layout %q: %w", layoutsKey, t.ID, err)
		}
		var n int
		if _, err := fmt.Sscanf(t.ID, "layout-%d", &n); err == nil {
			ls.seq = max(ls.seq, n)
		}
		for _, id := range t.sessions() {
			if seen[id] && !t.remove(id) {
				break
			}
			seen[id] = true
		}
		if t.Root != nil && !t.holds(t.Focus) {
			t.Focus = t.sessions()[0]
		}
	}
	ls.tabs = slices.DeleteFunc(ls.tabs, func(t layoutTab) bool { return t.Root == nil || t.Root.leaf() })
	return ls, nil
}

func validNode(n *paneNode) error {
	switch {
	case n == nil:
		return fmt.Errorf("missing node")
	case n.leaf():
		return nil
	case n.Axis != splitRow && n.Axis != splitColumn:
		return fmt.Errorf("split axis %q, want row or column", n.Axis)
	}
	n.Ratio = clampRatio(n.Ratio)
	if err := validNode(n.First); err != nil {
		return err
	}
	return validNode(n.Second)
}
