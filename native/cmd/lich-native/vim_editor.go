package main

// The stage editor run by the window's Neovim while Vim is on: every key
// goes to its buffer there, and what Neovim reports (the lines, the cursor,
// the mode, the visual region, the window's top line, a :w) is laid into
// the editor on the next frame. The text stays the editor's, so saving,
// the disk's changes and the margin's marks work as they do without Vim.

import (
	"image"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/neovim/go-client/nvim"
)

// vimBuffer is the editor's buffer in Neovim. mu guards what the host's
// goroutine writes: the buffer, the process it lives in and the open it
// holds, and the tick of a replacement lich made.
type vimBuffer struct {
	mu      sync.Mutex
	buf     int
	gen     int
	opens   int
	setTick int
	setDone bool

	// UI goroutine: opening while the buffer is on its way, setting while a
	// replacement is, skip the line events up to the tick it answered; top is
	// the window's top line as the editor and Neovim last agreed on, state
	// what Neovim last reported.
	opening bool
	setting bool
	skip    int
	top     int
	state   *vimState
	// shown is the :s preview laid in the editor, nil for none.
	shown *vimShown
	// sent counts the clicks and wheels handed to Neovim; a state that has not
	// seen the last is older than what the editor shows.
	sent int
	// yank is the last yank's text, lit until yankUntil.
	yank      []godemirror.SelectionRange
	yankUntil time.Time
	pum       vimPumView
	lspUI     vimLspUI
	// installing hears the end of an Install running, nil for none;
	// installErr is the last one's failure.
	installing <-chan error
	installErr string
}

// vimLspUI is the strip's language-server widgets.
type vimLspUI struct {
	install ui.Clickable
	reason  ui.Tooltip
}

// vimShown is a :s preview in the editor: bytes from..to of the text are
// the preview's, standing where original was.
type vimShown struct {
	from, to int
	original string
	preview  vimPreview
}

// held is the buffer and the process and open it belongs to, 0 for none.
func (b *vimBuffer) held() (buf, gen, opens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf, b.gen, b.opens
}

// vimHasKeys is whether an editor Vim runs has the keyboard, which the
// window's Ctrl+letter shortcuts then leave to it, as to a focused terminal.
func (s *stageEditors) vimHasKeys(gtx C) bool {
	for _, f := range s.byCheckout {
		if f.vimOn() && f.ed.HasFocus(gtx) {
			return true
		}
	}
	return false
}

// vimOn is whether Neovim runs the editor's keys now: not when it failed to
// start, which leaves the editor its own keys until Vim is turned on again.
func (f *fileEditor) vimOn() bool {
	if f.env.vim == nil || !f.editable() {
		return false
	}
	err, tell := f.env.vim.startErr()
	if tell {
		f.env.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("dock.editor.vimFailed"), Description: err.Error()})
	}
	return err == nil
}

// syncVim lays what Neovim reported into the editor and hands it the keys,
// or gives the editor its own keys back once Vim is off.
func (f *fileEditor) syncVim() {
	if f.ed != nil {
		// Neovim pairs, indents and takes Tab by the person's own config.
		f.ed.CloseBrackets, f.ed.IndentWithTab = !f.vimOn(), !f.vimOn()
	}
	if !f.vimOn() {
		f.vimOff()
		return
	}
	h := f.env.vim
	f.ed.OnKey = f.vimKey
	f.ed.OnScroll = f.vimScrolled
	f.vim.mu.Lock()
	buf, gen, opens, setTick, setDone := f.vim.buf, f.vim.gen, f.vim.opens, f.vim.setTick, f.vim.setDone
	f.vim.setDone = false
	f.vim.mu.Unlock()
	if setDone {
		f.vim.setting, f.vim.skip = false, setTick
	}
	events, live := h.take(buf)
	if buf == 0 || gen != live || opens != f.opens {
		f.vimOpen(buf, gen == live)
		return
	}
	f.vim.opening = false
	for _, ev := range events {
		f.vimApply(ev)
	}
	f.vimInstalled()
}

func (f *fileEditor) vimOff() {
	if f.ed != nil {
		f.ed.OnKey, f.ed.OnScroll = nil, nil
		f.vimUnpreview()
	}
	if buf, _, _ := f.vim.held(); buf == 0 && !f.vim.opening {
		return
	}
	f.vim.mu.Lock()
	buf := f.vim.buf
	f.vim.buf, f.vim.gen, f.vim.opens = 0, 0, 0
	f.vim.mu.Unlock()
	f.vim.opening, f.vim.setting, f.vim.state = false, false, nil
	if f.ed != nil {
		f.ed.SetDiagnostics(nil)
	}
	if h := f.env.vim; h != nil && buf != 0 {
		h.do(func(v *nvim.Nvim, _ int) error { return v.ExecLua("lich_editor.close(...)", nil, buf) })
	}
}

