package main

// The paths an agent writes in code in its answers, which open as a path in
// its terminal does: Ctrl+click (Cmd on macOS) on a file brings it to the
// stage editor, on a folder unfolds it in the dock's Code tab.

import (
	"context"
	"path"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// chatFiles is a checkout's listing as an answer's paths are looked up in it.
type chatFiles struct {
	files []string
	// listed is each file, and dirs each folder over one.
	listed, dirs map[string]bool
	// tails is what a path not listed as written matched as the tail of a
	// file, by the path as written: the match walks the whole listing.
	tails map[string]bool
}

func readChatFiles(ctx context.Context, client *backend.Client, checkout string) (*chatFiles, error) {
	l, err := client.Tree(ctx, checkout)
	if err != nil {
		return nil, err
	}
	return newChatFiles(l.Files), nil
}

func newChatFiles(files []string) *chatFiles {
	f := &chatFiles{files: files, listed: map[string]bool{}, dirs: map[string]bool{}, tails: map[string]bool{}}
	for _, file := range files {
		f.listed[file] = true
		for d := path.Dir(file); d != "." && !f.dirs[d]; d = path.Dir(d) {
			f.dirs[d] = true
		}
	}
	return f
}

// links says whether written names a file or a folder of checkout, the way a
// terminal's path:line is found: as a path from the checkout, or for a file
// as the tail of listed ones.
func (f *chatFiles) links(checkout, written string) bool {
	rel, ok := codeRefRel(checkout, written)
	if !ok {
		return false
	}
	if f.listed[rel] || f.dirs[rel] {
		return true
	}
	if strings.HasSuffix(written, "/") {
		return false
	}
	if hit, seen := f.tails[written]; seen {
		return hit
	}
	hit := len(codeRefMatches(f.files, checkout, gioterm.CodeRef{Path: written})) > 0
	f.tails[written] = hit
	return hit
}

// chatPaths is what makes the paths of sv's answers links this frame. The
// listing is read again as the conversation grows, since a turn writes the
// files its answer names.
func (u *window) chatPaths(p *chatPane, sv sessionView, items int) *markdown.Paths {
	if p.files.invalidate == nil {
		p.files.invalidate = u.model.invalidate
	}
	checkout := sv.path
	key := ""
	if checkout != "" {
		key = checkout + " " + strconv.Itoa(items)
	}
	p.files.want(key, checkout, func(ctx context.Context) (*chatFiles, error) { return readChatFiles(ctx, u.client, checkout) })
	files, _, _ := p.files.get()
	return &markdown.Paths{
		Held: u.terms.linkKeyHeld,
		Link: func(written string) bool { return files != nil && files.links(checkout, written) },
		Open: func(written string) {
			if rel, ok := codeRefRel(checkout, written); ok && files.dirs[rel] {
				u.unfoldFolder(sv, rel)
				return
			}
			u.followCodeRef(sv.ID, gioterm.CodeRef{Path: written})
		},
	}
}

// unfoldFolder brings sv up if it is not the active session, then the dock's
// Code tab on its checkout with rel and the folders over it unfolded.
func (u *window) unfoldFolder(sv sessionView, rel string) {
	if !u.model.sessionOnScreen(sv.ID) {
		u.focusSession(sv.projectID, sv.ID)
	}
	u.dock.code.unfold(sv.path, rel)
	u.dock.show(dockCode)
}
