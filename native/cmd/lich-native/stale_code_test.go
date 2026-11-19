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
	r.click(t, r.at(t, "a.go"))
	if want := (codeTarget{session: "/repo", rel: "a.go"}); *opened != want {
		t.Fatalf("opened %+v, want %+v", *opened, want)
	}
}

func TestCodeSearchHitOpensInTheEditorInTheFrameThatTookTheClick(t *testing.T) {
	c, r, opened := codeFrames(t)
	r.waitFor(t, "a.go")
	c.mode.Value = browseByText
	c.query.SetText("needle")
	r.waitFor(t, "needle")
	r.click(t, r.at(t, "needle"))
	if want := (codeTarget{session: "/repo", rel: "a.go", line: 2}); *opened != want {
		t.Fatalf("opened %+v, want %+v", *opened, want)
	}
}

func TestCodeBrowseModeSaysWhatTheFieldDoesInTheFrameThatTookTheClick(t *testing.T) {
	c, r, _ := codeFrames(t)
	r.waitFor(t, "a.go")
	c.mode.Value = browseByText
	r.clickLabel(t, i18n.T("dock.filesPanel.byName"))
	if c.mode.Value != browseByName {
		t.Fatal("the click missed By name")
	}
}
