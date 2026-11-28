package main

import (
	"image"
	"testing"

	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// artifactWindow is a window over editorModel holding just what putting an
// artifact on the stage touches.
func artifactWindow(t *testing.T) (*window, *model) {
	t.Helper()
	m := editorModel(t)
	u := &window{model: m, settings: new(settingsScreen), pulls: new(pullsScreen), extension: new(extensionScreen)}
	u.editors = stageEditors{client: m.client, invalidate: func() {}, dirty: m.markEditorDirty, saved: func(string) {}}
	return u, m
}

func TestAMarkdownArtifactOpensOnTheStageRenderedInTheSessionsCheckoutEditor(t *testing.T) {
	u, m := artifactWindow(t)
	u.openArtifactInEditor("a2", backend.Artifact{Name: "notes.md", Path: osPath("/art/notes.md")})
	if m.editorShown() != "/repo" {
		t.Fatalf("stage shows %q's editor, want /repo's", m.editorShown())
	}
	n, _ := m.editorOf("/repo")
	f := u.editors.byCheckout["/repo"]
	if n.name() != "notes.md" || f.dir != osPath("/art") || f.rel != "notes.md" || f.md.mode != mdRendered {
		t.Fatalf("node %q, editor %q in %q mode %d; want notes.md of /art, rendered", n.name(), f.rel, f.dir, f.md.mode)
	}
}

func TestACheckoutMarkdownOpensInTheCheckoutAfterAnArtifact(t *testing.T) {
	u, _ := artifactWindow(t)
	u.openArtifactInEditor("a2", backend.Artifact{Name: "notes.md", Path: "/art/notes.md"})
	u.openInEditor("/repo", "notes.md", 0, 0, agentNote{})
	f := u.editors.byCheckout["/repo"]
	if f.dir != "" || f.md.mode != mdRendered {
		t.Fatalf("checkout's notes.md opened in %q mode %d, want it rendered in the checkout", f.dir, f.md.mode)
	}
}

func TestAnArtifactIsReadOnly(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\ntwo\n")
	f.openArtifact("/art", "notes.md")
	f.loaded = f.opens
	if err := f.setText("one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	f.md.mode = mdSource
	typeAt(t, f, 0, "x")
	if f.dirty || f.ed.State().Doc() != "one\ntwo\n" {
		t.Fatalf("dirty %v, text %q after typing; want the text untouched and no unsaved mark", f.dirty, f.ed.State().Doc())
	}
	f.startSave(nil, true)
	f.autoSave(frameCtx())
	if busy(f) || len(d.writes) != 0 {
		t.Fatal("an artifact was written")
	}
}

func TestAnArtifactsHeaderSaysItIsTheAgentsAndReadOnly(t *testing.T) {
	f := newFileEditor(nil, func() {}, "", 13, "/c")
	f.openArtifact("/art", "notes.md")
	r := newFrameRig(image.Pt(600, 60), func(gtx C, th *ui.Theme) { f.header(gtx, th) })
	r.settle(t)
	if !r.shows(i18n.T("dock.editor.artifactReadOnly")) {
		t.Fatalf("header drew %q, want the read-only note", r.drawn())
	}
}

func TestClosingAnArtifactFromItsNodesMenuTakesItOffTheStage(t *testing.T) {
	u, m := artifactWindow(t)
	u.openArtifactInEditor("a2", backend.Artifact{Name: "notes.md", Path: "/art/notes.md"})
	root := new(ui.Root)
	s := newSidebar(root)
	s.open = true
	act := new(sidebarAction)
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			_, a := s.layout(gtx, th, m, active(m))
			if a.editorClose != "" {
				*act = a
			}
			return D{Size: gtx.Constraints.Max}
		})
	})
	r.settle(t)
	at := r.at(t, "notes.md")
	r.hover(at)
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	r.frame()
	r.clickLabel(t, i18n.T("sidebar.editorRow.close"))
	if act.editorClose != "/repo" {
		t.Fatalf("act %+v, want /repo's editor closed", *act)
	}
	u.applyEditor(*act)
	if _, ok := m.editorOf("/repo"); ok || m.editorShown() != "" || u.editors.byCheckout["/repo"] != nil {
		t.Fatal("the artifact's editor is still there after Close")
	}
}
