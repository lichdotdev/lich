package main

import (
	"fmt"
	"slices"
	"strings"
)

// A browser tab is a layout of two panes, a session and the page its browser
// shows, and nothing else joins it. The page's pane is a leaf like any other,
// named after its session with browserPanePrefix, so rotating, dragging the
// divider, zooming and storing the tab are the layout's own.

const browserPanePrefix = "browser:"

func browserPaneOf(session string) string { return browserPanePrefix + session }

// browserOwner is the session whose page pane is, false for a session's pane.
func browserOwner(pane string) (string, bool) {
	return strings.CutPrefix(pane, browserPanePrefix)
}

// paneSession is the session a pane belongs to: itself, or a page's owner.
func paneSession(pane string) string {
	if owner, ok := browserOwner(pane); ok {
		return owner
	}
	return pane
}

// panes are every leaf of the tab in reading order, pages included.
func (t *layoutTab) panes() []string {
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

// browserOf is the session whose page this tab shows, false for a tab
// without one.
func (t *layoutTab) browserOf() (string, bool) {
	for _, pane := range t.panes() {
		if owner, ok := browserOwner(pane); ok {
			return owner, true
		}
	}
	return "", false
}

// openBrowser makes session's browser tab, its terminal at the left and its
// page at the right, named name and focused on the page, and answers its id.
// session may not be in a layout already: moving it out is the caller's to
// ask about.
func (ls *layoutTabs) openBrowser(name, session string) string {
	ls.seq++
	page := browserPaneOf(session)
	t := layoutTab{ID: fmt.Sprintf("layout-%d", ls.seq), Name: name, Root: &paneNode{Session: session}, Focus: page}
	t.split(session, page, edgeRight)
	ls.tabs = append(ls.tabs, t)
	return t.ID
}

// browserTab is the index of session's browser tab, -1 for none.
func (ls *layoutTabs) browserTab(session string) int {
	return slices.IndexFunc(ls.tabs, func(t layoutTab) bool {
		return findLeaf(t.Root, browserPaneOf(session)) != nil
	})
}

// dropBrowser takes session's page out of its tab, which leaves the session
// alone and so dissolves the tab. It reports whether there was one.
func (ls *layoutTabs) dropBrowser(session string) bool {
	i := ls.browserTab(session)
	if i < 0 {
		return false
	}
	ls.tabs = slices.Delete(ls.tabs, i, i+1)
	return true
}

// locked reports whether layout i takes no pane from outside it: a browser
// tab holds its session and that session's page only.
func (ls *layoutTabs) locked(i int) bool {
	if i < 0 {
		return false
	}
	_, ok := ls.tabs[i].browserOf()
	return ok
}
