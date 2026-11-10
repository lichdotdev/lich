package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

func put(t *testing.T, path, body string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

type sent struct {
	events []api.SessionArtifacts
	shown  []string
}

func (s *sent) emit(name string, data any) {
	if name == api.ArtifactsEvent.Name {
		s.events = append(s.events, data.(api.SessionArtifacts))
	}
}

func newWired(t *testing.T, open map[string]string) (*Service, *sent, string) {
	t.Helper()
	root := t.TempDir()
	s := New(root, nil)
	out := &sent{}
	key := func(id string) string {
		if k, ok := open[id]; ok {
			return k
		}
		return id
	}
	s.SetSessions(Sessions{
		Open:  func() (map[string]string, error) { return open, nil },
		Key:   key,
		Label: func(id string) string { return "label of " + id },
		Lang:  func() prompt.Lang { return prompt.English },
		Show: func(session, from, path string) error {
			out.shown = append(out.shown, session+" "+from+" "+path)
			return nil
		},
	}, out.emit)
	return s, out, root
}

func names(files []api.Artifact) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}

func TestListDirKeepsPagesNewestFirst(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "old.html"), "old", time.Hour)
	put(t, filepath.Join(dir, "new.svg"), "new", 0)
	put(t, filepath.Join(dir, "site", "index.html"), "site", 30*time.Minute)
	put(t, filepath.Join(dir, "assets", "logo.png"), "png", 0)
	put(t, filepath.Join(dir, ".draft.html"), "hidden", 0)
	if err := os.Symlink(filepath.Join(dir, "old.html"), filepath.Join(dir, "link.html")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}

	files := listDir(dir)

	if got := strings.Join(names(files), " "); got != "new.svg site old.html" {
		t.Fatalf("listed %q, want the pages newest first, without the asset folder, the hidden file and the link", got)
	}
	if files[1].Path != filepath.Join(dir, "site", "index.html") {
		t.Errorf("a folder opens at %q, want its index page", files[1].Path)
	}
	if got := listDir(filepath.Join(dir, "missing")); got == nil || len(got) != 0 {
		t.Errorf("a missing folder lists %v, want an empty list", got)
	}
}

func TestWatchSendsAChangedListAndEmptiesAClosedSession(t *testing.T) {
	open := map[string]string{"s1": "s1", "s2": "s2"}
	s, out, root := newWired(t, open)
	seen := map[string][]api.Artifact{}

	s.watchOnce(seen)
	if len(out.events) != 0 {
		t.Fatalf("a first pass over empty folders sent %v", out.events)
	}
	put(t, filepath.Join(root, "s1", "report.html"), "r", 0)
	s.watchOnce(seen)
	s.watchOnce(seen)
	if len(out.events) != 1 || out.events[0].SessionID != "s1" || len(out.events[0].Files) != 1 {
		t.Fatalf("after a new page: %+v, want s1's list once", out.events)
	}
	put(t, filepath.Join(root, "s1", "report.html"), "rewritten", -time.Minute)
	s.watchOnce(seen)
	if len(out.events) != 2 {
		t.Fatalf("a page rewritten in place sent %d events, want a second list", len(out.events))
	}
	delete(open, "s1")
	s.watchOnce(seen)
	if last := out.events[len(out.events)-1]; last.SessionID != "s1" || last.Files == nil || len(last.Files) != 0 {
		t.Fatalf("a closed session sent %+v, want an empty list", last)
	}
}

