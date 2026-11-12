package main

import "strings"

// A page's pane is a leaf of the layout the stage draws, named after its
// session with browserPanePrefix, so the divider, the zoom and the focus are
// a layout's own. No stored layout holds one (pages.go).

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
