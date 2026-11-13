package main

import (
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func gitRefsBackend(t *testing.T) *backend.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		var out any
		switch strings.TrimPrefix(r.URL.Path, "/rpc/") {
		case "project.GitRefs":
			out = backend.GitRefs{Head: "aaaaaaaaaaaa", Stamp: "1", Refs: []backend.GitRef{
				{Name: "main", Kind: backend.GitRefLocal, Head: true}, {Name: "feature", Kind: backend.GitRefLocal}}}
		case "project.GitLog":
			out = backend.GitLogPage{Commits: []backend.GitCommit{{Hash: "aaaaaaaaaaaa", Author: "me", Subject: "first"}}}
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

func TestBranchMenuButtonOpensTheMenuWithoutSelectingTheBranch(t *testing.T) {
	r := newFrameRig(image.Pt(1200, 400), nil)
	var root ui.Root
	g := newGitPanel(t.Context(), gitRefsBackend(t), &root, r.invalidate, func(ui.Toast) {}, "monospace", 12)
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			g.layout(gtx, th, gitPanelInput{path: "/repo"})
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.waitFor(t, "feature")
	row := r.at(t, "feature")
	r.hover(row)
	more := r.smallestClickIn(t, image.Rect(int(row.X), int(row.Y)-rowReach, r.size.X, int(row.Y)+rowReach))
	r.queueClick(more)
	r.frame()
	if !g.treeRows["refs/heads/feature"].menu.Opened() {
		t.Fatal("the click missed the branch's menu button")
	}
	if g.filter.ref != "" {
		t.Errorf("the menu button narrowed the log to %q", g.filter.ref)
	}
}
