package main

// The stage editor's header and what it saves: the path with the file's
// name standing out, the unsaved dot, Undo, and Save, which Ctrl+S presses.

import (
	"context"
	"image"
	"path"
	"runtime"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// editorSaving is the save in flight, which lands on the frame after it.
// UI goroutine fields: next is a save asked for while one was in flight, run
// once it lands so it writes the text as it is then, with what waits on it.
type editorSaving struct {
	mu     sync.Mutex
	busy   bool
	done   *editorSaved
	next   bool
	nextDo func()
}

// editorSaved is a save that came back: the text it wrote, of which open,
// and what was waiting on it.
type editorSaved struct {
	doc   string
	opens int
	err   error
	then  func()
}

var saveChord = combo{Mod: true, Key: "s"}

// actions runs the header's clicks and Ctrl+S.
func (f *fileEditor) actions(gtx C) {
	if f.undo.Clicked(gtx) && f.editable() {
		f.ed.Undo()
		f.focus = true
	}
	saved := f.save.Clicked(gtx)
	for {
		ev, ok := gtx.Event(key.Filter{Name: "S", Required: key.ModShortcut})
		if !ok {
			break
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
			saved = true
		}
	}
	if saved && f.editable() && f.dirty {
		f.startSave(nil)
	}
}

// startSave writes the text over the file, then runs then; with nothing to
// save, then runs at once.
func (f *fileEditor) startSave(then func()) {
	if !f.editable() || !f.dirty {
		if then != nil {
			then()
		}
		return
	}
	f.saving.mu.Lock()
	if f.saving.busy {
		f.saving.mu.Unlock()
		f.saving.next, f.saving.nextDo = true, then
		return
	}
	f.saving.busy = true
	f.saving.mu.Unlock()
	st := f.ed.State()
	doc, content := st.Doc(), st.SliceDoc(0, len(st.Doc()))
	client, checkout, rel, opens := f.client, f.checkout, f.rel, f.opens
	go func() {
		err := client.WriteFile(context.Background(), checkout, rel, content)
		f.saving.mu.Lock()
		f.saving.busy, f.saving.done = false, &editorSaved{doc: doc, opens: opens, err: err, then: then}
		f.saving.mu.Unlock()
		f.invalidate()
	}()
}

// takeSave lays a save that came back into the editor: what it wrote is
// what the disk holds now, so the disk's next read of it is no news.
func (f *fileEditor) takeSave() {
	f.saving.mu.Lock()
	s := f.saving.done
	f.saving.done = nil
	f.saving.mu.Unlock()
	if s == nil {
		return
	}
	next, nextDo := f.saving.next, f.saving.nextDo
	f.saving.next, f.saving.nextDo = false, nil
	if s.err != nil {
		f.env.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("dock.editor.saveFailed", i18n.Args{"name": path.Base(f.rel)}), Description: backend.ErrorText(s.err)})
		return
	}
	if s.opens != f.opens {
		return
	}
	if next {
		f.startSave(nextDo)
	}
	f.base, f.disk.seen, f.disk.theirs = s.doc, s.doc, nil
	f.gen++
	f.blame.gen++
	f.setDirty(f.ed.State().Doc() != f.base)
	f.onSaved()
	if s.then != nil {
		s.then()
	}
}

// header is the h-8 strip: the folder muted and the file's name in the
// foreground, the unsaved dot, then the actions.
func (f *fileEditor) header(gtx C, th *ui.Theme) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(10))))
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			children := []layout.FlexChild{layout.Flexed(1, func(gtx C) D { return f.title(gtx, th) })}
			if isMarkdown(f.rel) && f.kind == blobNone {
				children = append(children, layout.Rigid(func(gtx C) D { return f.markdownToggle(gtx, th, f.env.root) }), ui.Gap(1.5))
			}
			if f.kind == blobNone && !f.md.rendering {
				children = append(children, layout.Rigid(func(gtx C) D { return f.undoButton(gtx, th) }), ui.Gap(1.5),
					layout.Rigid(func(gtx C) D { return f.saveButton(gtx, th) }))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (f *fileEditor) title(gtx C, th *ui.Theme) D {
	dir, name := path.Dir(f.rel), path.Base(f.rel)
	children := []layout.FlexChild{}
	if dir != "." {
		children = append(children, layout.Rigid(th.Text(ui.TextSM, strings.TrimSuffix(dir, "/")+"/").Mono().Muted().Layout))
	}
	children = append(children, layout.Rigid(th.Text(ui.TextSM, name).Mono().Weight(font.Medium).Layout))
	if f.dirty {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return ui.Dot(gtx, th.ToneWait, editorDirtyDot) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (f *fileEditor) undoButton(gtx C, th *ui.Theme) D {
	b := th.Button(&f.undo, i18n.T("dock.editor.undo"))
	b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantGhost, !f.editable() || f.ed.UndoDepth() == 0
	return b.Layout(gtx)
}

func (f *fileEditor) saveButton(gtx C, th *ui.Theme) D {
	b := th.Button(&f.save, i18n.T("dock.editor.save"))
	b.Size, b.Disabled = ui.SizeSM, !f.editable() || !f.dirty
	tip := i18n.T("dock.editor.saveTip", i18n.Args{"keys": formatCombo(saveChord, runtime.GOOS == "darwin")})
	return th.Tooltip(f.env.root, &f.saveTip, tip).Layout(gtx, b.Layout)
}
