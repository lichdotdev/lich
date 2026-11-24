package main

import (
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

// fakeDisk stands in for the backend's reads and writes of the one file an
// editor holds, handing each write to the test.
type fakeDisk struct {
	mu     sync.Mutex
	text   string
	fail   bool
	writes chan api.WriteFileOptions
}

func (d *fakeDisk) set(text string) {
	d.mu.Lock()
	d.text = text
	d.mu.Unlock()
}

func (d *fakeDisk) serve(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch r.URL.Path {
	case "/rpc/project.ReadFile":
		_ = json.NewEncoder(w).Encode(d.text)
	case "/rpc/project.WriteFile":
		if d.fail {
			http.Error(w, `{"error":"disk full"}`, http.StatusBadRequest)
			return
		}
		var args []api.WriteFileOptions
		_ = json.NewDecoder(r.Body).Decode(&args)
		d.text = args[0].Content
		d.writes <- args[0]
		_, _ = w.Write([]byte("null"))
	default:
		_, _ = w.Write([]byte("null"))
	}
}

func diskBackend(t *testing.T) (*httptest.Server, *fakeDisk) {
	t.Helper()
	d := &fakeDisk{writes: make(chan api.WriteFileOptions, 8)}
	srv := httptest.NewServer(http.HandlerFunc(d.serve))
	t.Cleanup(srv.Close)
	return srv, d
}

// waitSave lands the save in flight, as the next frame would.
func waitSave(t *testing.T, f *fileEditor) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		f.saving.mu.Lock()
		landed := !f.saving.busy && f.saving.done != nil
		f.saving.mu.Unlock()
		if landed {
			f.takeSave()
			return
		}
	}
	t.Fatal("the save never came back")
}

func busy(f *fileEditor) bool {
	f.saving.mu.Lock()
	defer f.saving.mu.Unlock()
	return f.saving.busy
}

// loadedEditor is checkout /c's editor holding a.go, read from d as text.
func loadedEditor(t *testing.T, srv *httptest.Server, d *fakeDisk, text string) *fileEditor {
	t.Helper()
	d.set(text)
	ed, err := codeview.NewTextEditor(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := codeview.NewDiffEditor(fonts.SystemMono, 13, godemirror.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f := newFileEditor(clientOf(srv), func() {}, fonts.SystemMono, 13, "/c")
	f.env.notify = func(ui.Toast) {}
	f.open("a.go", 0, 0, agentNote{})
	f.ed, f.diff, f.loaded = ed, diff, f.opens
	f.ed.OnUpdate = f.updated
	if err := f.setText(text); err != nil {
		t.Fatal(err)
	}
	return f
}

// typeAt inserts text at pos as the user would, then lays the editor out
// once: it reports the edit to OnUpdate at the end of its layout.
func typeAt(t *testing.T, f *fileEditor, pos int, text string) {
	t.Helper()
	if err := f.ed.Dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{{From: pos, To: pos, Insert: text}}, UserEvent: "input.type"}); err != nil {
		t.Fatal(err)
	}
	layOut(t, f)
}

func layOut(t *testing.T, f *fileEditor) {
	t.Helper()
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300)), Now: time.Unix(0, 0)}
	if _, err := f.ed.Layout(gtx); err != nil {
		t.Fatal(err)
	}
}

func TestEditingMarksTheEditorUnsavedAndUndoingClearsIt(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\ntwo\n")
	heard := 0
	f.onDirty = func() { heard++ }
	typeAt(t, f, 0, "x")
	if !f.dirty || heard != 1 {
		t.Fatalf("dirty %v, heard %d after an edit; want unsaved, told once", f.dirty, heard)
	}
	f.ed.Undo()
	layOut(t, f)
	if f.dirty {
		t.Fatal("the editor stayed unsaved once the edit was undone")
	}
}

func TestAnIdleEditorTakesTheDiskOnItsCursorsLine(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "a\nb\nc\n")
	at := f.ed.State().Line(3).From
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(at, at)})
	f.weighDisk("a\nB\nc\nd\n")
	st := f.ed.State()
	if st.Doc() != "a\nB\nc\nd\n" || f.dirty || f.disk.theirs != nil {
		t.Fatalf("doc %q dirty %v warning %v; want the disk's text, clean, no warning", st.Doc(), f.dirty, f.disk.theirs != nil)
	}
	if line := st.LineAt(st.Selection().Main().Head()).Number; line != 3 {
		t.Fatalf("cursor on line %d after the reload, want 3", line)
	}
}

