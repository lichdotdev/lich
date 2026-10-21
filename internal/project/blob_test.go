package project

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// pngBytes is a PNG signature followed by a NUL, which is what makes git and
// lich call a file binary.
const pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

func TestBlobWorkTree(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	write(t, repo, "img/logo.PNG", pngBytes)

	blob, err := New(nil).Blob(repo, "img/logo.PNG", "")
	if err != nil {
		t.Fatalf("Blob: %v", err)
	}
	if string(blob.Data) != pngBytes {
		t.Errorf("data = %q, want %q", blob.Data, pngBytes)
	}
	if blob.ContentType != "image/png" {
		t.Errorf("content type = %q, want image/png", blob.ContentType)
	}
}

// The review's sides: HEAD for a working-tree diff, or a tree oid. Each reads
// its own revision while the working tree has moved on.
func TestBlobAtRevision(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	write(t, repo, "logo.png", "png-old\x00")
	git("add", "logo.png")
	git("commit", "-m", "logo")
	tree := strings.TrimSpace(git("rev-parse", "HEAD^{tree}"))
	write(t, repo, "logo.png", "png-new\x00")

	svc := New(nil)
	for _, ref := range []string{"HEAD", tree} {
		blob, err := svc.Blob(repo, "logo.png", ref)
		if err != nil {
			t.Fatalf("Blob at %s: %v", ref, err)
		}
		if string(blob.Data) != "png-old\x00" || blob.ContentType != "image/png" {
			t.Errorf("at %s: %q as %q, want the committed image", ref, blob.Data, blob.ContentType)
		}
	}
}

// An added file has no old side and a deleted one no new side; both must read
// as absent, which the window reads as a side with nothing to show.
func TestBlobMissingSideIsNotExist(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	svc := New(nil)
	for _, ref := range []string{"", "HEAD"} {
		if _, err := svc.Blob(repo, "new.png", ref); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("absent at %q: err = %v, want fs.ErrNotExist", ref, err)
		}
	}
}

// A pull request's commits need not be in the clone; GitHub answers for them.
func TestBlobMissingObjectAsksGitHub(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	missing := strings.Repeat("b", 40)

	gh := &fakeGH{out: []byte(pngBytes)}
	blob, err := withGH(gh).Blob(repo, "img/logo.png", missing)
	if err != nil {
		t.Fatalf("Blob: %v", err)
	}
	if string(blob.Data) != pngBytes || blob.ContentType != "image/png" {
		t.Errorf("got %q as %q, want GitHub's image", blob.Data, blob.ContentType)
	}
	want := []string{"api", "-H", ghRawAccept, contentsPath("img/logo.png", missing)}
	if !slices.Equal(gh.args, want) {
		t.Errorf("gh args = %v, want %v", gh.args, want)
	}
}

// A commit the clone holds is read locally, even when the path is absent at
// it: that answer is the clone's to give, not GitHub's.
func TestBlobLocalObjectNeverAsksGitHub(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	write(t, repo, "logo.png", pngBytes)
	git("add", ".")
	git("commit", "-m", "logo")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))

	gh := &fakeGH{err: errors.New("gh must not run")}
	svc := withGH(gh)
	if blob, err := svc.Blob(repo, "logo.png", head); err != nil || string(blob.Data) != pngBytes {
		t.Errorf("present: %q, %v; want the committed image", blob.Data, err)
	}
	if _, err := svc.Blob(repo, "gone.png", head); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("absent: err = %v, want fs.ErrNotExist", err)
	}
	if gh.calls != 0 {
		t.Errorf("gh ran %d times, want 0", gh.calls)
	}
}

func TestBlobGitHubSizeLimit(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	gh := &fakeGH{out: make([]byte, maxPreviewBytes+1)}
	if _, err := withGH(gh).Blob(repo, "big.png", strings.Repeat("c", 40)); !errors.Is(err, errPreviewTooLarge) {
		t.Errorf("err = %v, want errPreviewTooLarge", err)
	}
}

