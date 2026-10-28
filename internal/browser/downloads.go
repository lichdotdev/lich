package browser

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/profilestore"
)

// fallbackName is what a download is called when the page suggests no usable
// name.
const fallbackName = "download"

// stagingDir is the folder inside the session's profile Chrome writes into.
const stagingDir = "lich-downloads"

// partSuffix marks a download still being copied into Downloads.
const partSuffix = ".part"

// downloads names what a page saves. Chrome writes each file under its GUID
// (allowAndName) into staging; begin reserves the name the person will see in
// dir, the suggested one made unique the way browsers do ("report (1).csv"),
// and the completed file is moved to it, so nothing already there is
// overwritten. The download behaviour is the whole browser's, so what Chrome
// downloads on its own, with no page, lands in staging too; no begin names it,
// nothing moves it, and it goes with the profile.
type downloads struct {
	staging, dir string
	// rename is os.Rename, which the tests make cross filesystems.
	rename func(from, to string) error
	mu     sync.Mutex
	// reserved are the paths in-flight downloads will take, by GUID.
	reserved map[string]string
}

func newDownloads(staging, dir string) *downloads {
	return &downloads{staging: staging, dir: dir, rename: os.Rename, reserved: make(map[string]string)}
}

// begin reserves a path for the download guid announced.
func (d *downloads) begin(guid, suggested string) api.BrowserDownload {
	d.mu.Lock()
	defer d.mu.Unlock()
	path := d.freeLocked(safeName(suggested))
	d.reserved[guid] = path
	return api.BrowserDownload{Path: path, State: api.DownloadInProgress}
}

// progress applies one progress report. A completed download is not in place
// yet: its name stays reserved until place moves it there. ok is false for a
// GUID begin never saw.
func (d *downloads) progress(guid string, received float64, state string) (api.BrowserDownload, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	path, ok := d.reserved[guid]
	if !ok {
		return api.BrowserDownload{}, false
	}
	out := api.BrowserDownload{Path: path, Bytes: int64(received), State: state}
	if state == api.DownloadInProgress {
		return out, true
	}
	if state != api.DownloadCompleted {
		delete(d.reserved, guid)
		return out, true
	}
	if _, err := os.Lstat(path); err == nil {
		// Something else took the name while the file downloaded.
		out.Path = d.freeLocked(filepath.Base(path))
		d.reserved[guid] = out.Path
	}
	return out, true
}

// place moves the completed download dl from staging to its reserved path,
// outside the lock: across filesystems the move is a copy as long as the file.
// A move that fails reports the download failed, since the staged file goes
// with the profile.
func (d *downloads) place(guid string, dl api.BrowserDownload) api.BrowserDownload {
	err := d.move(filepath.Join(d.staging, guid), dl.Path)
	d.mu.Lock()
	delete(d.reserved, guid)
	d.mu.Unlock()
	if err != nil {
		slog.Warn("browser: move a finished download", "path", dl.Path, "err", err)
		dl.State = api.DownloadFailed
	}
	return dl
}

// move renames from to to, and copies where a rename cannot go: the profile
// and Downloads may sit on different filesystems, or volumes on Windows. The
// copy is written under a temporary name beside to and renamed once whole, so
// a half-copied file never carries the download's name.
func (d *downloads) move(from, to string) error {
	err := d.rename(from, to)
	var linkErr *os.LinkError
	if err == nil || !errors.As(err, &linkErr) || !profilestore.CrossDevice(linkErr.Err) {
		return err
	}
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	part := filepath.Join(filepath.Dir(to), "."+filepath.Base(from)+partSuffix)
	err = profilestore.CopyFile(from, part)
	if err == nil {
		err = os.Chmod(part, info.Mode().Perm())
	}
	if err == nil {
		err = os.Rename(part, to)
	}
	if err != nil {
		_ = os.Remove(part)
		return err
	}
	return os.Remove(from)
}

// freeLocked is name in dir, or its first " (n)" variant that neither exists
// nor is reserved.
func (d *downloads) freeLocked(name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		candidate := name
		if n > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		path := filepath.Join(d.dir, candidate)
		if d.takenLocked(path) {
			continue
		}
		// A name Lstat cannot read is left to the rename to fail on.
		if _, err := os.Lstat(path); err != nil {
			return path
		}
	}
}

func (d *downloads) takenLocked(path string) bool {
	for _, p := range d.reserved {
		if p == path {
			return true
		}
	}
	return false
}

// safeName keeps a page-suggested name inside the folder: no separators, no
// dot-only name.
func safeName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < ' ' {
			return '_'
		}
		return r
	}, name))
	if strings.Trim(name, ".") == "" {
		return fallbackName
	}
	return name
}
