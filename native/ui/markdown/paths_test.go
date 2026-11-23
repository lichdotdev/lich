package markdown

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// pathRig lays src out with paths as its links, through a router that takes
// the pointer.
type pathRig struct {
	router input.Router
	cache  Cache
	theme  *ui.Theme
	paths  Paths
	opened []string
}

func newPathRig(link func(string) bool) *pathRig {
	r := &pathRig{cache: Cache{}, theme: markdownTheme()}
	r.paths = Paths{Link: link, Open: func(p string) { r.opened = append(r.opened, p) }}
	return r
}

func (r *pathRig) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(600, 200)}, Source: r.router.Source()}
	r.cache.LayoutLinks(gtx, r.theme, "k", "Read `app.go` now", Links{Paths: &r.paths})
	r.router.Frame(ops)
}

// sweep moves along the first line and presses at each point with mods,
// noting where the hand shows.
func (r *pathRig) sweep(mods key.Modifiers) (hand bool) {
	r.frame()
	for x := float32(1); x < 300; x += 3 {
		at := f32.Pt(x, 10)
		r.router.Queue(pointer.Event{Kind: pointer.Move, Position: at, Modifiers: mods})
		r.frame()
		hand = hand || r.router.Cursor() == pointer.CursorPointer
		r.router.Queue(
			pointer.Event{Kind: pointer.Press, Position: at, Buttons: pointer.ButtonPrimary, Modifiers: mods},
			pointer.Event{Kind: pointer.Release, Position: at, Modifiers: mods},
		)
		r.frame()
	}
	return hand
}

// A path in code that names something opens on a click with the shortcut
// modifier, and only then shows the hand, as a terminal's path does.
func TestAPathInCodeOpensOnAShortcutClick(t *testing.T) {
	r := newPathRig(func(p string) bool { return p == "app.go" })
	r.paths.Held = true
	if !r.sweep(key.ModShortcut) {
		t.Error("no hand over the path while the link key is held")
	}
	if len(r.opened) == 0 || r.opened[0] != "app.go" {
		t.Errorf("a shortcut click opened %q, want app.go", r.opened)
	}
}

func TestAPlainClickOnAPathOpensNothing(t *testing.T) {
	r := newPathRig(func(string) bool { return true })
	if r.sweep(0) {
		t.Error("the hand shows over a path with no link key held")
	}
	if len(r.opened) > 0 {
		t.Errorf("a plain click opened %q", r.opened)
	}
}

// A path that names nothing is no link: no hand, nothing opens.
func TestAPathThatNamesNothingIsNoLink(t *testing.T) {
	r := newPathRig(func(string) bool { return false })
	r.paths.Held = true
	if r.sweep(key.ModShortcut) {
		t.Error("the hand shows over a path that names nothing")
	}
	if len(r.opened) > 0 {
		t.Errorf("a path that names nothing opened %q", r.opened)
	}
}

// With a listing to look in, only code that names something there wears a
// path's mark; a branch, a repository and a dotted name give the room back.
func TestOnlyCodeTheListingFindsWearsAPathMark(t *testing.T) {
	listed := map[string]bool{"src/main.go": true, "docs/": true}
	paths := &Paths{Link: func(p string) bool { return listed[p] }, Version: 1}
	for src, want := range map[string]string{
		"See `src/main.go` now":         "file",
		"Look in `docs/`":               "folder",
		"Push `origin/main`":            "",
		"Fork `lichdotdev/gio`":         "",
		"Read `api.PullRequest` again":  "",
		"Open `github.com` in a tab":    "",
		"Key `chat.slash.new` is wrong": "",
	} {
		cache := Cache{}
		c := cache.text(markdownTheme(), "k", src, paths).blocks[0].prose
		got := ""
		for i, d := range c.decor {
			switch d.icon {
			case icFile:
				got = "file"
			case icFolder:
				got = "folder"
			}
			if d.icon == nil && strings.HasPrefix(c.spans[i].Content, codeIconRoom) {
				t.Errorf("%q: span %d keeps the mark's room with no mark", src, i)
			}
		}
		if got != want {
			t.Errorf("%q: mark %q, want %q", src, got, want)
		}
	}
	// A listing that changes checks the marks again.
	cache := Cache{}
	src := "Push `origin/main`"
	cache.text(markdownTheme(), "k", src, paths)
	listed["origin/main"] = true
	paths.Version = 2
	if !hasMark(cache.text(markdownTheme(), "k", src, paths).blocks[0].prose) {
		t.Error("a path the new listing finds did not get its mark")
	}
}

func hasMark(c *markdownCell) bool {
	for _, d := range c.decor {
		if d.icon == icFile || d.icon == icFolder {
			return true
		}
	}
	return false
}
