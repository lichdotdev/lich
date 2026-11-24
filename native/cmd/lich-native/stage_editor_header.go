package main

// The stage editor's header and its saves: the path with the file's name
// standing out and the unsaved dot, a second after the last edit or at once
// on Ctrl+S, and whenever the editor is left.

import (
	"context"
	"image"
	"path"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// autoSaveDelay is how long after the last edit the editor writes it.
const autoSaveDelay = time.Second

// editorSaving is the save in flight, which lands on the frame after it.
// UI goroutine fields: next is a save asked for while one was in flight, run
// once it lands so it writes the text as it is then, with what waits on it;
// failedGen is the text a save failed on, which waits for an edit before it
// is tried again by itself.
type editorSaving struct {
	mu        sync.Mutex
	busy      bool
	done      *editorSaved
	next      bool
	nextDo    func()
	failed    bool
	failedGen int
}

// editorSaved is a save that came back: the text it wrote, of which open,
// and what was waiting on it. disk is the file as it was found when it no
// longer held what the editor read, in which case nothing was written.
type editorSaved struct {
	doc   string
	opens int
	err   error
	disk  *string
	then  func()
}

// actions runs Ctrl+S, which saves at once.
func (f *fileEditor) actions(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: "S", Required: key.ModShortcut})
		if !ok {
			break
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press && f.editable() && f.dirty {
			f.startSave(nil, false)
		}
	}
}

// autoSave writes the text autoSaveDelay after the last edit. It waits while
// the disk's change is up for the user to decide, and after a failed save
// until the next edit.
func (f *fileEditor) autoSave(gtx C) {
	if !f.dirty || !f.editable() || f.disk.theirs != nil || (f.saving.failed && f.saving.failedGen == f.gen) {
		return
	}
	due := f.editedAt.Add(autoSaveDelay)
	if now := f.now(); now.Before(due) {
		gtx.Execute(op.InvalidateCmd{At: due})
		return
	}
	f.startSave(nil, false)
}

// flush saves what the editor holds now, then runs then; with nothing to
// save, or only a failure or a disk change to decide on, it saves nothing.
func (f *fileEditor) flush(then func()) {
	if f.dirty && f.disk.theirs == nil && !(f.saving.failed && f.saving.failedGen == f.gen) {
		f.startSave(then, false)
	}
}

// startSave writes the text over the file, then runs then; with nothing to
// save, then runs at once. Unless overwrite, it first reads the file and
// writes nothing when it no longer holds what the editor read: an agent wrote
// it since, and the disk's change is put to the user instead.
func (f *fileEditor) startSave(then func(), overwrite bool) {
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
	doc, content := f.savedText()
	base := f.base
	client, root, rel, opens := f.client, f.root(), f.rel, f.opens
	go func() {
		saved := &editorSaved{doc: doc, opens: opens, then: then}
		ctx := context.Background()
		if !overwrite {
			disk, err := client.ReadFile(ctx, root, rel)
			if err == nil && normalizedText(disk) != base {
				saved.disk = &disk
			}
		}
		if saved.disk == nil {
			saved.err = client.WriteFile(ctx, root, rel, content)
		}
		f.saving.mu.Lock()
		f.saving.busy, f.saving.done = false, saved
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
	switch {
	case s.opens != f.opens:
		return
	case s.err != nil:
		f.saving.failed, f.saving.failedGen = true, f.gen
		f.env.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("dock.editor.saveFailed", i18n.Args{"name": path.Base(f.rel)}), Description: backend.ErrorText(s.err)})
		return
	case s.disk != nil:
		f.disk.seen = *s.disk
		f.weighDisk(*s.disk)
		return
	}
	f.saving.failed = false
	f.disk.saves++
	if s.doc != f.base {
		f.blame.gen++
	}
	f.base, f.disk.seen, f.disk.theirs = s.doc, s.doc, nil
	f.gen++
	doc, _ := f.savedText()
	f.setDirty(doc != f.base)
	f.onSaved()
	f.vimSaved(s.doc)
	if next {
		f.startSave(nextDo, false)
	}
	if s.then != nil {
		s.then()
	}
}

// mustAsk is whether leaving f's text needs the user: a save of it failed,
// or the disk changed under it and nothing was decided.
func (f *fileEditor) mustAsk() bool {
	return f.dirty && (f.saving.failed || f.disk.theirs != nil)
}

// header is the strip over the code: the folder muted and the file's name
// in the foreground, the unsaved dot, then the Markdown toggles.
func (f *fileEditor) header(gtx C, th *ui.Theme) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(10))))
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			children := []layout.FlexChild{layout.Flexed(1, func(gtx C) D { return f.title(gtx, th) })}
			if isMarkdown(f.rel) && f.kind == blobNone {
				children = append(children, layout.Rigid(func(gtx C) D { return f.markdownToggles(gtx, th, f.env.root) }), ui.Gap(1.5))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (f *fileEditor) title(gtx C, th *ui.Theme) D {
	dir, name := path.Dir(f.rel), path.Base(f.rel)
	if f.writable {
		dir = displayPath(f.dir)
	}
	children := []layout.FlexChild{}
	if dir != "." {
		children = append(children, layout.Rigid(th.Text(ui.TextSM, strings.TrimSuffix(dir, "/")+"/").Mono().Muted().Layout))
	}
	children = append(children, layout.Rigid(th.Text(ui.TextSM, name).Mono().Weight(font.Medium).Layout))
	if f.dirty {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return ui.Dot(gtx, th.ToneWait, editorDirtyDot) }))
	}
	if f.readOnly() {
		children = append(children, ui.Gap(3), layout.Rigid(th.Text(ui.TextXS, i18n.T("dock.editor.artifactReadOnly")).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}
