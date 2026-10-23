package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"

	"github.com/lichdotdev/lich/api"
)

const twoHunks = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,3 @@
 one
-two
+TWO
 three
@@ -10,2 +10,3 @@
 ten
+ten-and-a-half
 eleven
`

func onlyFile(t *testing.T, patch string) patchFile {
	t.Helper()
	files, err := parsePatchFiles(patch)
	if err != nil || len(files) != 1 {
		t.Fatalf("parsePatchFiles = %+v, %v; want one file", files, err)
	}
	return files[0]
}

func TestCardIsAsTallAsTheRowsGodemirrorLaysOut(t *testing.T) {
	f := onlyFile(t, twoHunks)
	cases := []struct {
		name   string
		split  bool
		filled map[int]int
		want   int
	}{
		{"unified, gap closed", false, nil, 8},
		{"side by side pairs a change's two sides", true, nil, 7},
		{"gap partly pulled in keeps its separator", false, map[int]int{9: 2}, 10},
		{"gap pulled in whole drops its separator", false, map[int]int{9: 6}, 13},
	}
	for _, c := range cases {
		if got := diffRows(f, c.split, c.filled); got != c.want {
			t.Errorf("%s: diffRows = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestPatchFilesKnowTheirStatus(t *testing.T) {
	patch := "diff --git a/new.txt b/new.txt\nnew file mode 100644\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+hi\n" +
		"diff --git a/old.go b/moved.go\nsimilarity index 90%\nrename from old.go\nrename to moved.go\n" +
		"diff --git a/logo.png b/logo.png\nBinary files a/logo.png and b/logo.png differ\n"
	files, err := parsePatchFiles(patch)
	if err != nil || len(files) != 3 {
		t.Fatalf("parsePatchFiles = %+v, %v", files, err)
	}
	if files[0].status != statusAdded || files[0].path != "new.txt" {
		t.Errorf("added file read as %+v", files[0])
	}
	if files[1].status != statusRenamed || files[1].oldPath != "old.go" || files[1].path != "moved.go" {
		t.Errorf("rename read as %+v", files[1])
	}
	if got := discardTargets(files[1]); strings.Join(got, ",") != "moved.go,old.go" {
		t.Errorf("discarding a rename reverts %v, want both paths", got)
	}
	if !files[2].binary {
		t.Errorf("binary file read as %+v", files[2])
	}
}

func addedLines(n int) string {
	var b strings.Builder
	b.WriteString("diff --git a/big.lock b/big.lock\nnew file mode 100644\n--- /dev/null\n+++ b/big.lock\n")
	b.WriteString("@@ -0,0 +1," + strconv.Itoa(n) + " @@\n")
	for i := range n {
		b.WriteString("+line " + strconv.Itoa(i) + "\n")
	}
	return b.String()
}

func TestBigFilesStartCollapsed(t *testing.T) {
	if c := newFileCard(nil, onlyFile(t, addedLines(largeFileRows)), false, 0); !c.open {
		t.Error("a file of exactly the bar starts collapsed, want open")
	}
	if c := newFileCard(nil, onlyFile(t, addedLines(largeFileRows+1)), false, 0); c.open {
		t.Error("a file past the bar starts open, want collapsed")
	}
}

func TestBinaryFilesOpenWhenTheyHaveAPreview(t *testing.T) {
	p := &reviewPanel{}
	img := onlyFile(t, "diff --git a/a.png b/a.png\nBinary files a/a.png and b/a.png differ\n")
	if c := newFileCard(p, img, false, 0); !c.open {
		t.Error("a changed image starts collapsed, want open: its two sides are the diff")
	}
	zip := onlyFile(t, "diff --git a/a.zip b/a.zip\nBinary files a/a.zip and b/a.zip differ\n")
	if c := newFileCard(p, zip, false, 0); c.open {
		t.Error("a binary with no preview starts open, want collapsed")
	}
	p.pr = &prFiles{}
	if c := newFileCard(p, img, false, 0); !c.open {
		t.Error("an image in a pull request starts collapsed, want open: its sides are read from the clone or GitHub")
	}
	if c := newFileCard(p, zip, false, 0); c.open {
		t.Error("a binary with no preview in a pull request starts open, want collapsed")
	}
}

func TestPatchKeepsTheBlobIDsOfABinary(t *testing.T) {
	f := onlyFile(t, "diff --git a/a.png b/a.png\nindex 1a2b3c4..5d6e7f8 100644\nBinary files a/a.png and b/a.png differ\n")
	if f.blobs != "1a2b3c4..5d6e7f8" {
		t.Errorf("blobs = %q, want the index line's ids", f.blobs)
	}
}

func TestPRFileURLAnchorsTheFile(t *testing.T) {
	got := prFileURL("https://github.com/o/r/pull/7/", "frontend/public/og-card.png")
	want := "https://github.com/o/r/pull/7/files#diff-d90a4ea99bc7c7480992c895a9d777870268f64a5b9b12c354b632cc7c21b6dd"
	if got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestARefetchKeepsAHandFoldedFileFolded(t *testing.T) {
	f := onlyFile(t, twoHunks)
	c := newFileCard(nil, f, false, 0)
	c.open = false
	c.sync(f, false)
	if c.open {
		t.Fatal("the same text read again reopened the file")
	}
	c.sync(onlyFile(t, strings.Replace(twoHunks, "+TWO", "+Two", 1)), false)
	if !c.open {
		t.Fatal("a new version of the file stayed folded")
	}
}

func TestHidingWhitespaceFoldsOnlyWhitespaceBlocks(t *testing.T) {
	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" +
		"@@ -1,2 +1,2 @@\n-  call(x)\n+\tcall( x )\n ctx\n" +
		"@@ -9,2 +9,2 @@\n-a b\n+a c\n ctx\n"
	f := onlyFile(t, patch)
	shown := hideWhitespace(f)
	if len(shown.hunks) != 1 || shown.hunks[0].lines[0] != "-a b" {
		t.Fatalf("hidden = %+v, want only the real change left", shown.hunks)
	}
	if whitespaceOnly(shown, f) {
		t.Fatal("a file with a real change reads as whitespace only")
	}
	onlySpaces := onlyFile(t, "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x  = 1\n+x = 1\n")
	if !whitespaceOnly(hideWhitespace(onlySpaces), onlySpaces) {
		t.Fatal("a whitespace-only file does not read as one")
	}
}

func TestStickyHeaderIsPushedUpByTheNextCard(t *testing.T) {
	cases := []struct {
		offset, card, header, wantY int
		want                        bool
	}{
		{0, 300, 30, 0, false},
		{100, 300, 30, 0, true},
		{285, 300, 30, -15, true},
		{300, 300, 30, 0, false},
	}
	for _, c := range cases {
		y, ok := stickyAt(c.offset, c.card, c.header)
		if y != c.wantY || ok != c.want {
			t.Errorf("stickyAt(%d, %d, %d) = %d, %v; want %d, %v", c.offset, c.card, c.header, y, ok, c.wantY, c.want)
		}
	}
}

// fakeRevert serves RevertLines with undo and records every RestoreLines.
func fakeRevert(t *testing.T, undo backend.RevertResult, restored chan<- []json.RawMessage) *backend.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc/project.RevertLines":
			_ = json.NewEncoder(w).Encode(undo)
		case "/rpc/project.RestoreLines":
			var args []json.RawMessage
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &args)
			restored <- args
			_ = json.NewEncoder(w).Encode(nil)
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

func TestARevertsUndoPutsTheLinesBack(t *testing.T) {
	undo := backend.RevertResult{Patch: "the patch", IndexPatch: "the index"}
	restored := make(chan []json.RawMessage, 1)
	toasts := make(chan ui.Toast, 1)
	p := &reviewPanel{client: fakeRevert(t, undo, restored), target: "/repo", notify: func(t ui.Toast) { toasts <- t }}
	p.revert("internal/a.go", []backend.RevertLine{{Side: "new", Line: 3, Text: "x"}, {Side: "old", Line: 3, Text: "y"}})
	toast := <-toasts
	if toast.Title != "Reverted +1 -1 in a.go" || toast.Duration != revertUndoFor || toast.Action == nil {
		t.Fatalf("toast = %+v, want the revert's stat and a long-lived Undo", toast)
	}
	toast.Action.OnClick()
	select {
	case args := <-restored:
		var got api.RestoreLinesOptions
		_ = json.Unmarshal(args[0], &got)
		if got.Path != "/repo" || got.Rel != "internal/a.go" || got.Undo != undo {
			t.Fatalf("RestoreLines(%s) — want the checkout, the file and the revert's own undo", args)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Undo never called RestoreLines")
	}
}

func TestHidingWhitespaceKeepsLinesPulledIntoAnUntouchedFile(t *testing.T) {
	c := newFileCard(nil, onlyFile(t, twoHunks), false, 0)
	c.filled[9] = 2
	c.sync(c.git, true)
	if got := diffRows(c.shown, false, c.filled); got != 10 {
		t.Fatalf("rows after hiding whitespace = %d, want 10: the diff drawn did not change, so its gap stays filled", got)
	}
}

func TestHideWhitespaceIsReadFromTheEditorSetting(t *testing.T) {
	f, client := newSidebarBackend(t)
	cases := []struct {
		stored string
		set    bool
		want   bool
	}{
		{"", false, false},
		{"false", true, false},
		{"true", true, true},
	}
	for _, c := range cases {
		delete(f.settings, hideWhitespaceKey+"@")
		if c.set {
			f.settings[hideWhitespaceKey+"@"] = c.stored
		}
		if got, err := loadHideWhitespace(t.Context(), client); err != nil || got != c.want {
			t.Errorf("stored %q: hiding = %v, %v; want %v", c.stored, got, err, c.want)
		}
	}
}
