package main

// The stage editor run by the window's Neovim while Vim is on: every key
// goes to its buffer there, and what Neovim reports (the lines, the cursor,
// the mode, the visual region, the window's top line, a :w) is laid into
// the editor on the next frame. The text stays the editor's, so saving,
// the disk's changes and the margin's marks work as they do without Vim.

import (
	"image"
	"path/filepath"
	"sync"

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
}

// held is the buffer and the process and open it belongs to, 0 for none.
func (b *vimBuffer) held() (buf, gen, opens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf, b.gen, b.opens
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
}

func (f *fileEditor) vimOff() {
	if f.ed != nil {
		f.ed.OnKey, f.ed.OnScroll = nil, nil
	}
	if buf, _, _ := f.vim.held(); buf == 0 && !f.vim.opening {
		return
	}
	f.vim.mu.Lock()
	buf := f.vim.buf
	f.vim.buf, f.vim.gen, f.vim.opens = 0, 0, 0
	f.vim.mu.Unlock()
	f.vim.opening, f.vim.setting, f.vim.state = false, false, nil
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
	f.vim.opening, f.vim.setting, f.vim.state = true, false, nil
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
		doc := f.ed.State().Doc()
		if ch, ok := vimChange(doc, vimSplice(doc, ev.lines.first, ev.lines.last, ev.lines.lines)); ok {
			_ = f.ed.Dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{ch}, UserEvent: "input.vim"})
		}
		if f.vim.state != nil {
			f.vimSelect(*f.vim.state)
		}
	case ev.state != nil:
		f.vim.state = ev.state
		f.vimSelect(*ev.state)
	case ev.write:
		// The lines before the :w reach dirty only at the editor's next
		// Layout; the save cannot wait for it.
		f.setDirty(f.ed.State().Doc() != f.base)
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
		f.vimFeed("lich_editor.feed(...)", keys)
	}
	return true
}

// vimFeed calls fn of vim.lua with the buffer first, once it is open.
func (f *fileEditor) vimFeed(fn string, args ...any) {
	f.env.vim.do(func(v *nvim.Nvim, gen int) error {
		f.vim.mu.Lock()
		buf, live := f.vim.buf, f.vim.gen == gen
		f.vim.mu.Unlock()
		if !live || buf == 0 {
			return nil
		}
		return v.ExecLua(fn, nil, append([]any{buf}, args...)...)
	})
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
	if main.Empty() {
		f.vimFeed("lich_editor.click(...)", row, col)
		return
	}
	arow, acol := vimRowCol(st, main.Anchor())
	f.vimFeed("lich_editor.click(...)", row, col, arow, acol)
}

// vimScrolled tells Neovim the line the wheel brought to the editor's top.
func (f *fileEditor) vimScrolled() {
	top := f.ed.LineBlockAtHeight(f.ed.ScrollTop()).Number
	if top == f.vim.top || !f.vimOn() {
		return
	}
	f.vim.top = top
	f.vimFeed("lich_editor.scroll(...)", top)
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
