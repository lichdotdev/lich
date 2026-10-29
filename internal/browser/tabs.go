package browser

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/chromedp/cdproto/target"
	"github.com/lichdotdev/lich/api"
)

var (
	errNoTab   = errors.New("no such tab in this session's browser")
	errLastTab = errors.New("the browser's last tab cannot be closed; close the browser instead")
)

// tab is one of the page's tabs. Only the front one is drawn and driven: a
// headless Chrome draws no frames for a tab behind another and takes no input
// there.
type tab struct {
	ctx    context.Context
	cancel context.CancelFunc
	id     target.ID
	// opener is the tab that opened this one, which comes back to the front
	// when this one closes; nil for one the person or the agent opened.
	opener     *tab
	url, title string
}

// tabStack is the page's tabs in the order the pane shows them. A tab one
// opens goes right after its opener, and the opener stays as it was, still
// logged in: loading the new tab's URL in the opener instead lost a login a
// script held there.
type tabStack struct {
	mu    sync.Mutex
	open  []*tab
	front *tab
	// view is the size the page was last laid out at, which a tab coming to
	// the front is laid out at too.
	view *Viewport
	// dialog is the tab holding a JavaScript dialog open, nil for none. A tab
	// behind the front one can: with its opener, a new tab shares the
	// renderer of the one under it, and a dialog there holds the front tab's
	// script too (Chromium 155, measured 2026-10-29).
	dialog *tab
}

// start is the stack holding the browser's first tab, in front.
func (s *tabStack) start(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open, s.front = []*tab{t}, t
}

func (s *tabStack) frontTab() *tab {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.front
}

func (s *tabStack) setFront(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.front = t
}

// add puts t right after the tabs its opener opened before it, or last.
func (s *tabStack) add(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := len(s.open)
	if i := slices.Index(s.open, t.opener); i >= 0 {
		at = i + 1
		for at < len(s.open) && s.open[at].opener == t.opener {
			at++
		}
	}
	s.open = slices.Insert(s.open, at, t)
}

func (s *tabStack) byID(id target.ID) *tab {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.open {
		if t.id == id {
			return t
		}
	}
	return nil
}

func (s *tabStack) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.open)
}

// remove takes the tab id out, and says whether it was the front one, in
// which case its opener comes to the front, else the tab beside it. The last
// tab is never removed: a browser has one.
func (s *tabStack) remove(id target.ID) (t *tab, front bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.open, func(t *tab) bool { return t.id == id })
	if i < 0 || len(s.open) == 1 {
		return nil, false
	}
	t = s.open[i]
	s.open = slices.Delete(s.open, i, i+1)
	for _, o := range s.open {
		if o.opener == t {
			o.opener = t.opener
		}
	}
	if s.front != t {
		return t, false
	}
	s.front = s.open[min(i, len(s.open)-1)]
	if t.opener != nil {
		s.front = t.opener
	}
	return t, true
}

// setInfo records a tab's URL and title, and says whether either changed.
func (s *tabStack) setInfo(t *tab, url, title string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.url == url && t.title == title {
		return false
	}
	t.url, t.title = url, title
	return true
}

func (s *tabStack) list() []api.BrowserTab {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]api.BrowserTab, len(s.open))
	for i, t := range s.open {
		out[i] = api.BrowserTab{ID: string(t.id), URL: t.url, Title: t.title, Front: t == s.front}
	}
	return out
}

func (s *tabStack) setView(v Viewport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view = &v
}

func (s *tabStack) lastView() *Viewport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

func (s *tabStack) setDialog(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialog = t
}

// dialogClosed says whether t was the tab holding the dialog, which it no
// longer does.
func (s *tabStack) dialogClosed(t *tab) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dialog != t {
		return false
	}
	s.dialog = nil
	return true
}

// dialogHolder is the tab holding the dialog, else the front one.
func (s *tabStack) dialogHolder() *tab {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dialog != nil {
		return s.dialog
	}
	return s.front
}