func TestBlobGitHubFailureSurfaces(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	gh := &fakeGH{err: errors.New("gh: Not Found (HTTP 404)")}
	if _, err := withGH(gh).Blob(repo, "x.png", strings.Repeat("c", 40)); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want gh's failure, not a missing side", err)
	}
}

// Both tips in the clone: git computes the merge base, GitHub is asked only
// for the tips.
func TestPullRequestMergeBaseLocal(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	write(t, repo, "fork.txt", "fork\n")
	git("add", ".")
	git("commit", "-m", "fork point")
	fork := strings.TrimSpace(git("rev-parse", "HEAD"))
	git("checkout", "-q", "-b", "topic")
	write(t, repo, "b.txt", "topic\n")
	git("add", ".")
	git("commit", "-m", "topic")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))
	git("checkout", "-q", "-")
	write(t, repo, "c.txt", "base moved\n")
	git("add", ".")
	git("commit", "-m", "base")
	base := strings.TrimSpace(git("rev-parse", "HEAD"))

	gh := &fakeGH{out: []byte(base + " " + head + "\n")}
	got, err := withGH(gh).PullRequestMergeBase(repo, 7)
	if err != nil {
		t.Fatalf("PullRequestMergeBase: %v", err)
	}
	if got != fork {
		t.Errorf("merge base = %s, want the fork point %s", got, fork)
	}
	if want := []string{"api", "repos/{owner}/{repo}/pulls/7", "--jq", `.base.sha + " " + .head.sha`}; gh.calls != 1 || !slices.Equal(gh.args, want) {
		t.Errorf("gh calls = %v, want one: %v", gh.history, want)
	}
}

func TestPullRequestMergeBaseAsksGitHub(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	base, head, mergeBase := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)

	gh := &fakeGH{outs: [][]byte{[]byte(base + " " + head + "\n"), []byte(mergeBase + "\n")}}
	got, err := withGH(gh).PullRequestMergeBase(repo, 7)
	if err != nil {
		t.Fatalf("PullRequestMergeBase: %v", err)
	}
	if got != mergeBase {
		t.Errorf("merge base = %s, want %s", got, mergeBase)
	}
	want := []string{"api", "repos/{owner}/{repo}/compare/" + base + "..." + head, "--jq", ".merge_base_commit.sha"}
	if !slices.Equal(gh.args, want) {
		t.Errorf("gh args = %v, want %v", gh.args, want)
	}
}

func TestPullRequestMergeBaseRejectsAnUnreadableAnswer(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	tips := []byte(strings.Repeat("a", 40) + " " + strings.Repeat("b", 40))
	for name, outs := range map[string][][]byte{
		"tips":       {[]byte("null null")},
		"merge base": {tips, []byte("null")},
	} {
		if _, err := withGH(&fakeGH{outs: outs}).PullRequestMergeBase(repo, 7); err == nil {
			t.Errorf("%s: want an error, got nil", name)
		}
	}
}

