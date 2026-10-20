// Package drop turns a file dropped on the terminal into a path the session
// can paste.
//
// The window hands over the dropped files' own paths, and an unconfined
// session takes them as they are. A confined session's home is an empty
// private one (internal/sandbox), so a file outside its checkout is a path
// only lich can open: for those the window uploads the bytes (Upload), lich
// keeps a copy under the config dir, and the copy's path is pasted. Reading it
// works; editing it edits the copy, which is why the copy's path never lands at
// a prompt without the line that says so (copyNotice). The sandbox binds the
// copies directory and nothing else of the home.
//
// The copies are kept one directory per session, and that is the unit they are
// deleted in: a confined session sees its own directory mounted and not the one
// beside it, and every copy a session was dropped goes when its row does
// (Purge): a copy lives exactly as long as the session it was dropped into,
// and the clock only ever collects the orphans (Prune).
//
// The footer's attach button lands here too (Attach): a file chosen from the
// native picker already has its path, but a confined session cannot open one
// outside its checkout either, and the same copy is the answer.
package drop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/lichdotdev/lich/internal/prompt"

	"github.com/lichdotdev/lich/internal/sandbox"
)

// maxUpload bounds one dropped file. Screenshots and logs are the payload; a
// bigger drop is a mistake the terminal should refuse rather than copy.
const maxUpload = 32 << 20

// sandboxAvailable is sandbox.Available, replaced in tests: whether a machine
// can confine anything is the host's answer — bubblewrap installed, macOS —
// and a test about the copy must not take it.
var sandboxAvailable = sandbox.Available

// maxCopies bounds how many copies of one name live side by side (the name
// itself plus its -2… suffixes). Past it a caller is looping on something other
// than collisions, and the drop is refused rather than overwriting.
const maxCopies = 999

// keepDropped is how long an orphaned copy outlives its drop: one whose
// session row is already gone, which only a lich that died without deleting it
// leaves behind. A copy whose session still exists is not aged out at all
// (prune), so this is the bound on what a crash leaks, not on what a drop
// lasts.
const keepDropped = 3 * 24 * time.Hour

type Service struct {
	dir string
	// pick opens the host's file chooser. See SetPicker.
	pick func(title string) (string, error)
	// live reports whether a session's row is still there, open or parked. See
	// New.
	live func(sessionID string) bool
	// promptLang is the language of the notice under a copy's path. See
	// SetPromptLanguage.
	promptLang func() prompt.Lang
}

// SetPromptLanguage wires where the copy notice reads its language. It is
// read again for every notice, so a change in Settings reaches the next drop;
// without it the notice is English, the default. Startup wiring, called before
// anything serves.
func (s *Service) SetPromptLanguage(lang func() prompt.Lang) {
	s.promptLang = lang
}

func (s *Service) lang() prompt.Lang {
	if s.promptLang == nil {
		return prompt.English
	}
	return s.promptLang()
}

// SetPicker wires the host's file picker (internal/project), which is what the
// footer's attach button reaches through Attach. Startup wiring, called before
// anything serves.
func (s *Service) SetPicker(pick func(title string) (string, error)) {
	s.pick = pick
}

// Dir is where a lich with this config directory keeps the copies. It is the
// directory the sandbox binds into a confined session, so it is named here
// rather than spelled out at the two callers (main.go wires it through to
// internal/terminal).
func Dir(configDir string) string {
	return filepath.Join(configDir, "lich", "dropped")
}

// New keeps uploaded copies under Dir(configDir), and creates that directory
// now rather than on the first drop. A confined session binds its own copies
// directory when its PTY spawns (internal/terminal/sandbox.go), and a bind
// whose source is not there yet is skipped: the first file dropped into a
// session that opened before the directory existed would land where that
// session cannot read it.
//
// live answers whether a session's row still exists, open or parked
// (store.SessionExists): it is what keeps the age rule off the copies of a
// session that is still around. A nil one makes every copy an orphan, which is
// what the tests want and never what a running lich does.
func New(configDir string, live func(sessionID string) bool) *Service {
	dir := Dir(configDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("drop: could not create the copies dir", "dir", dir, "err", err)
	}
	return &Service{dir: dir, live: live}
}

