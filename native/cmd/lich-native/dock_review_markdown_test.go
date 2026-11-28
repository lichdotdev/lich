package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

const (
	editedReadme = "diff --git a/README.md b/README.md\nindex 1111111..2222222 100644\n--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old\n+new\n"
	addedReadme  = "diff --git a/docs/a.md b/docs/a.md\nnew file mode 100644\n--- /dev/null\n+++ b/docs/a.md\n@@ -0,0 +1,2 @@\n+# Title\n+body\n\\ No newline at end of file\n"
	gonePage     = "diff --git a/b.markdown b/b.markdown\ndeleted file mode 100644\n--- a/b.markdown\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone\n"
)

func TestMarkdownPreviewIsOfferedWhereTheNewVersionIsReadable(t *testing.T) {
	tree, pr := &reviewPanel{}, &reviewPanel{pr: &prFiles{}}
	for _, tc := range []struct {
		name  string
		panel *reviewPanel
		patch string
		want  bool
	}{
		{"an edited file in the working tree", tree, editedReadme, true},
		{"an added file in the working tree", tree, addedReadme, true},
		{"an added file in a pull request", pr, addedReadme, true},
		{"an edited file in a pull request, read at its head", pr, editedReadme, true},
		{"a deleted file", tree, gonePage, false},
		{"a file that is not Markdown", tree, twoHunks, false},
	} {
		if got := newFileCard(tc.panel, onlyFile(t, tc.patch), false, 0).previewsMarkdown(); got != tc.want {
			t.Errorf("%s: previewsMarkdown = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAnAddedFilesPreviewIsItsPatchLines(t *testing.T) {
	if got := addedText(onlyFile(t, addedReadme)); got != "# Title\nbody\n" {
		t.Errorf("addedText = %q, want the added lines without git's no-newline marker", got)
	}
}

func TestANewVersionOfTheFileGoesBackToTheDiff(t *testing.T) {
	f := onlyFile(t, editedReadme)
	c := newFileCard(&reviewPanel{}, f, false, 0)
	c.previewing = true
	c.sync(f, false)
	if !c.previewing {
		t.Fatal("the same text read again left the preview")
	}
	c.sync(onlyFile(t, strings.Replace(editedReadme, "+new", "+newer", 1)), false)
	if c.previewing {
		t.Fatal("a new version of the file stayed in the preview")
	}
}

func TestRouteChatLink(t *testing.T) {
	for _, tt := range []struct {
		name, session, url, want string
	}{
		{name: "absolute path", session: "s1", url: filepath.Join(t.TempDir(), "artifacts", "report.html"), want: "browser"},
		{name: "web URL", session: "s1", url: "https://example.com", want: "external"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var where, gotSession, gotURL string
			routeChatLink(tt.session, tt.url,
				func(session, url string) { where, gotSession, gotURL = "browser", session, url },
				func(url string) { where, gotURL = "external", url },
			)
			if where != tt.want || gotURL != tt.url {
				t.Fatalf("route = %s %q, want %s %q", where, gotURL, tt.want, tt.url)
			}
			if tt.want == "browser" && gotSession != tt.session {
				t.Fatalf("browser session = %q, want %q", gotSession, tt.session)
			}
		})
	}
}

func TestOpenChatBrowserURLReportsFailure(t *testing.T) {
	wantErr := errors.New("path is outside this session's artifacts")
	var gotSession, gotURL string
	var toast ui.Toast
	err := openChatBrowserURL(t.Context(), "s1", "/artifacts/report.html", func(_ context.Context, session, url string) error {
		gotSession, gotURL = session, url
		return wantErr
	}, func(got ui.Toast) { toast = got })
	if !errors.Is(err, wantErr) || gotSession != "s1" || gotURL != "/artifacts/report.html" {
		t.Fatalf("open = %q %q, %v", gotSession, gotURL, err)
	}
	if toast.Variant != ui.ToastError || toast.Description == "" {
		t.Fatalf("failure toast = %+v, want an error with a description", toast)
	}
}
