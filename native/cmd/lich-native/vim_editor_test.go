package main

import (
	"image"
	"image/color"
	"os/exec"
	"slices"
	"strings"
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

func TestVimSearchShowsItsMatches(t *testing.T) {
	f, _ := vimEditor(t, "one two\ntwo one one\n")
	want := [][3]int{{1, 0, 3}, {2, 4, 3}, {2, 8, 3}}
	matches := func() [][3]int {
		if f.vim.state == nil {
			return nil
		}
		return f.vim.state.matches
	}
	vimType(f, "/one")
	vimUntil(t, f, "the matches while the pattern is typed", func() bool { return slices.Equal(matches(), want) })
	vimPress(f, key.NameReturn, 0)
	vimType(f, "n")
	vimUntil(t, f, "the matches stay after Enter", func() bool { return f.vim.state.row == 2 && slices.Equal(matches(), want) })
	vimType(f, ":noh")
	vimPress(f, key.NameReturn, 0)
	vimUntil(t, f, ":noh clear them", func() bool { return f.vim.state.mode == "n" && len(matches()) == 0 })
}

// vimCommand types :cmd and Enter.
func vimCommand(f *fileEditor, cmd string) {
	vimType(f, ":"+cmd)
	vimPress(f, key.NameReturn, 0)
}

func TestVimQuitClosesTheFileAndKeepsNeovim(t *testing.T) {
	f, d := vimEditor(t, "one\n")
	var quits []bool
	f.onQuit = func(discard bool) { quits = append(quits, discard) }
	vimCommand(f, "q")
	vimUntil(t, f, ":q close the file", func() bool { return slices.Equal(quits, []bool{false}) })

	vimType(f, "x")
	vimUntil(t, f, "the edit", func() bool { return f.ed.State().Doc() == "ne\n" })
	vimCommand(f, "qa")
	vimUntil(t, f, "E37 refuse :qa over unwritten changes", func() bool {
		buf, _, _ := f.vim.held()
		u, _ := f.env.vim.status(buf)
		return u.msgError && strings.HasPrefix(u.msg, "E37")
	})
	if len(quits) != 1 {
		t.Fatalf(":qa over unwritten changes quit: %v", quits)
	}
	vimCommand(f, "q!")
	vimUntil(t, f, ":q! close it, dropping the change", func() bool { return slices.Equal(quits, []bool{false, true}) })

	vimCommand(f, "wq")
	vimUntil(t, f, ":wq write and close", func() bool {
		f.saving.mu.Lock()
		done := f.saving.done != nil
		f.saving.mu.Unlock()
		if done {
			f.takeSave()
		}
		return slices.Equal(quits, []bool{false, true, false}) && !f.dirty
	})
	if w := <-d.writes; w.Content != "ne\n" {
		t.Fatalf(":wq wrote %q", w.Content)
	}
	vimType(f, "x")
	vimUntil(t, f, "Neovim still run the keys", func() bool { return f.ed.State().Doc() == "e\n" })
}

func TestVimSubstitutePreviewsAsItIsTyped(t *testing.T) {
	f, _ := vimEditor(t, "one two\ntwo one\n")
	matches := func() [][3]int {
		if f.vim.state == nil {
			return nil
		}
		return f.vim.state.matches
	}
	vimType(f, ":%s/one")
	vimUntil(t, f, "the pattern's matches before the replacement", func() bool {
		return slices.Equal(matches(), [][3]int{{1, 0, 3}, {2, 4, 3}})
	})
	vimType(f, "/ONE")
	vimUntil(t, f, "the preview in the editor", func() bool { return f.ed.State().Doc() == "ONE two\ntwo ONE\n" })
	if f.dirty {
		t.Fatal("a preview marked the file unsaved")
	}
	if doc, _ := f.savedText(); doc != "one two\ntwo one\n" {
		t.Fatalf("a save during the preview would write %q", doc)
	}
	if !slices.Equal(matches(), [][3]int{{1, 0, 3}, {2, 4, 3}}) {
		t.Fatalf("the replaced text lights %v", matches())
	}
	vimPress(f, key.NameEscape, 0)
	vimUntil(t, f, "Esc put the text back", func() bool { return f.ed.State().Doc() == "one two\ntwo one\n" && f.vim.shown == nil })
	vimCommand(f, "%s/one/ONE/")
	vimUntil(t, f, "Enter make it a change", func() bool { return f.ed.State().Doc() == "ONE two\ntwo ONE\n" && f.dirty })
	vimType(f, "u")
	vimUntil(t, f, "u take it back", func() bool { return f.ed.State().Doc() == "one two\ntwo one\n" })
}

func TestVimLightsTheMatchparenPair(t *testing.T) {
	f, _ := vimEditor(t, "(a [b] c)\n")
	vimType(f, "l")
	vimType(f, "h")
	vimUntil(t, f, "the pair around the cursor", func() bool {
		return slices.Equal(f.vim.state.parens, [][3]int{{1, 0, 1}, {1, 8, 1}})
	})
	vimType(f, "l")
	vimUntil(t, f, "the pair go off a letter", func() bool { return len(f.vim.state.parens) == 0 })
}

func TestVimFlashesAYankOnlyWhenTheConfigLightsYanks(t *testing.T) {
	f, _ := vimEditor(t, "one\ntwo\n")
	vimType(f, "yy")
	vimUntil(t, f, "the yank land", func() bool { return f.vim.state != nil })
	if len(f.vim.yank) != 0 {
		t.Fatal("a yank flashed with no TextYankPost of the person's own")
	}
	vimCommand(f, "autocmd TextYankPost * lua vim.hl.on_yank()")
	vimType(f, "jyy")
	vimUntil(t, f, "the yank's line lit", func() bool {
		return len(f.vim.yank) == 1 && f.vim.yank[0].From() == 4 && f.vim.yank[0].To() == 8
	})
}

func TestVimCompletionMenuShowsAndPicks(t *testing.T) {
	f, _ := vimEditor(t, "alphabet alpine\n")
	vimType(f, "oal")
	vimPress(f, "N", key.ModCtrl)
	pum := func() *vimPum {
		u, _ := f.env.vim.status(func() int { b, _, _ := f.vim.held(); return b }())
		return u.pum
	}
	vimUntil(t, f, "the completion menu", func() bool {
		p := pum()
		return p != nil && len(p.items) == 2 && p.items[0].word == "alphabet" && p.selected == 0
	})
	f.vimFeed("lich_editor.pick(...)", 1)
	vimUntil(t, f, "a pick put the word in", func() bool { return f.ed.State().Doc() == "alphabet alpine\nalpine\n" })
	vimPress(f, key.NameEscape, 0)
	vimUntil(t, f, "the menu close", func() bool { return pum() == nil })
}