func TestListCoversEveryOpenSession(t *testing.T) {
	s, _, root := newWired(t, map[string]string{"fresh": "root", "s2": "s2"})
	put(t, filepath.Join(root, "root", "a.html"), "a", 0)

	got, err := s.List(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SessionID != "fresh" || len(got[0].Files) != 1 || len(got[1].Files) != 0 {
		t.Fatalf("List = %+v, want fresh's page under its files key and an empty s2", got)
	}
}

func TestAttachHandsTheSameSessionItsOwnPath(t *testing.T) {
	s, _, root := newWired(t, map[string]string{"s1": "s1"})
	put(t, filepath.Join(root, "s1", "report.html"), "r", 0)

	got, err := s.Attach(t.Context(), api.AttachArtifactOptions{SessionID: "s1", From: "s1", Name: "report.html"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != filepath.Join(root, "s1", "report.html") || got.Notice != "" {
		t.Fatalf("Attach = %+v, want the page's own path and no notice", got)
	}
}

func TestAttachCopiesAnotherSessionsPageBesideEarlierCopies(t *testing.T) {
	s, _, root := newWired(t, map[string]string{"a": "a", "b": "b"})
	put(t, filepath.Join(root, "a", "report.html"), "from a", 0)

	first, err := s.Attach(t.Context(), api.AttachArtifactOptions{SessionID: "b", From: "a", Name: "report.html"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Attach(t.Context(), api.AttachArtifactOptions{SessionID: "b", From: "a", Name: "report.html"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != filepath.Join(root, "b", "report.html") || second.Path != filepath.Join(root, "b", "report-2.html") {
		t.Fatalf("copies at %q and %q", first.Path, second.Path)
	}
	if body, err := os.ReadFile(second.Path); err != nil || string(body) != "from a" {
		t.Fatalf("copy holds %q (%v)", body, err)
	}
	if want := `[lich] copy of report.html from session "label of a"; edits stay in the copy.`; first.Notice != want {
		t.Errorf("notice = %q, want %q", first.Notice, want)
	}
}

func TestAttachCopiesAFolderPageWithItsAssets(t *testing.T) {
	s, _, root := newWired(t, map[string]string{"a": "a", "b": "b"})
	put(t, filepath.Join(root, "a", "site", "index.html"), "index", 0)
	put(t, filepath.Join(root, "a", "site", "app.js"), "js", 0)

	got, err := s.Attach(t.Context(), api.AttachArtifactOptions{SessionID: "b", From: "a", Name: "site"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != filepath.Join(root, "b", "site", "index.html") {
		t.Fatalf("Attach = %q, want the copy's index page", got.Path)
	}
	if body, err := os.ReadFile(filepath.Join(root, "b", "site", "app.js")); err != nil || string(body) != "js" {
		t.Fatalf("the folder's asset was not copied: %q (%v)", body, err)
	}
}

func TestAttachRefusesWhatIsNotListed(t *testing.T) {
	s, _, root := newWired(t, map[string]string{"a": "a", "b": "b"})
	secret := filepath.Join(t.TempDir(), "secret")
	put(t, secret, "secret", 0)
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "a", "link.html")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	for _, name := range []string{"link.html", "missing.html", "../b", ""} {
		if _, err := s.Attach(t.Context(), api.AttachArtifactOptions{SessionID: "b", From: "a", Name: name}); err == nil {
			t.Errorf("Attach(%q) copied it", name)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "b")); len(entries) != 0 {
		t.Errorf("a refused attach left %d entries in the target folder", len(entries))
	}
}

func TestOpenShowsAnotherSessionsPageFromItsOwnFolder(t *testing.T) {
	s, out, root := newWired(t, map[string]string{"a": "a", "b": "b"})
	put(t, filepath.Join(root, "a", "site", "index.html"), "index", 0)

	if _, err := s.Open(t.Context(), api.AttachArtifactOptions{SessionID: "b", From: "a", Name: "site"}); err != nil {
		t.Fatal(err)
	}
	if want := "b a " + filepath.Join(root, "a", "site", "index.html"); len(out.shown) != 1 || out.shown[0] != want {
		t.Fatalf("shown %v, want %q", out.shown, want)
	}
	if _, err := s.Open(t.Context(), api.AttachArtifactOptions{SessionID: "b", From: "a", Name: "missing.html"}); err == nil {
		t.Error("Open of an artifact not listed went through")
	}
}