// vimOpen asks Neovim for a buffer holding the editor's text, closing the
// one of the file before when it still lives.
func (f *fileEditor) vimOpen(old int, alive bool) {
	if f.vim.opening {
		return
	}
	// A new buffer may be in a new Neovim, which has seen none of the clicks.
	f.vim.opening, f.vim.setting, f.vim.state, f.vim.sent = true, false, nil, 0
	lines, _ := vimLinesOf(f.ed.State().Doc())
	path, opens, wrap := filepath.Join(f.root(), filepath.FromSlash(f.rel)), f.opens, f.ed.Wrap
	f.env.vim.do(func(v *nvim.Nvim, gen int) error {
		if alive && old != 0 {
			_ = v.ExecLua("lich_editor.close(...)", nil, old)
		}
		var buf int
		if err := v.ExecLua("return lich_editor.open(...)", &buf, path, lines, wrap); err != nil {
			return err
		}
		f.vim.mu.Lock()
		f.vim.buf, f.vim.gen, f.vim.opens = buf, gen, opens
		f.vim.mu.Unlock()
		return nil
	})
}

// vimSet hands Neovim the text lich put in the editor, the disk's.
func (f *fileEditor) vimSet() {
	buf, _, opens := f.vim.held()
	if !f.vimOn() || f.vim.opening || buf == 0 || opens != f.opens {
		return
	}
	f.vim.setting = true
	lines, _ := vimLinesOf(f.ed.State().Doc())
	f.env.vim.do(func(v *nvim.Nvim, gen int) error {
		f.vim.mu.Lock()
		buf, live := f.vim.buf, f.vim.gen == gen
		f.vim.mu.Unlock()
		if !live {
			return nil
		}
		var tick int
		if err := v.ExecLua("return lich_editor.set(...)", &tick, buf, lines); err != nil {
			return err
		}
		f.vim.mu.Lock()
		f.vim.setTick, f.vim.setDone = tick, true
		f.vim.mu.Unlock()
		return nil
	})
}

func (f *fileEditor) vimApply(ev vimEvent) {
	switch {
	case ev.lines != nil:
		if f.vim.setting || ev.lines.tick <= f.vim.skip {
			return
		}
		f.vimUnpreview()
		doc := f.ed.State().Doc()
		if ch, ok := vimChange(doc, vimSplice(doc, ev.lines.first, ev.lines.last, ev.lines.lines)); ok {
			_ = f.ed.Dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{ch}, UserEvent: "input.vim"})
		}
		if f.vim.state != nil {
			f.vimSelect(*f.vim.state)
		}
	case ev.state != nil:
		if ev.state.seen < f.vim.sent {
			return
		}
		f.vim.state = ev.state
		f.vimPreview(ev.state.preview)
		f.vimSelect(*ev.state)
	case ev.yank != nil:
		f.vim.yank = vimSelection(f.ed.State(), *ev.yank).Ranges()
		f.vim.yankUntil = f.now().Add(vimYankFlash)
	case ev.diags != nil:
		st := f.ed.State()
		out := make([]godemirror.Diagnostic, 0, len(*ev.diags))
		for _, d := range *ev.diags {
			out = append(out, godemirror.Diagnostic{From: vimPos(st, d.row+1, d.col), To: vimPos(st, d.endRow+1, d.endCol),
				Severity: godemirror.Severity(d.severity), Message: d.message, Source: d.source, Code: d.code})
		}
		f.ed.SetDiagnostics(out)
	case ev.quit:
		f.onQuit(ev.discard)
	case ev.write:
		// The lines before the :w reach dirty only at the editor's next
		// Layout; the save cannot wait for it.
		doc, _ := f.savedText()
		f.setDirty(doc != f.base)
		f.startSave(nil, false)
	}
}

// vimSelect shows Neovim's cursor and region in the editor, and scrolls it
// to the window's top line.
func (f *fileEditor) vimSelect(s vimState) {
	st := f.ed.State()
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Selection: vimSelection(st, s), UserEvent: "select.vim"})
	if s.top != f.vim.top && s.top >= 1 {
		f.vim.top = s.top
		top, _ := f.ed.LineBlockAt(st.Line(min(s.top, st.Lines())).From)
		f.ed.SetScrollTop(top)
	}
}

// vimKey sends the editor's keys and typed text to Neovim; the editor acts
// on none of them while Vim runs it.
func (f *fileEditor) vimKey(_ layout.Context, ev event.Event) bool {
	var keys string
	switch ev := ev.(type) {
	case key.Event:
		keys, _ = vimKeyOf(ev)
	case key.EditEvent:
		keys = vimTextOf(ev.Text)
	default:
		return false
	}
	if keys != "" {
		f.env.vim.keys(f.vimBufOf, keys)
	}
	return true
}

// vimFeed calls fn of vim.lua with the buffer first, once it is open.
func (f *fileEditor) vimFeed(fn string, args ...any) {
	f.env.vim.do(func(v *nvim.Nvim, gen int) error {
		buf := f.vimBufOf(gen)
		if buf == 0 {
			return nil
		}
		return v.ExecLua(fn, nil, append([]any{buf}, args...)...)
	})
}

