package main

import (
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
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

// writes stands in for the backend's project.WriteFile, handing each write
// to the test.
func writes(t *testing.T) (*httptest.Server, chan api.WriteFileOptions) {
	t.Helper()
	got := make(chan api.WriteFileOptions, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/project.WriteFile" {
			var args []api.WriteFileOptions
			_ = json.NewDecoder(r.Body).Decode(&args)
			got <- args[0]
		}
		_, _ = w.Write([]byte("null"))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

// loadedEditor is checkout /c's editor holding a.go, read as text.
func loadedEditor(t *testing.T, srv *httptest.Server, text string) *fileEditor {
	t.Helper()
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
	srv, _ := writes(t)
	f := loadedEditor(t, srv, "one\ntwo\n")
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
	srv, _ := writes(t)
	f := loadedEditor(t, srv, "a\nb\nc\n")
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
	srv, _ := writes(t)
	f := loadedEditor(t, srv, "a\nb\n")
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
	srv, got := writes(t)
	f := loadedEditor(t, srv, "a\r\nb\r\n")
	typeAt(t, f, 0, "x")
	saved := false
	f.startSave(func() { saved = true })
	select {
	case w := <-got:
		if w.Path != "/c" || w.Rel != "a.go" || w.Content != "xa\r\nb\r\n" {
			t.Fatalf("wrote %+v, want /c a.go in CRLF", w)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Save wrote nothing")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !saved && time.Now().Before(deadline) {
		f.takeSave()
		time.Sleep(time.Millisecond)
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

// askFrames draws the Save / Discard / Cancel question over f's unsaved
// changes, asked for then.
func askFrames(t *testing.T, f *fileEditor, then func()) (*stageEditors, *frameRig) {
	t.Helper()
	s := &stageEditors{byCheckout: map[string]*fileEditor{"/c": f}, invalidate: func() {}}
	s.unlessUnsaved(f, then)
	root := new(ui.Root)
	r := newFrameRig(image.Pt(800, 600), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layoutAsk(gtx, th, root)
			return D{Size: gtx.Constraints.Max}
		})
	})
	return s, r
}

func TestAnotherFileOverUnsavedChangesAsksFirst(t *testing.T) {
	srv, got := writes(t)
	f := loadedEditor(t, srv, "one\n")
	ran := 0
	if s, _ := askFrames(t, f, func() { ran++ }); s.ask.open || ran != 1 {
		t.Fatalf("asked %v ran %d with nothing unsaved; want it done at once", s.ask.open, ran)
	}
	typeAt(t, f, 0, "x")
	s, r := askFrames(t, f, func() { ran++ })
	if !s.ask.open || ran != 1 {
		t.Fatalf("asked %v ran %d over unsaved changes; want the question first", s.ask.open, ran)
	}
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if s.ask.open || ran != 1 {
		t.Fatalf("after Cancel: asked %v ran %d; want the question gone and nothing done", s.ask.open, ran)
	}
	_, r = askFrames(t, f, func() { ran++ })
	r.clickLabel(t, i18n.T("dock.editor.discard"))
	if ran != 2 {
		t.Fatalf("after Discard ran %d, want what was asked for done", ran)
	}
	_, r = askFrames(t, f, func() { ran++ })
	r.clickLabel(t, i18n.T("dock.editor.save"))
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("Save in the question wrote nothing")
	}
	// The editor is not laid out here, as when it is off the stage: the
	// question's own frames land the save and run what waited on it.
	for deadline := time.Now().Add(5 * time.Second); ran != 3 && time.Now().Before(deadline); {
		r.frame()
		time.Sleep(time.Millisecond)
	}
	if ran != 3 || f.dirty {
		t.Fatalf("after Save ran %d dirty %v; want what was asked for done once the save landed", ran, f.dirty)
	}
}