func TestBlobRefusals(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	write(t, repo, "assets/x.png", pngBytes)
	write(t, repo, "notes.txt", "text\n")
	git("add", ".")
	git("commit", "-m", "assets")

	tests := []struct {
		name, rel, ref string
		want           error
	}{
		{"traversal", "../x.png", "", errBlobRequest},
		{"branch name is not a revision", "assets/x.png", "main", errBlobRequest},
		{"option-shaped revision", "assets/x.png", "--output=x", errBlobRequest},
		{"text file", "notes.txt", "", errPreviewUnsupported},
		{"svg stays a text diff", "icon.svg", "", errPreviewUnsupported},
		{"a format the window does not decode", "doc.pdf", "", errPreviewUnsupported},
	}
	svc := New(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Blob(repo, tt.rel, tt.ref); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBlobDirectoryNamedLikeAnImage(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	write(t, repo, "shots.png/a.txt", "x\n")
	git("add", ".")
	git("commit", "-m", "dir")

	svc := New(nil)
	for _, ref := range []string{"", "HEAD"} {
		if _, err := svc.Blob(repo, "shots.png", ref); !errors.Is(err, errBlobRequest) {
			t.Errorf("directory at %q: err = %v, want errBlobRequest", ref, err)
		}
	}
}

// The limit is pinned to a literal and probed on both sides, in the working
// tree and in git's object store, which are read by different code.
func TestBlobSizeLimit(t *testing.T) {
	t.Parallel()
	if maxPreviewBytes != 20<<20 {
		t.Fatalf("maxPreviewBytes = %d, want 20 MiB", maxPreviewBytes)
	}
	repo, git := initRepo(t)
	write(t, repo, "at.png", strings.Repeat("\x00", maxPreviewBytes))
	write(t, repo, "over.png", strings.Repeat("\x00", maxPreviewBytes+1))
	git("add", ".")
	git("commit", "-m", "big")

	svc := New(nil)
	for _, ref := range []string{"", "HEAD"} {
		if _, err := svc.Blob(repo, "at.png", ref); err != nil {
			t.Errorf("at the limit (%q): %v", ref, err)
		}
		if _, err := svc.Blob(repo, "over.png", ref); !errors.Is(err, errPreviewTooLarge) {
			t.Errorf("over the limit (%q): err = %v, want errPreviewTooLarge", ref, err)
		}
	}
}

func serveBlob(t *testing.T, method string, query url.Values, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/blob?"+query.Encode(), nil)
	for key, values := range header {
		req.Header[key] = values
	}
	rec := httptest.NewRecorder()
	New(nil).ServeBlob(rec, req)
	return rec
}

func TestServeBlob(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	write(t, repo, "logo.png", pngBytes)

	rec := serveBlob(t, http.MethodGet, url.Values{"path": {repo}, "rel": {"logo.png"}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", rec.Code, rec.Body)
	}
	if !bytes.Equal(rec.Body.Bytes(), []byte(pngBytes)) {
		t.Errorf("body = %q, want the file", rec.Body.Bytes())
	}
	for key, want := range map[string]string{
		"Content-Type":  "image/png",
		"Cache-Control": "no-store",
	} {
		if got := rec.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestServeBlobStatuses(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	write(t, repo, "big.png", strings.Repeat("\x00", maxPreviewBytes+1))

	tests := []struct {
		name   string
		method string
		rel    string
		ref    string
		want   int
	}{
		{"missing side", http.MethodGet, "gone.png", "", http.StatusNotFound},
		{"over the limit", http.MethodGet, "big.png", "", http.StatusRequestEntityTooLarge},
		{"no preview for the type", http.MethodGet, "a.txt", "", http.StatusUnsupportedMediaType},
		{"bad revision", http.MethodGet, "big.png", "main", http.StatusBadRequest},
		{"write method", http.MethodPost, "big.png", "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := url.Values{"path": {repo}, "rel": {tt.rel}, "ref": {tt.ref}}
			if rec := serveBlob(t, tt.method, query, nil); rec.Code != tt.want {
				t.Errorf("status = %d (%s), want %d", rec.Code, rec.Body, tt.want)
			}
		})
	}
}

// The window prints these bodies verbatim, so their words are the contract.
func TestServeBlobRefusalWording(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	write(t, repo, "big.png", strings.Repeat("\x00", maxPreviewBytes+1))

	tests := []struct{ rel, want string }{
		{"big.png", "20.0 MB, above the 20 MB preview limit\n"},
		{"a.txt", "this file type can't be previewed\n"},
	}
	for _, tt := range tests {
		rec := serveBlob(t, http.MethodGet, url.Values{"path": {repo}, "rel": {tt.rel}}, nil)
		if rec.Body.String() != tt.want {
			t.Errorf("%s: body = %q, want %q", tt.rel, rec.Body, tt.want)
		}
	}
}
