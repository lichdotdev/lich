package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/internal/relpath"
)

// maxPreviewBytes caps one side of an image preview. It covers a 4K
// screenshot saved as PNG (5 to 15 MB) with room to spare, and keeps a stray
// asset from being held whole in memory twice, here and in the window.
const maxPreviewBytes = 20 << 20

// previewTypes is both the allowlist and the Content-Type table: the formats
// the window decodes (imageExtensions in native/cmd/lich-native/blob_preview.go).
// SVG is left out on purpose: git diffs it as text.
var previewTypes = map[string]string{
	".bmp":  "image/bmp",
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// The two refusals a reader meets are worded for the window, which prints the
// response body as it is.
var (
	errPreviewUnsupported = errors.New("this file type can't be previewed")
	errPreviewTooLarge    = fmt.Errorf("above the %d MB preview limit", maxPreviewBytes>>20)
	errBlobRequest        = errors.New("invalid preview request")
)

// Blob is one file's bytes at one revision, with the type it is served as.
type Blob struct {
	Data        []byte
	ContentType string
}

// githubBlobTimeout bounds one image read from the contents API: up to
// maxPreviewBytes over the network, well past what prReadTimeout allows a
// JSON answer.
const githubBlobTimeout = 30 * time.Second

// Blob reads rel at ref for an image preview. ref is "" for the working tree,
// "HEAD" for the commit a checkout's uncommitted diff stands on, or an object
// id. An object this clone does not hold, a pull request's head or merge base,
// is read from GitHub, the way FileLines reads it.
//
// A path absent at ref is fs.ErrNotExist: the side of an added or deleted file
// that has nothing to show.
func (s *Service) Blob(dir, rel, ref string) (Blob, error) {
	if err := relpath.Validate(rel); err != nil {
		return Blob{}, fmt.Errorf("%w: %w", errBlobRequest, err)
	}
	if ref != "" && ref != "HEAD" && !isOID(ref) {
		return Blob{}, fmt.Errorf("%w: revision %q", errBlobRequest, ref)
	}
	contentType, ok := previewTypes[strings.ToLower(path.Ext(rel))]
	if !ok {
		return Blob{}, errPreviewUnsupported
	}
	var data []byte
	var err error
	switch {
	case ref == "":
		data, err = workTreeBlob(dir, rel)
	case ref == "HEAD" || holdsObject(dir, ref):
		data, err = objectBlob(dir, rel, ref)
	default:
		data, err = s.githubBlob(dir, rel, ref)
	}
	if err != nil {
		return Blob{}, err
	}
	return Blob{Data: data, ContentType: contentType}, nil
}

// workTreeBlob reads through relpath.Resolve for the reason ReadFile does: a
// symlink out of the checkout is not the checkout's file.
func workTreeBlob(dir, rel string) ([]byte, error) {
	full, err := relpath.Resolve(dir, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", errBlobRequest, rel)
	}
	if err := checkPreviewSize(info.Size()); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	return data, nil
}

// objectBlob asks git for the size before the bytes, so an oversize object is
// refused without being read.
func objectBlob(dir, rel, ref string) ([]byte, error) {
	object := ref + ":" + rel
	// gitQuiet: a path absent at the revision is the expected answer for an
	// added or deleted file, not a failure to report.
	kind, ok := gitQuiet(dir, "cat-file", "-t", object)
	if !ok {
		return nil, fmt.Errorf("%s at %s: %w", rel, ref, fs.ErrNotExist)
	}
	if strings.TrimSpace(kind) != "blob" {
		return nil, fmt.Errorf("%w: %s is not a regular file", errBlobRequest, rel)
	}
	sizeText, err := runGit(dir, "cat-file", "-s", object)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(sizeText), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("size of %s at %s: %w", rel, ref, err)
	}
	if err := checkPreviewSize(size); err != nil {
		return nil, err
	}
	out, err := runGit(dir, "cat-file", "blob", object)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// holdsObject reports whether this clone has the object ref names.
func holdsObject(dir, ref string) bool {
	_, ok := gitQuiet(dir, "cat-file", "-e", ref)
	return ok
}

// githubBlob reads rel at a commit this clone does not hold. The size is only
// known once the bytes are here: the raw contents answer carries no length
// worth trusting ahead of the body.
func (s *Service) githubBlob(dir, rel, ref string) ([]byte, error) {
	out, err := s.gh(githubBlobTimeout, dir, "api", "-H", ghRawAccept, contentsPath(rel, ref))
	if err != nil {
		return nil, err
	}
	if err := checkPreviewSize(int64(len(out))); err != nil {
		return nil, err
	}
	return out, nil
}

// PullRequestMergeBase is the commit pull request number's diff is taken
// from, the old side of every file GitHub shows. The base branch's tip is not
// it once the base moved past where the branch left it. The tips come from
// GitHub; their merge base is computed here when the clone holds both, and
// asked of GitHub otherwise.
func (s *Service) PullRequestMergeBase(path string, number int) (string, error) {
	out, err := s.gh(prReadTimeout, path, "api", fmt.Sprintf("repos/{owner}/{repo}/pulls/%d", number),
		"--jq", `.base.sha + " " + .head.sha`)
	if err != nil {
		return "", err
	}
	base, head, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	if !isOID(base) || !isOID(head) {
		return "", errUnreadableAnswer(fmt.Errorf("pull request tips %q", out))
	}
	if local, ok := gitQuiet(path, "merge-base", base, head); ok {
		return strings.TrimSpace(local), nil
	}
	out, err = s.gh(prReadTimeout, path, "api", "repos/{owner}/{repo}/compare/"+base+"..."+head,
		"--jq", ".merge_base_commit.sha")
	if err != nil {
		return "", err
	}
	mergeBase := strings.TrimSpace(string(out))
	if !isOID(mergeBase) {
		return "", errUnreadableAnswer(fmt.Errorf("merge base %q", out))
	}
	return mergeBase, nil
}

func checkPreviewSize(size int64) error {
	if size > maxPreviewBytes {
		return fmt.Errorf("%.1f MB, %w", float64(size)/(1<<20), errPreviewTooLarge)
	}
	return nil
}

// ServeBlob is Blob over HTTP, so the bytes travel raw rather than as base64
// inside an RPC answer. The query carries path, rel and ref.
//
// The window tells the sides apart by status: 404 is a side with no file, 413
// a file over the limit, 415 a type with no preview. The body says which, in
// the words the window shows.
func (s *Service) ServeBlob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	blob, err := s.Blob(query.Get("path"), query.Get("rel"), query.Get("ref"))
	if err != nil {
		http.Error(w, err.Error(), blobStatus(err))
		return
	}
	w.Header().Set("Content-Type", blob.ContentType)
	// The working tree answers differently under the same URL as the agent
	// writes; the window keys each side by its blob oid instead of by caching.
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(blob.Data))
}

func blobStatus(err error) int {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, errPreviewTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, errPreviewUnsupported):
		return http.StatusUnsupportedMediaType
	case errors.Is(err, errBlobRequest):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
