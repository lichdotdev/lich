package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// fallbackName is what a download is called when the page suggests no usable
// name.
const fallbackName = "download"

// downloads names what a page saves. Chrome writes each file under its GUID
// (allowAndName) into dir; begin reserves the name the person will see, the
// suggested one made unique the way browsers do ("report (1).csv"), and the
// completed file is renamed to it, so nothing already there is overwritten.
type downloads struct {
	dir string
	mu  sync.Mutex
	// reserved are the paths in-flight downloads will take, by GUID.
	reserved map[string]string
}

func newDownloads(dir string) *downloads {
	return &downloads{dir: dir, reserved: make(map[string]string)}
}

// begin reserves a path for the download guid announced.
func (d *downloads) begin(guid, suggested string) api.BrowserDownload {
	d.mu.Lock()
	defer d.mu.Unlock()
	path := d.freeLocked(safeName(suggested))
	d.reserved[guid] = path
	return api.BrowserDownload{Path: path, State: api.DownloadInProgress}
}

// progress applies one progress report, renaming the file into place once it
// is complete. ok is false for a GUID begin never saw.
func (d *downloads) progress(guid string, received float64, state string) (api.BrowserDownload, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	path, ok := d.reserved[guid]
	if !ok {
		return api.BrowserDownload{}, false, nil
	}
	out := api.BrowserDownload{Path: path, Bytes: int64(received), State: state}
	if state == api.DownloadInProgress {
		return out, true, nil
	}
	delete(d.reserved, guid)
	if state != api.DownloadCompleted {
		return out, true, nil
	}
	if _, err := os.Lstat(path); err == nil {
		// Something else took the name while the file downloaded.
		path = d.freeLocked(filepath.Base(path))
		out.Path = path
	}
	written := filepath.Join(d.dir, guid)
	if err := os.Rename(written, path); err != nil {
		out.Path = written
		return out, true, err
	}
	return out, true, nil
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