func TestADiskChangeUnderUnsavedEditsWarnsAndWaits(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "a\nb\n")
	typeAt(t, f, 0, "mine ")
	f.weighDisk("a\nagent\n")
	if f.disk.theirs == nil || *f.disk.theirs != "a\nagent\n" {
		t.Fatal("no warning over the user's unsaved edits")
	}
	if got := f.ed.State().Doc(); got != "mine a\nb\n" {
		t.Fatalf("the disk's text replaced the user's: %q", got)
	}
	// Keep mine: the disk's text is what Save now overwrites.
	f.base, f.disk.theirs = *f.disk.theirs, nil
	f.weighDisk("a\nagent\n")
	if f.disk.theirs != nil || !f.dirty {
		t.Fatalf("warning %v dirty %v after Keep mine; want no warning, still unsaved", f.disk.theirs != nil, f.dirty)
	}
	f.weighDisk("a\nagain\n")
	if f.disk.theirs == nil {
		t.Fatal("a second write by the agent raised no warning")
	}
	f.reloadFrom(*f.disk.theirs)
	if got := f.ed.State().Doc(); got != "a\nagain\n" || f.dirty || f.disk.theirs != nil {
		t.Fatalf("after Reload: doc %q dirty %v; want the disk's text, clean", got, f.dirty)
	}
}

func TestSaveWritesTheFilesOwnLineBreaksAndCleansTheEditor(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "a\r\nb\r\n")
	typeAt(t, f, 0, "x")
	saved := false
	f.startSave(func() { saved = true }, false)
	waitSave(t, f)
	if w := <-d.writes; w.Path != "/c" || w.Rel != "a.go" || w.Content != "xa\r\nb\r\n" {
		t.Fatalf("wrote %+v, want /c a.go in CRLF", w)
	}
	if !saved || f.dirty || f.base != "xa\nb\n" {
		t.Fatalf("saved %v dirty %v base %q; want what waited on the save run, clean", saved, f.dirty, f.base)
	}
	// The disk's next read of what was saved is no news.
	f.weighDisk("xa\r\nb\r\n")
	if f.disk.theirs != nil || f.dirty {
		t.Fatal("reading back the saved file raised a warning")
	}
}

// clockedEditor is loadedEditor on a clock the test moves.
func clockedEditor(t *testing.T, text string) (*fileEditor, *fakeDisk, *time.Time) {
	t.Helper()
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, text)
	now := time.Unix(1000, 0)
	f.now = func() time.Time { return now }
	return f, d, &now
}

func frameCtx() C {
	return layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300))}
}

func TestAutoSaveWritesASecondAfterTheLastEdit(t *testing.T) {
	f, d, now := clockedEditor(t, "one\n")
	typeAt(t, f, 0, "a")
	*now = now.Add(800 * time.Millisecond)
	typeAt(t, f, 1, "b")
	*now = now.Add(800 * time.Millisecond)
	if f.autoSave(frameCtx()); busy(f) {
		t.Fatal("saved 0.8s after the last edit, want a second of quiet first")
	}
	*now = now.Add(200 * time.Millisecond)
	if f.autoSave(frameCtx()); !busy(f) {
		t.Fatal("nothing saved a second after the last edit")
	}
	waitSave(t, f)
	if w := <-d.writes; w.Content != "abone\n" || f.dirty {
		t.Fatalf("wrote %q, dirty %v; want the text, and the dot gone", w.Content, f.dirty)
	}
}

func TestAutoSaveNeverWritesOverTheAgentsChange(t *testing.T) {
	f, d, now := clockedEditor(t, "a\nb\n")
	typeAt(t, f, 0, "mine ")
	d.set("a\nagent\n")
	*now = now.Add(autoSaveDelay)
	f.autoSave(frameCtx())
	waitSave(t, f)
	if f.disk.theirs == nil || *f.disk.theirs != "a\nagent\n" || !f.dirty {
		t.Fatalf("warning %v dirty %v; want the agent's change put to the user, the edit unsaved", f.disk.theirs != nil, f.dirty)
	}
	select {
	case w := <-d.writes:
		t.Fatalf("auto-save wrote %q over the agent's change", w.Content)
	default:
	}
	*now = now.Add(time.Hour)
	if f.autoSave(frameCtx()); busy(f) {
		t.Fatal("auto-save ran while the warning waits")
	}
	f.keepMine()
	waitSave(t, f)
	if w := <-d.writes; w.Content != "mine a\nb\n" || f.dirty {
		t.Fatalf("Keep mine wrote %q, dirty %v; want the user's text written", w.Content, f.dirty)
	}
}