// Upload stores one dropped file's bytes and answers with the path it landed
// at. The name and the session ride the query because the body is the file
// itself.
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	name := element(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	// No session, no copy: the id is what decides which directory the file
	// lands in, which is both what a confined session can read and what its
	// close deletes. A copy dropped outside either would live on with nothing
	// to end it.
	session := element(r.URL.Query().Get("session"))
	if session == "" {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}
	path, err := s.Save(session, name, http.MaxBytesReader(w, r.Body, maxUpload))
	if err != nil {
		slog.Warn("drop: upload failed", "name", name, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	body := map[string]string{"path": path, "notice": copyNotice(s.lang(), name)}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("drop: encode response", "path", path, "err", err)
	}
}

// copyNotice is the line that lands at the prompt under a copy's path, for a
// drag and for the attach button alike. Nothing about the path itself says it
// is a copy, so without it an agent told to edit the file edits the copy and
// reports back as if it had edited the original, and the user reads a path
// they cannot tell from their own. It is one line of plain text because both
// of them read the same prompt.
func copyNotice(lang prompt.Lang, name string) string {
	return fmt.Sprintf(prompt.For(lang).CopyNotice, name)
}

// attachTitle is the file chooser's own title, which is all the dialog says
// about why it opened.
const attachTitle = "Attach File"

// Attachment is what the picker produced: the path to write at the prompt, and
// the line that goes under it when that path is a copy's (copyNotice), empty
// when the session opens the file where it lies. Both empty for a cancelled
// dialog.
type Attachment struct {
	Path   string `json:"path"`
	Notice string `json:"notice"`
}

// Attach opens the file chooser and answers with a path the session can
// actually open: the file's own where the session reaches it, a copy's inside
// the session's dropped-files directory otherwise. It is the footer's attach
// button, and the counterpart of a drop — the same problem arrives through a
// dialog instead of a drag.
//
// The picker runs here rather than in the caller, and that is the security
// property of this method: every session carries LICH_TOKEN (see
// internal/terminal), so a confined agent can call any RPC on the loopback
// listener. A method that copied a *path it was handed* would let that agent
// name ~/.ssh/id_rsa and read the copy from inside the sandbox — the sandbox
// undone by its own harness. Here the path can only come from a dialog a human
// answers on screen.
//
// root is the session's checkout, the one tree a confined session sees; a file
// under it keeps its own path. Anything else is copied, including files under
// directories the sandbox happens to bind (a toolchain, ~/.config): copying one
// of those costs a few bytes, and deciding it does not need copying means
// reproducing the mount list here, where it would go stale in silence.
func (s *Service) Attach(sessionID, root string, confined bool) (Attachment, error) {
	if s.pick == nil {
		return Attachment{}, errors.New("no file picker wired")
	}
	path, err := s.pick(attachTitle)
	if err != nil {
		return Attachment{}, fmt.Errorf("open dialog failed: %w", err)
	}
	// A cancelled dialog is not a failure, and neither is a session that reaches
	// the file where it is.
	if path == "" || !confined || !sandboxAvailable() || under(root, path) {
		return Attachment{Path: path}, nil
	}
	copied, err := s.copyIn(sessionID, path)
	if err != nil {
		return Attachment{}, err
	}
	return Attachment{Path: copied, Notice: copyNotice(s.lang(), filepath.Base(path))}, nil
}

// under reports whether path is inside root. Both are cleaned but neither is
// resolved through symlinks: a link answering "outside" costs a copy, and a
// wrong "inside" costs the session the file altogether.
func under(root, path string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// copyIn puts one of the user's own files in a session's copies directory, so a
// confined session can read it. The ceiling is the upload's, for the same
// reason: past it the copy is a mistake rather than an attachment.
func (s *Service) copyIn(sessionID, path string) (string, error) {
	// Without a session there is no directory the sandbox binds, so a copy would
	// land where the session it was made for cannot read it.
	session, name := element(sessionID), element(filepath.Base(path))
	if session == "" || name == "" {
		return "", fmt.Errorf("cannot attach %q to session %q", path, sessionID)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder — a sandboxed session takes files, not trees", path)
	}
	if info.Size() > maxUpload {
		return "", fmt.Errorf(
			"%s is over the %dMB a sandboxed session can be handed", filepath.Base(path), maxUpload>>20,
		)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	return s.Save(session, name, file)
}

// element is one path element taken from untrusted input — a dropped file's
// name, a session id off the query — reduced to its last element so it can only
// ever name something inside the copies directory. Empty when nothing usable is
// left, which the callers refuse.
//
// The control characters go first because the name leaves again in copyNotice,
// which the window writes into the session's terminal inside a bracketed paste:
// an ESC of the name's own closes that paste early and every byte after it
// arrives as keystrokes the agent runs, and a bare newline forges a line of
// lich's own at the prompt. They are replaced rather than refused so a file
// still reaches the session it was dropped on — the name it lands under is
// worth less than the drop — and replaced rather than dropped so no pair of
// them can close up into the ".." this function exists to refuse.
func element(raw string) string {
	safe := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, raw)
	name := filepath.Base(filepath.FromSlash(safe))
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

// Save writes one dropped file under the session's own copies directory,
// without ever overwriting an earlier drop — its path may still be sitting in
// a prompt. sessionID must already be one path element (element).
func (s *Service) Save(sessionID, name string, body io.Reader) (string, error) {
	dir := filepath.Join(s.dir, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("dropped-files dir: %w", err)
	}
	path, err := uniquePath(dir, name)
	if err != nil {
		return "", err
	}
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	if _, err := io.Copy(file, body); err != nil {
		// A body that stops mid-write — an aborted request, a full disk —
		// otherwise leaves a truncated copy nobody asked for and nobody pasted,
		// under a name the next drop of the same file will then avoid. Windows
		// refuses to remove a file that still has an open handle, so the close
		// happens here rather than in the deferred one below it.
		file.Close()
		if err := os.Remove(path); err != nil {
			slog.Warn("drop: could not remove a half-written copy", "path", path, "err", err)
		}
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	s.Prune()
	return path, nil
}

// Purge deletes every copy dropped into one session. It is what makes a copy
// die with the session it was dropped into rather than with the clock: the
// paths pasted from it only ever meant anything inside that conversation, and
// the session's own row is gone by the time this runs (store.SetSessionGone).
//
// Best effort, like Prune: a failure leaves copies that the age rule still
// clears.
func (s *Service) Purge(sessionID string) {
	name := element(sessionID)
	if name == "" {
		return
	}
	dir := filepath.Join(s.dir, name)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("drop: purge failed", "dir", dir, "err", err)
	}
}

// Prune deletes the copies of sessions that are gone and older than
// keepDropped. Called after each new copy, the only thing that grows the
// directory, so a lich left running for weeks still clears what earlier drops
// left behind.
//
// Age is the backstop, not the rule: a session that ends takes its copies with
// it (Purge), and what this catches is the session lich never saw end — a
// crash, a kill, a machine that went down.
//
// A failure here is never the caller's problem — the copy it just wrote is
// good, and the stale ones get another chance on the next drop.
func (s *Service) Prune() {
	s.prune(false)
}

// PruneStale is Prune plus the session directories left with nothing in them.
// Startup only, and that is the whole reason it is a second method: a confined
// session binds its own copies directory when it spawns, and removing that
// directory under a live mount leaves every later copy of that session
// invisible inside it — the mount would still point at the deleted one. At
// startup no session has spawned yet, so there is no mount to strand.
func (s *Service) PruneStale() {
	s.prune(true)
}

func (s *Service) prune(empty bool) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		// Nothing dropped yet is the common case, not a fault.
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("drop: prune could not read the copies dir", "dir", s.dir, "err", err)
		}
		return
	}
	deadline := time.Now().Add(-keepDropped)
	for _, entry := range entries {
		path := filepath.Join(s.dir, entry.Name())
		if !entry.IsDir() {
			pruneExpired(path, entry, deadline)
			continue
		}
		// A copy is deleted by its session's row, not by the clock: the row
		// still being there (open or parked) means its paths can still be
		// pasted, however long ago the file was dropped. Only an orphan ages
		// out.
		if s.live != nil && s.live(entry.Name()) {
			continue
		}
		s.pruneSession(path, deadline, empty)
	}
}

