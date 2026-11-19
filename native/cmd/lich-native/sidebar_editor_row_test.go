package main

import (
	"image"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func TestAnEditorHangsLastUnderItsCheckout(t *testing.T) {
	b := blockView{under: "/repo", sessions: []sessionView{treeSession("a", "", "/repo"), treeSession("b", "", "/repo")},
		editors: map[string]editorNode{"/repo": {rel: "src/total.ts"}}}
	rows := blockRows(b, b.sessions)
	want := []string{"1:a", "1:b", "1:__editor__:/repo."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if rows[2].kind != rowEditor || rows[2].path != "/repo" || rows[2].editor.name() != "total.ts" {
		t.Fatalf("row %+v, want /repo's editor named total.ts", rows[2])
	}
}

func TestAFolderHangsTheEditorUnderTheCheckoutItGathers(t *testing.T) {
	b := blockView{byCheckout: true, sessions: []sessionView{treeSession("a", "", "/repo"), treeSession("w", "", "/wt")},
		editors: map[string]editorNode{"/wt": {rel: "go.mod"}}}
	got := rowShape(blockRows(b, b.sessions))
	want := []string{"0:__checkout__::/repo", "1:a.", "0:__checkout__::/wt", "1:w", "1:__editor__:/wt."}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestAChildsCheckoutRowHangsItsEditor(t *testing.T) {
	sessions := []sessionView{treeSession("mother", "", "/repo"), treeSession("child", "mother", "/wt")}
	rows := checkoutRows(sessions, "/repo", func(string) bool { return false }, map[string]editorNode{"/wt": {rel: "a.go"}})
	want := []string{"1:mother.", "2:__checkout__:mother:/wt.", "3:child", "3:__editor__:mother:/wt."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// editorModel is one project at /repo: a1 pinned, a2 in the root checkout,
// so /repo is drawn by its own block and by the pinned block's group.
func editorModel(t *testing.T) *model {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("null")) }))
	t.Cleanup(srv.Close)
	m := newModel(clientOf(srv), func() {})
	m.projects = []backend.Project{{ID: "a", Name: "alpha", Path: "/repo", ActiveSessionID: "a2",
		Sessions: []backend.Session{{ID: "a1", Label: "A one", Pinned: true}, {ID: "a2", Label: "A two"}}}}
	m.setActiveProject("a")
	return m
}

func editorsOf(v sidebarView) map[string]int {
	count := map[string]int{}
	for _, b := range v.blocks {
		for _, r := range blockRows(b, b.sessions) {
			if r.kind == rowEditor {
				count[r.path+" "+r.editor.name()]++
			}
		}
	}
	return count
}

func TestACheckoutHasOneEditorNodeNamedAfterItsFile(t *testing.T) {
	m := editorModel(t)
	m.openEditor("/repo", "src/a.go")
	m.markEditorDirty("/repo", true)
	view := func() sidebarView { return m.sidebarBlocks(sidebarFilter{}) }
	if got := editorsOf(view()); len(got) != 1 || got["/repo a.go"] != 1 {
		t.Fatalf("editor nodes %v, want one for /repo named a.go", got)
	}
	m.openEditor("/repo", "README.md")
	if got := editorsOf(view()); len(got) != 1 || got["/repo README.md"] != 1 {
		t.Fatalf("editor nodes %v, want the one node renamed to README.md", got)
	}
	m.mu.Lock()
	n, _ := m.editorOf("/repo")
	m.mu.Unlock()
	if n.dirty || !n.shown {
		t.Errorf("node %+v: another file starts clean, and on the stage", n)
	}
}

func TestPickingASessionTakesTheEditorOffTheStageAndTheNodeBringsItBack(t *testing.T) {
	m := editorModel(t)
	m.openEditor("/repo", "a.go")
	m.setActive(t.Context(), "a1")
	if got := m.editorShown(); got != "" {
		t.Fatalf("editor shown %q after a session was picked, want none", got)
	}
	m.showEditor("/repo")
	if got := m.editorShown(); got != "/repo" {
		t.Fatalf("editor shown %q after its node was picked, want /repo", got)
	}
	m.closeEditor("/repo")
	if got, nodes := m.editorShown(), editorsOf(m.sidebarBlocks(sidebarFilter{})); got != "" || len(nodes) != 0 {
		t.Fatalf("after Close: shown %q, nodes %v; want neither", got, nodes)
	}
}

func TestTheRailDrawsTheEditorNode(t *testing.T) {
	b := blockView{under: "/repo", sessions: []sessionView{treeSession("a", "", "/repo")}, editors: map[string]editorNode{"/repo": {rel: "a.go"}}}
	items := railRows(b)
	if len(items) != 2 || items[1].kind != railEditor || items[1].path != "/repo" {
		t.Fatalf("rail items %+v, want the session then /repo's editor", items)
	}
}

// editorFrames is the open sidebar over editorModel with /repo's editor
// holding a.go, and the action of the last frame.
func editorFrames(t *testing.T) (*model, *frameRig, *sidebarAction) {
	t.Helper()
	m := editorModel(t)
	m.openEditor("/repo", "src/a.go")
	m.setActive(t.Context(), "a2")
	root := new(ui.Root)
	s := newSidebar(root)
	s.open = true
	act := new(sidebarAction)
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			_, a := s.layout(gtx, th, m, active(m))
			if a.editor != "" || a.editorClose != "" || a.activate != "" {
				*act = a
			}
			return D{Size: gtx.Constraints.Max}
		})
	})
	return m, r, act
}

func TestTheEditorNodeOpensTheEditorInTheFrameThatTookTheClick(t *testing.T) {
	_, r, act := editorFrames(t)
	r.clickLabel(t, "a.go")
	if act.editor != "/repo" {
		t.Fatalf("act %+v, want /repo's editor shown", *act)
	}
	*act = sidebarAction{}
	r.clickLabel(t, "A two")
	if act.activate != "a2" {
		t.Fatalf("act %+v, want the session picked", *act)
	}
}

func TestTheEditorNodesMenuClosesIt(t *testing.T) {
	_, r, act := editorFrames(t)
	r.settle(t)
	at := r.at(t, "a.go")
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
}

// active is the session the sidebar lights: none while the editor is on the
// stage, as sidebarActive has it.
func active(m *model) string {
	if m.editorShown() != "" {
		return ""
	}
	sv, _ := m.activeSession()
	return sv.ID
}