func TestAFailedSaveKeepsTheDotAndClosingAsks(t *testing.T) {
	f, d, now := clockedEditor(t, "one\n")
	d.fail = true
	told := 0
	f.env.notify = func(ui.Toast) { told++ }
	typeAt(t, f, 0, "x")
	*now = now.Add(autoSaveDelay)
	f.autoSave(frameCtx())
	waitSave(t, f)
	if !f.dirty || told != 1 {
		t.Fatalf("dirty %v, toasts %d; want the dot kept and the error told once", f.dirty, told)
	}
	*now = now.Add(time.Hour)
	if f.autoSave(frameCtx()); busy(f) {
		t.Fatal("a failed save retried by itself with nothing new to save")
	}
	closed := false
	s := &stageEditors{byCheckout: map[string]*fileEditor{"/c": f}}
	s.saveThen(f, func() { closed = true })
	if !s.ask.open || closed {
		t.Fatalf("asked %v closed %v; want the question before the text is left", s.ask.open, closed)
	}
}

func TestLeavingTheEditorOrTheWindowSavesIt(t *testing.T) {
	f, d, _ := clockedEditor(t, "one\n")
	s := &stageEditors{byCheckout: map[string]*fileEditor{"/c": f}}
	s.follow("/c")
	typeAt(t, f, 0, "x")
	s.follow("")
	if !busy(f) {
		t.Fatal("leaving the editor saved nothing")
	}
	waitSave(t, f)
	<-d.writes
	typeAt(t, f, 0, "y")
	s.flushAll()
	if !busy(f) {
		t.Fatal("the window losing the focus saved nothing")
	}
	waitSave(t, f)
	if w := <-d.writes; w.Content != "yxone\n" {
		t.Fatalf("wrote %q", w.Content)
	}
}

// askFrames draws the stage editors' frame step after asking to leave f's
// text for then.
func askFrames(t *testing.T, f *fileEditor, then func()) (*stageEditors, *frameRig) {
	t.Helper()
	s := &stageEditors{byCheckout: map[string]*fileEditor{"/c": f}, invalidate: func() {}}
	s.saveThen(f, then)
	root := new(ui.Root)
	r := newFrameRig(image.Pt(800, 600), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layoutAsk(gtx, th, root)
			return D{Size: gtx.Constraints.Max}
		})
	})
	return s, r
}

func TestAnotherFileSavesWithoutAQuestion(t *testing.T) {
	f, d, _ := clockedEditor(t, "one\n")
	typeAt(t, f, 0, "x")
	ran := 0
	s, r := askFrames(t, f, func() { ran++ })
	for deadline := time.Now().Add(5 * time.Second); ran == 0 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		r.frame()
	}
	if s.ask.open || ran != 1 || f.dirty {
		t.Fatalf("asked %v ran %d dirty %v; want the file saved and the next one opened, no question", s.ask.open, ran, f.dirty)
	}
	if w := <-d.writes; w.Content != "xone\n" {
		t.Fatalf("wrote %q", w.Content)
	}
}

func TestAFailedSaveAsksAndItsAnswersHoldOrLeave(t *testing.T) {
	f, d, _ := clockedEditor(t, "one\n")
	d.fail = true
	typeAt(t, f, 0, "x")
	f.startSave(nil, false)
	waitSave(t, f)
	ran := 0
	s, r := askFrames(t, f, func() { ran++ })
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if s.ask.open || ran != 0 {
		t.Fatalf("after Cancel: asked %v ran %d; want the question gone and nothing done", s.ask.open, ran)
	}
	_, r = askFrames(t, f, func() { ran++ })
	r.clickLabel(t, i18n.T("dock.editor.discard"))
	if ran != 1 {
		t.Fatalf("after Discard ran %d, want what was asked for done", ran)
	}
	d.mu.Lock()
	d.fail = false
	d.mu.Unlock()
	_, r = askFrames(t, f, func() { ran++ })
	r.clickLabel(t, i18n.T("dock.editor.save"))
	for deadline := time.Now().Add(5 * time.Second); ran != 2 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		r.frame()
	}
	if ran != 2 || f.dirty {
		t.Fatalf("after Save ran %d dirty %v; want the text written and what was asked for done", ran, f.dirty)
	}
}

func TestADiskReadFromBeforeASaveNeverUndoesIt(t *testing.T) {
	f, _, now := clockedEditor(t, "one\n")
	for deadline := time.Now().Add(5 * time.Second); f.disk.seen != "one\n"; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the disk was never read")
		}
		f.watchDisk(frameCtx())
	}
	typeAt(t, f, 0, "a")
	*now = now.Add(autoSaveDelay)
	f.autoSave(frameCtx())
	waitSave(t, f)
	f.watchDisk(frameCtx())
	if doc := f.ed.State().Doc(); doc != "aone\n" || f.disk.theirs != nil {
		t.Fatalf("editor holds %q, warning %v; want the saved text kept, no warning", doc, f.disk.theirs != nil)
	}
}