// vimBufOf is the editor's buffer when it lives in process gen, else 0.
func (f *fileEditor) vimBufOf(gen int) int {
	f.vim.mu.Lock()
	defer f.vim.mu.Unlock()
	if f.vim.gen != gen {
		return 0
	}
	return f.vim.buf
}

// vimPointer hands Neovim a click or a drag in the editor: its cursor
// there, or a characterwise Visual selection over what was dragged.
func (f *fileEditor) vimPointer(u godemirror.Update) {
	pointer := false
	for _, t := range u.Transactions {
		pointer = pointer || t.IsUserEvent("select.pointer")
	}
	if !pointer || !f.vimOn() {
		return
	}
	st := u.State
	main := st.Selection().Main()
	row, col := vimClickCol(st, main.Anchor(), main.Head())
	f.vim.sent++
	if main.Empty() {
		f.vimFeed("lich_editor.click(...)", f.vim.sent, row, col)
		return
	}
	arow, acol := vimRowCol(st, main.Anchor())
	f.vimFeed("lich_editor.click(...)", f.vim.sent, row, col, arow, acol)
}

// vimScrolled tells Neovim the line the wheel brought to the editor's top.
func (f *fileEditor) vimScrolled() {
	top := f.ed.LineBlockAtHeight(f.ed.ScrollTop()).Number
	if top == f.vim.top || !f.vimOn() {
		return
	}
	f.vim.top = top
	f.vim.sent++
	f.vimFeed("lich_editor.scroll(...)", f.vim.sent, top)
}

// vimResize fits Neovim's window to the editor's code area, size, so its
// scrolling commands move as many lines as the editor shows.
func (f *fileEditor) vimResize(size image.Point) {
	if !f.vimOn() {
		return
	}
	row, ok := f.ed.CoordsAtPos(0, 1)
	g := f.ed.Geometry()
	if !ok || row.Dy() <= 0 || g.CellWidth <= 0 {
		return
	}
	f.env.vim.resize(max(size.Y/row.Dy(), 1), max((size.X-g.TextX)/g.CellWidth, 1))
}

// vimSaved tells Neovim that lich wrote doc, which clears 'modified' when
// the buffer still holds it, so a :q after an auto-save closes as Vim's does.
func (f *fileEditor) vimSaved(doc string) {
	if !f.vimOn() {
		return
	}
	lines, _ := vimLinesOf(doc)
	f.vimFeed("lich_editor.saved(...)", lines)
}

// vimPreview lays the :s being typed into the editor, as 'inccommand' shows
// it, out of the history and of what the editor saves; nil takes it out.
func (f *fileEditor) vimPreview(p *vimPreview) {
	if p == nil {
		f.vimUnpreview()
		return
	}
	if f.vim.shown != nil && f.vim.shown.preview.first == p.first && f.vim.shown.preview.last == p.last &&
		slices.Equal(f.vim.shown.preview.lines, p.lines) {
		return
	}
	f.vimUnpreview()
	doc := f.ed.State().Doc()
	ch, changed := vimChange(doc, vimSplice(doc, p.first, p.last, p.lines))
	f.vim.shown = &vimShown{from: ch.From, to: ch.From + len(ch.Insert), original: doc[ch.From:ch.To], preview: *p}
	if changed {
		f.vimPreviewDispatch(ch)
	}
}

// vimUnpreview puts back the text a :s preview stood over.
func (f *fileEditor) vimUnpreview() {
	s := f.vim.shown
	if s == nil {
		return
	}
	f.vim.shown = nil
	if s.from != s.to || s.original != "" {
		f.vimPreviewDispatch(godemirror.ChangeSpec{From: s.from, To: s.to, Insert: s.original})
	}
}

// vimPreviewEvent names a preview's transactions, which the editor's edits
// leave out.
const vimPreviewEvent = "preview.vim"

func (f *fileEditor) vimPreviewDispatch(ch godemirror.ChangeSpec) {
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{ch}, UserEvent: vimPreviewEvent, SkipHistory: true})
}

// vimOnlyPreview is whether u's changes are a :s preview's alone.
func vimOnlyPreview(u godemirror.Update) bool {
	for _, t := range u.Transactions {
		if !t.Changes.Empty() && !t.IsUserEvent(vimPreviewEvent) {
			return false
		}
	}
	return true
}

// savedText is the text a save writes, in the editor's lines and in the
// file's: the editor's, with a :s preview's text put back.
func (f *fileEditor) savedText() (doc, content string) {
	st := f.ed.State()
	doc = st.Doc()
	if s := f.vim.shown; s != nil {
		doc = doc[:s.from] + s.original + doc[s.to:]
	}
	if f.sep != "" && f.sep != "\n" {
		return doc, strings.ReplaceAll(doc, "\n", f.sep)
	}
	return doc, doc
}
