package main

import (
	"encoding/json"
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

// codeFrames is the Code tab over codeBackend, read as a plain folder.
func codeFrames(t *testing.T) (*codeTab, *frameRig) {
	t.Helper()
	r := newFrameRig(image.Pt(500, 600), nil)
	c, err := newCodeTab(clientOf(codeBackend(t)), r.invalidate, "monospace", 12, new(commentStore), func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	root := new(ui.Root)
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			if err := c.layout(gtx, th, root, dockInput{checkout: "/repo"}); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return c, r
}

func TestCodeTreeOpensAFileInTheFrameAfterTheClick(t *testing.T) {
	c, r := codeFrames(t)
	r.waitFor(t, "a.go")
	r.click(t, r.at(t, "a.go"))
	if !c.preview.isOpen() {
		t.Fatal("the click missed the file")
	}
}

func TestCodeSearchHitOpensInTheFrameAfterTheClick(t *testing.T) {
	c, r := codeFrames(t)
	r.waitFor(t, "a.go")
	c.mode.Value = browseByText
	c.query.SetText("needle")
	r.waitFor(t, "needle")
	r.click(t, r.at(t, "needle"))
	if !c.preview.isOpen() {
		t.Fatal("the click missed the hit")
	}
}

func TestCodeBrowseModeSaysWhatTheFieldDoesInTheFrameThatTookTheClick(t *testing.T) {
	c, r := codeFrames(t)
	r.waitFor(t, "a.go")
	c.mode.Value = browseByText
	r.clickLabel(t, i18n.T("dock.filesPanel.byName"))
	if c.mode.Value != browseByName {
		t.Fatal("the click missed By name")
	}
}

func TestCodeFindMatchCaseCountsInTheFrameThatTookTheClick(t *testing.T) {
	c, r := codeFrames(t)
	r.waitFor(t, "a.go")
	c.open("a.go", 0)
	c.preview.find.open = true
	c.preview.find.query.SetText("needle")
	status := i18n.T("dock.findBar.matches", i18n.Args{"count": 2})
	r.waitFor(t, status)
	// Match case is the first toggle right of the status.
	var at image.Point
	for _, n := range r.nodes() {
		if n.Desc.Label == status {
			at = n.Desc.Bounds.Min
		}
	}
	r.click(t, r.smallestClickIn(t, image.Rect(at.X, at.Y-10, at.X+100, at.Y+22)))
	if !c.preview.find.matchCase.Value {
		t.Fatal("the click missed Match case")
	}
}
