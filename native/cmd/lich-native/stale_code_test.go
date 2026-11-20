package main

import (
	"reflect"

	"encoding/json"
	"gioui.org/f32"
	"gioui.org/io/key"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// codeBackend answers the Code tab's reads of a checkout holding one file,
// a.go, whose second line holds "needle".
func codeBackend(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answers := map[string]any{
			"project.Tree":     fileListing{Files: []string{"a.go"}},
			"project.Search":   searchResult{Matches: []searchMatch{{Path: "a.go", Line: 2, Text: "needle"}}},
			"project.ReadFile": "package a\nneedle\nNeedle\n",
		}
		_ = json.NewEncoder(w).Encode(answers[strings.TrimPrefix(r.URL.Path, "/rpc/")])
	}))
	t.Cleanup(srv.Close)
	return srv
}

// codeFrames is the Code tab over codeBackend, read as a plain folder, and
// the last file it opened in the stage editor.
func codeFrames(t *testing.T) (*codeTab, *frameRig, *codeTarget) {
	t.Helper()
	r := newFrameRig(image.Pt(500, 600), nil)
	opened := new(codeTarget)
	c := newCodeTab(clientOf(codeBackend(t)), r.invalidate, new(commentStore), func(ui.Toast) {}, func(checkout, rel string, line int) {
		*opened = codeTarget{session: checkout, rel: rel, line: line}
	})
	root := new(ui.Root)
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			c.layout(gtx, th, dockInput{checkout: "/repo"})
			return D{Size: gtx.Constraints.Max}
		})
	}
	return c, r, opened
}

func TestCodeTreeOpensAFileInTheEditorInTheFrameThatTookTheClick(t *testing.T) {
	_, r, opened := codeFrames(t)
	r.waitFor(t, "a.go")
	r.click(t, topmost(t, r, "a.go"))
	if want := (codeTarget{session: "/repo", rel: "a.go"}); *opened != want {
		t.Fatalf("opened %+v, want %+v", *opened, want)
	}
}

func TestCodeSearchHitOpensInTheEditorInTheFrameThatTookTheClick(t *testing.T) {
	c, r, opened := codeFrames(t)
	r.waitFor(t, "a.go")
	c.openSearch()
	c.query.SetText("needle")
	r.waitFor(t, "needle")
	r.click(t, r.at(t, "needle"))
	if want := (codeTarget{session: "/repo", rel: "a.go", line: 2}); *opened != want {
		t.Fatalf("opened %+v, want %+v", *opened, want)
	}
}

func TestCodeSearchBarStaysOutOfSightUntilOpenedAndEscapeFoldsItOnceTheFieldIsEmpty(t *testing.T) {
	c, r, _ := codeFrames(t)
	r.waitFor(t, "a.go")
	placeholder := i18n.T("dock.filesPanel.placeholder")
	if r.shows(placeholder) {
		t.Fatal("the search field is on screen before anything opened it")
	}
	c.openSearch()
	r.waitFor(t, placeholder)
	c.query.SetText("a.go")
	r.settle(t)
	r.press(key.NameEscape)
	if !c.searching || c.query.Text() != "" {
		t.Fatalf("first Escape: searching=%v query=%q, want the bar open and the field empty", c.searching, c.query.Text())
	}
	r.press(key.NameEscape)
	if c.searching {
		t.Fatal("Escape on an empty field left the bar open")
	}
}

func TestCodeSearchListsFilesByNameThenLinesByContentAndOpensEither(t *testing.T) {
	c, r, opened := codeFrames(t)
	r.waitFor(t, "a.go")
	c.openSearch()
	c.query.SetText("a.go")
	r.waitFor(t, "needle")
	for _, section := range []string{i18n.T("dock.searchResults.files"), i18n.T("dock.searchResults.inContent")} {
		if !r.shows(strings.ToUpper(section)) {
			t.Fatalf("no %q section:\n%s", section, r.drawn())
		}
	}
	r.click(t, r.at(t, "needle"))
	if want := (codeTarget{session: "/repo", rel: "a.go", line: 2}); *opened != want {
		t.Fatalf("the line opened %+v, want %+v", *opened, want)
	}
	r.click(t, topmost(t, r, "a.go"))
	if want := (codeTarget{session: "/repo", rel: "a.go"}); *opened != want {
		t.Fatalf("the file opened %+v, want %+v", *opened, want)
	}
}

func TestSearchRowsPutFilesFirstAndNumberThemWithTheHitsForFocus(t *testing.T) {
	var c codeTab
	groups := []searchGroup{{path: "a.go", hits: []searchMatch{{Path: "a.go", Line: 1}, {Path: "a.go", Line: 5}}}}
	rows := c.searchRows([]string{"x/a.go", "a.go"}, groups, searchResult{Matches: groups[0].hits}, false, nil)
	var kinds []resultKind
	var focus []int
	for _, r := range rows {
		kinds = append(kinds, r.kind)
		if r.kind == resultFile || r.kind == resultHit {
			focus = append(focus, r.hit)
		}
	}
	wantKinds := []resultKind{resultSection, resultFile, resultFile, resultSection, resultGroup, resultHit, resultHit}
	if !reflect.DeepEqual(kinds, wantKinds) || !reflect.DeepEqual(focus, []int{0, 1, 2, 3}) {
		t.Errorf("kinds %v focus %v, want %v and 0..3", kinds, focus, wantKinds)
	}
	if c.hits.hits != 4 || len(c.hits.clicks) != 4 {
		t.Errorf("focusable %d, clickables %d, want 4 of each", c.hits.hits, len(c.hits.clicks))
	}
}

// topmost is where the highest node drawn with label is: a name that rows of
// two sections both show.
func topmost(t *testing.T, r *frameRig, label string) f32.Point {
	t.Helper()
	var best image.Point
	found := false
	for _, n := range r.nodes() {
		if b := n.Desc.Bounds.Min; n.Desc.Label == label && (!found || b.Y < best.Y) {
			best, found = b, true
		}
	}
	if !found {
		t.Fatalf("nothing drawn as %q:\n%s", label, r.drawn())
	}
	return f32.Pt(float32(best.X+2), float32(best.Y+2))
}
