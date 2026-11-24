package main

// The window's side of the stage editor: one editor per checkout, what puts
// a file in one (the Code tab, a path Ctrl+clicked in a terminal, an agent's
// show_code), the node's Show and Close, and the saves leaving an editor
// makes: another file, another session or node, Close, the window losing
// the focus. Only a save that failed, or a disk change nobody decided on,
// asks before its text is left.

import (
	"path"
	"path/filepath"
	"slices"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

type stageEditors struct {
	client     *backend.Client
	invalidate func()
	family     string
	size       unit.Sp
	// dirty hears an editor start or stop holding unsaved changes, and saved
	// a save that landed, each with the editor's checkout.
	dirty      func(checkout string, dirty bool)
	saved      func(checkout string)
	byCheckout map[string]*fileEditor
	ask        unsavedAsk
	// shown is the checkout whose editor was on the stage last frame.
	shown string
	// pending are the checkouts a session wrote to since the last frame,
	// heard off the UI goroutine.
	mu      sync.Mutex
	pending map[string]bool
}

// unsavedAsk is Save / Discard / Cancel over an editor's unsaved changes,
// with then what Save or Discard leads to.
type unsavedAsk struct {
	open                  bool
	editor                *fileEditor
	then                  func()
	state                 ui.DialogState
	save, discard, cancel ui.Clickable
}

// of is checkout's editor, made the first time.
func (s *stageEditors) of(checkout string) *fileEditor {
	if f, ok := s.byCheckout[checkout]; ok {
		return f
	}
	if s.byCheckout == nil {
		s.byCheckout = map[string]*fileEditor{}
	}
	f := newFileEditor(s.client, s.invalidate, s.family, s.size, checkout)
	f.onDirty = func() { s.dirty(checkout, f.dirty) }
	f.onSaved = func() { s.saved(checkout) }
	s.byCheckout[checkout] = f
	return f
}

func (s *stageEditors) setFont(family string, size unit.Sp) {
	s.family, s.size = family, size
	for _, f := range s.byCheckout {
		f.setFont(family, size)
	}
}

// touched hears a session write to checkout; the editor reads its file
// again on the next frame.
func (s *stageEditors) touched(checkout string) {
	s.mu.Lock()
	if s.pending == nil {
		s.pending = map[string]bool{}
	}
	s.pending[checkout] = true
	s.mu.Unlock()
	s.invalidate()
}

// takeTouched has the editor of every checkout touched since the last frame
// read its file again.
func (s *stageEditors) takeTouched() {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()
	for checkout := range pending {
		if f, ok := s.byCheckout[checkout]; ok {
			f.recheck()
		}
	}
}

// recheckAll has every editor read its file again: the window came back,
// and anything may have written meanwhile.
func (s *stageEditors) recheckAll() {
	for _, f := range s.byCheckout {
		f.recheck()
	}
}

// saveThen saves f, then runs then; it asks first while f's text is one a
// save cannot simply write (mustAsk).
func (s *stageEditors) saveThen(f *fileEditor, then func()) {
	if f.mustAsk() {
		s.ask = unsavedAsk{open: true, editor: f, then: then}
		return
	}
	f.startSave(then, false)
}

// follow saves the editor that left the stage since the last frame: a
// session, another node or another project took its place.
func (s *stageEditors) follow(shown string) {
	if shown == s.shown {
		return
	}
	if f, ok := s.byCheckout[s.shown]; ok {
		f.flush(nil)
	}
	s.shown = shown
}

// flushAll saves every editor: the window lost the focus.
func (s *stageEditors) flushAll() {
	for _, f := range s.byCheckout {
		f.flush(nil)
	}
}

// layoutAsk draws the question while it is open and runs the answer.
func (s *stageEditors) layoutAsk(gtx C, th *ui.Theme, root *ui.Root) {
	s.takeTouched()
	// A save asked for from the question lands here when its editor is off
	// the stage, and what waited on it runs.
	for _, f := range s.byCheckout {
		f.takeSave()
	}
	a := &s.ask
	if !a.open && !a.state.Animating(gtx) {
		return
	}
	switch {
	case a.save.Clicked(gtx):
		a.open = false
		a.editor.startSave(a.then, true)
	case a.discard.Clicked(gtx):
		a.open = false
		a.then()
	case a.cancel.Clicked(gtx):
		a.open = false
	}
	pop := th.On(th.Popover)
	cancel := pop.Button(&a.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	discard := pop.Button(&a.discard, i18n.T("dock.editor.discard"))
	discard.Variant = ui.VariantDestructive
	save := pop.Button(&a.save, i18n.T("dock.editor.save"))
	name := ""
	if a.editor != nil {
		name = path.Base(a.editor.rel)
	}
	d := th.Dialog(&a.state, a.open, i18n.T("dock.editor.unsavedTitle", i18n.Args{"name": name}), i18n.T("dock.editor.unsavedDescription"))
	d.Actions, d.Root = []layout.Widget{cancel.Layout, discard.Layout, save.Layout}, root
	if d.Layout(gtx) {
		a.open = false
	}
}

// openInEditor puts rel of checkout on the stage, on lines line to end. A
// file in conflict goes to the Review tab's resolver instead, as from its
// card.
func (u *window) openInEditor(checkout, rel string, line, end int, note agentNote) {
	if slices.Contains(u.model.gitOf(checkout).Conflicted, rel) {
		u.dock.show(dockReview)
		u.review.resolveNext = rel
		u.model.invalidate()
		return
	}
	u.stageFile(checkout, "", rel, func(f *fileEditor) { f.open(rel, line, end, note) })
}

// openArtifactInEditor puts the agent's Markdown file f of session's folder
// on the stage in session's checkout editor, read only.
func (u *window) openArtifactInEditor(session string, f backend.Artifact) {
	view, ok := u.model.sessionViewOf(session)
	if !ok {
		return
	}
	dir, name := filepath.Dir(f.Path), filepath.Base(f.Path)
	u.stageFile(view.path, dir, name, func(e *fileEditor) { e.openArtifact(dir, name) })
}

// stageFile puts rel on the stage in checkout's editor: dir is where an
// artifact lives, "" for the checkout's own file. open points the editor at
// it, once the file it held is saved.
func (u *window) stageFile(checkout, dir, rel string, open func(*fileEditor)) {
	f := u.editors.of(checkout)
	show := func() {
		open(f)
		u.model.openEditor(checkout, rel)
		u.settings.hide()
		u.pulls.hide()
		u.extension.hide()
		u.model.invalidate()
	}
	if f.holds(dir, rel) {
		show()
		return
	}
	u.editors.saveThen(f, show)
}

// applyEditor carries out what an editor node asked: show the editor, or
// close it, asking first about unsaved changes.
func (u *window) applyEditor(act sidebarAction) {
	if c := act.editor; c != "" {
		u.settings.hide()
		u.pulls.hide()
		u.extension.hide()
		u.model.showEditor(c)
		if f, ok := u.editors.byCheckout[c]; ok {
			f.focus = true
			f.recheck()
		}
		u.model.invalidate()
	}
	if c := act.editorClose; c != "" {
		f, ok := u.editors.byCheckout[c]
		closeIt := func() {
			delete(u.editors.byCheckout, c)
			u.model.closeEditor(c)
			u.model.invalidate()
		}
		if !ok {
			closeIt()
			return
		}
		u.editors.saveThen(f, func() {
			f.vimOff()
			closeIt()
		})
	}
}

// editorIsland draws checkout's editor over the island, its code on the
// island's own fill.
func (u *window) editorIsland(gtx C, v stageView) error {
	f, ok := u.editors.byCheckout[v.editor]
	if !ok {
		return nil
	}
	fill := u.islandColor(v)
	th := u.theme.On(fill)
	code := u.code
	// The island paints fill and the backdrop over it; the editor paints its
	// ground in fill with no alpha, which lets the dots through, and still
	// mixes its selection and line tints over fill's color.
	ground := fill
	ground.A = 0
	code.Background, code.GutterBackground = ground, ground
	inject := func(string) bool { return false }
	if id := u.model.referenceTarget(v.editor); id != "" {
		inject = u.injector(id)
	}
	return f.layout(gtx, th, editorEnv{root: &u.root, code: code, inject: inject, comments: &u.comments, notify: func(t ui.Toast) { u.toaster.Show(t) }, vim: u.vimHost()})
}

// vimHost is the Neovim behind the editors, nil while Vim is off.
func (u *window) vimHost() *vimHost {
	if !u.settings.editor.prefs.vim {
		return nil
	}
	if u.vim == nil {
		u.vim = newVimHost(u.editors.invalidate)
	}
	return u.vim
}

// editorWouldAsk is whether opening t would ask about unsaved changes to
// another file of its checkout: an agent never raises that question.
func (u *window) editorWouldAsk(t codeTarget) bool {
	view, ok := u.model.sessionViewOf(t.session)
	if !ok {
		return false
	}
	f, ok := u.editors.byCheckout[view.path]
	return ok && f.mustAsk() && f.rel != t.rel
}
