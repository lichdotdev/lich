package main

import "path"

// A checkout's file in the stage editor: the node its tree hangs under the
// checkout, one per checkout, named after the file the editor holds, and
// whether the editor is what the stage shows in place of a session.

// editorNode is what a checkout's editor node shows.
type editorNode struct {
	rel   string
	dirty bool
	// shown is the editor on the stage, which lights its node.
	shown bool
}

func (n editorNode) name() string { return path.Base(n.rel) }

// openEditor puts checkout's editor on the stage, holding rel.
func (m *model) openEditor(checkout, rel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.editors[checkout]
	if n.rel != rel {
		n = editorNode{rel: rel}
	}
	if m.editors == nil {
		m.editors = map[string]editorNode{}
	}
	m.editors[checkout] = n
	m.showEditorLocked(checkout)
}

// showEditor brings checkout's editor back to the stage, if it has one.
func (m *model) showEditor(checkout string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.editors[checkout]; ok {
		m.showEditorLocked(checkout)
	}
}

func (m *model) showEditorLocked(checkout string) {
	m.editorOn, m.console, m.shownLayout = checkout, "", ""
}

// markEditorDirty records whether checkout's editor holds unsaved changes.
func (m *model) markEditorDirty(checkout string, dirty bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.editors[checkout]; ok && n.dirty != dirty {
		n.dirty = dirty
		m.editors[checkout] = n
	}
}

// closeEditor takes checkout's node out of the tree, and its editor off the
// stage.
func (m *model) closeEditor(checkout string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.editors, checkout)
	if m.editorOn == checkout {
		m.editorOn = ""
	}
}

// editorShown is the checkout whose editor is on the stage, "" for none.
func (m *model) editorShown() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.editorOn
}

// editorOf is checkout's node. Callers hold m.mu.
func (m *model) editorOf(checkout string) (editorNode, bool) {
	n, ok := m.editors[checkout]
	n.shown = ok && m.editorOn == checkout
	return n, ok
}
