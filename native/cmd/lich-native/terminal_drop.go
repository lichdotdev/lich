package main

import (
	"context"
	"fmt"
	"image"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"gioui.org/app"

	"github.com/omartelo/lich/native/lichclient"
)

// Files dropped on the terminal land at its prompt as paths, as useTerminalDrop
// does on the web. The window gets what the web never does, the files' own
// paths (text/uri-list), so there is no lookup by name, size and mtime: an
// unconfined session takes the paths as they are. A confined one reads only
// its checkout, so anything outside it is uploaded to /drop and pasted as the
// copy's path with the backend's notice under it. /drop takes bytes, not a
// path, so the confined agent holding LICH_TOKEN cannot use it to copy a file
// it names (internal/drop Attach says why that matters).

// uriListMIME is the type a file manager offers a drag of files as.
const uriListMIME = "text/uri-list"

// maxDropUpload matches internal/drop's maxUpload, past which the backend
// refuses the copy anyway.
const maxDropUpload = 32 << 20

// parseURIList reads an RFC 2483 text/uri-list into local paths: one URI per
// CRLF line, # lines are comments, and only file:// URIs name something a
// session can open. A file URI with a host other than localhost is another
// machine's file and is skipped with the rest.
func parseURIList(data string) []string {
	var paths []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.Path == "" {
			continue
		}
		paths = append(paths, filepath.FromSlash(windowsDrive(u.Path)))
	}
	return paths
}

// windowsDrive drops the slash file:///C:/x leaves before a drive letter.
func windowsDrive(p string) string {
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		return p[1:]
	}
	return p
}

// dropTarget is the session a drop lands on: whether it is confined, the
// checkout that bounds what a confined one reads, and the id its copies are
// kept under.
type dropTarget struct {
	sessionID string
	checkout  string
	confined  bool
}

// dropResult is a resolved drop: the paths to paste, in drop order, the
// backend's notices for the ones that are copies, and what yielded no path.
type dropResult struct {
	paths, notices, skipped []string
}

// resolveDrop turns dropped paths into what the session can open.
func resolveDrop(ctx context.Context, up *lichclient.Client, target dropTarget, dropped []string) dropResult {
	var r dropResult
	for _, path := range dropped {
		name := filepath.Base(path)
		info, err := os.Stat(path)
		switch {
		case err != nil:
			r.skipped = append(r.skipped, fmt.Sprintf("%s (%v)", name, err))
		case !target.confined || under(target.checkout, path):
			r.paths = append(r.paths, path)
		case info.IsDir():
			// The web's folderRefused, confined half: the other half, a folder
			// not found, cannot happen with a real path.
			r.skipped = append(r.skipped, name+" (folders outside this sandboxed session's checkout cannot be handed over; drop files)")
		case info.Size() > maxDropUpload:
			r.skipped = append(r.skipped, fmt.Sprintf("%s (over %dMB)", name, maxDropUpload>>20))
		default:
			a, err := uploadDropped(ctx, up, target.sessionID, path)
			if err != nil {
				r.skipped = append(r.skipped, fmt.Sprintf("%s (%v)", name, err))
				continue
			}
			r.paths, r.notices = append(r.paths, a.Path), append(r.notices, a.Notice)
		}
	}
	return r
}

func uploadDropped(ctx context.Context, up *lichclient.Client, sessionID, path string) (lichclient.Attachment, error) {
	f, err := os.Open(path)
	if err != nil {
		return lichclient.Attachment{}, err
	}
	defer f.Close()
	return up.UploadDrop(ctx, sessionID, filepath.Base(path), f)
}

// under reports whether path is inside root, as internal/drop under does:
// cleaned, not resolved through symlinks.
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

// terminalDrop follows a file drag across the window. rect is the terminal's
// last drawn bounds in window pixels, empty while no terminal is on screen,
// so a drag over the sidebar or the footer neither lights nor lands.
type terminalDrop struct {
	mu   sync.Mutex
	rect image.Rectangle
	// files is whether the drag in progress offers text/uri-list: text
	// dragged from another app carries none and the terminal leaves it
	// alone. Only DropEnter lists the types, so it is kept for the moves.
	files bool
	over  bool
}

// setRect records where the terminal was drawn this frame.
func (d *terminalDrop) setRect(r image.Rectangle) {
	d.mu.Lock()
	d.rect = r
	d.mu.Unlock()
}

// hinting reports whether a file drag is over the terminal.
func (d *terminalDrop) hinting() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.over
}

// handle follows one drag event and answers the paths of a drop that landed
// on the terminal, nil for anything else.
func (d *terminalDrop) handle(e app.DropEvent) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch e.Kind {
	case app.DropEnter:
		d.files = slices.Contains(e.Types, uriListMIME)
		d.over = d.files && d.inside(e)
	case app.DropMove:
		d.over = d.files && d.inside(e)
	case app.DropLeave:
		d.files, d.over = false, false
	case app.DropDrop:
		d.files, d.over = false, false
		if e.Type == uriListMIME && d.inside(e) {
			return parseURIList(string(e.Data))
		}
	}
	return nil
}

func (d *terminalDrop) inside(e app.DropEvent) bool {
	return e.Position.Round().In(d.rect)
}