// pruneSession clears the expired copies of one session, and the directory
// itself when the caller allows it and nothing is left in it.
func (s *Service) pruneSession(dir string, deadline time.Time, empty bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Warn("drop: prune could not read a session's copies", "dir", dir, "err", err)
		return
	}
	left := 0
	for _, entry := range entries {
		if entry.IsDir() || !pruneExpired(filepath.Join(dir, entry.Name()), entry, deadline) {
			left++
		}
	}
	if !empty || left > 0 {
		return
	}
	if err := os.Remove(dir); err != nil {
		slog.Warn("drop: prune failed", "path", dir, "err", err)
	}
}

// pruneExpired deletes one copy when it is past the deadline, and reports
// whether it is gone.
func pruneExpired(path string, entry fs.DirEntry, deadline time.Time) bool {
	info, err := entry.Info()
	if err != nil || info.ModTime().After(deadline) {
		return false
	}
	if err := os.Remove(path); err != nil {
		slog.Warn("drop: prune failed", "path", path, "err", err)
		return false
	}
	return true
}

// uniquePath is name, or name-2, name-3… up to maxCopies of them. Past that the
// namespace is exhausted and there is no path left to give: answering with the
// last one would overwrite a copy whose path may still be sitting in a prompt,
// which is the one thing Save promises never to do.
func uniquePath(dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; i <= maxCopies; i++ {
		path := filepath.Join(dir, name)
		if i > 1 {
			path = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		}
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s: %d copies already, no free name left", name, maxCopies)
}
