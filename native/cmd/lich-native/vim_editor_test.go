package main

import (
	"image"
	"image/color"
	"os/exec"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/neovim/go-client/nvim"

	"github.com/lichdotdev/lich/native/ui"
)

// vimEditor is loadedEditor run by a Neovim of its own, without the
// person's config.
func vimEditor(t *testing.T, text string) (*fileEditor, *fakeDisk) {
	t.Helper()
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("no nvim on the PATH")
	}
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, text)
	h := newVimHost(func() {})
	h.args = []string{"--embed", "--clean"}
	t.Cleanup(h.close)
	f.env.vim = h
	layOut(t, f)
	vimUntil(t, f, "the buffer to open", func() bool { return f.vim.state != nil })
	return f, d
}

// vimUntil lays Neovim's reports into f until ok, as frames would.
func vimUntil(t *testing.T, f *fileEditor, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		f.syncVim()
		layOut(t, f)
		if buf, _, _ := f.vim.held(); buf == 0 {
			f.vimFeed("lich_editor.focus(...)")
		}
		if ok() {
			return
		}
	}
	err, _ := f.env.vim.startErr()
	t.Fatalf("never saw %s; editor holds %q, Neovim at %+v, failed with %v", what, f.ed.State().Doc(), f.vim.state, err)
}

func vimType(f *fileEditor, text string) {
	f.vimKey(frameCtx(), key.EditEvent{Text: text})
}

func vimPress(f *fileEditor, name key.Name, mods key.Modifiers) {
	f.vimKey(frameCtx(), key.Event{Name: name, Modifiers: mods, State: key.Press})
}

func TestVimRunsTheEditorsKeys(t *testing.T) {
	f, _ := vimEditor(t, "one\ntwo\nthree\n")
	vimType(f, "jdd")
	vimUntil(t, f, "dd delete the second line", func() bool { return f.ed.State().Doc() == "one\nthree\n" })
	if !f.dirty {
		t.Fatal("Vim's edit left the editor clean")
	}
	vimType(f, "u")
	vimUntil(t, f, "u undo it", func() bool { return f.ed.State().Doc() == "one\ntwo\nthree\n" })
	vimType(f, "ggciwONE")
	vimPress(f, key.NameEscape, 0)
	vimType(f, "j.")
	vimUntil(t, f, "the dot repeat the change", func() bool { return f.ed.State().Doc() == "ONE\nONE\nthree\n" })
}

func TestVimVisualModeSelectsInTheEditor(t *testing.T) {
	f, _ := vimEditor(t, "one\ntwo\n")
	vimType(f, "vj")
	vimUntil(t, f, "a charwise selection", func() bool {
		r := f.ed.State().Selection().Main()
		return r.Anchor() == 0 && r.Head() == 5
	})
	vimPress(f, key.NameEscape, 0)
	vimType(f, "V")
	vimUntil(t, f, "a linewise selection", func() bool {
		r := f.ed.State().Selection().Main()
		return r.From() == 4 && r.To() == 8
	})
}

func TestVimWriteSavesThroughTheEditor(t *testing.T) {
	f, d := vimEditor(t, "one\n")
	vimType(f, "x:w")
	vimPress(f, key.NameReturn, 0)
	vimUntil(t, f, "the save land", func() bool {
		f.saving.mu.Lock()
		done := f.saving.done != nil
		f.saving.mu.Unlock()
		if done {
			f.takeSave()
		}
		return !f.dirty && f.ed.State().Doc() == "ne\n"
	})
	select {
	case w := <-d.writes:
		if w.Content != "ne\n" {
			t.Fatalf(":w wrote %q", w.Content)
		}
	default:
		t.Fatal(":w wrote nothing")
	}
}

func TestVimTakesTheDisksTextWhenTheEditorReloads(t *testing.T) {
	f, _ := vimEditor(t, "one\n")
	f.reloadFrom("agent\n")
	vimUntil(t, f, "Neovim answer the replacement", func() bool { return !f.vim.setting })
	vimType(f, "x")
	vimUntil(t, f, "x act on the disk's text", func() bool { return f.ed.State().Doc() == "gent\n" })
}

func TestVimOffGivesTheEditorItsKeysBack(t *testing.T) {
	f, _ := vimEditor(t, "one\n")
	f.env.vim = nil
	f.syncVim()
	if buf, _, _ := f.vim.held(); f.ed.OnKey != nil || buf != 0 {
		t.Fatal("the editor still hands its keys to Neovim")
	}
}

func TestVimAvailableFindsTheNeovimOnThePath(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("no nvim on the PATH")
	}
	if err := vimAvailable(); err != nil {
		t.Fatalf("a Neovim %s reported missing: %v", vimMinVersion, err)
	}
}

func TestVimThatFailsToStartLeavesTheEditorItsKeys(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\n")
	var told []ui.Toast
	f.env.notify = func(t ui.Toast) { told = append(told, t) }
	h := newVimHost(func() {})
	h.args = []string{"--no-such-flag-lich"}
	f.env.vim = h
	h.do(func(*nvim.Nvim, int) error { return nil })
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if f.syncVim(); f.ed.OnKey == nil && len(told) == 1 {
			break
		}
	}
	if f.ed.OnKey != nil || len(told) != 1 {
		t.Fatalf("OnKey set %v, %d toasts; want the editor's own keys and one toast", f.ed.OnKey != nil, len(told))
	}
	f.syncVim()
	if len(told) != 1 {
		t.Fatal("the failure was told twice")
	}
}

func TestVimCursorWearsTheThemesCaretColor(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\n")
	f.env.code.Cursor = color.NRGBA{R: 0xff, A: 0xff}
	f.env.vim = newVimHost(func() {})
	f.ed.Theme = vimHideCaret(f.env.code)
	if got := f.vimCursorColor(vimCursor{shape: "vertical", percent: 25}); got != f.env.code.Cursor {
		t.Fatalf("Vim's cursor drawn in %v, want the theme's caret %v", got, f.env.code.Cursor)
	}
}

func TestAFocusedVimEditorKeepsTheControlKeys(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\n")
	f.env.vim = newVimHost(func() {})
	s := stageEditors{byCheckout: map[string]*fileEditor{"/c": f}}
	var router input.Router
	var ops op.Ops
	frame := func() C {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300))}
		if _, err := f.ed.Layout(gtx); err != nil {
			t.Fatal(err)
		}
		router.Frame(&ops)
		return gtx
	}
	gtx := frame()
	if s.vimHasKeys(gtx) {
		t.Fatal("an editor without the keyboard kept the control keys")
	}
	f.ed.Focus(gtx)
	frame()
	if !s.vimHasKeys(layout.Context{Source: router.Source()}) {
		t.Fatal("the focused Vim editor left Ctrl+letter to the window's shortcuts")
	}
	f.env.vim = nil
	if s.vimHasKeys(layout.Context{Source: router.Source()}) {
		t.Fatal("the editor kept the control keys with Vim off")
	}
}
