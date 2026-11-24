package main

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func TestAFolderHeaderSaysItsNameAsTypedAndCountsItsRoots(t *testing.T) {
	m := modelOn("a")
	m.projects[0].Sessions = append(m.projects[0].Sessions,
		backend.Session{ID: "cut", Label: "Changelog cut", Path: "/a/wt/r", Folder: "Release"},
		backend.Session{ID: "shell", Label: "Shell", Kind: "shell", Path: "/a/wt/r", Folder: "Release", OriginSessionID: "cut"},
		backend.Session{ID: "notes", Label: "Translate notes", Path: "/a/wt/r", Folder: "Release"},
	)
	root := new(ui.Root)
	s := newSidebar(root)
	s.open = true
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layout(gtx, th, m, "a1")
			return D{Size: gtx.Constraints.Max}
		})
	})
	r.settle(t)
	if !r.shows("Release") || r.shows("RELEASE") {
		t.Errorf("the header is not the name as typed:\n%s", r.drawn())
	}
	if !r.shows("2") || r.shows("3") {
		t.Errorf("the count is not the folder's two roots, the shell under cut left out:\n%s", r.drawn())
	}
}
