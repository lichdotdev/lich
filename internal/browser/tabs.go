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
	// devtools is the pane's DevTools for this tab, nil for none.
	devtools *devtools
}

// devtools is Chrome's own DevTools front-end inspecting one tab, a target
// lich creates in a headless window of its own: as a tab behind the page it
// drew no frames (Chromium 157, measured 2026-11-12). It is never one of the
// tabs the agent lists or drives. paused: the tab it inspects is stopped in
// its debugger.
type devtools struct {
	ctx    context.Context
	cancel context.CancelFunc
	id     target.ID
	paused bool
}

// tabStack is the page's tabs in the order the pane shows them. A tab one
// opens goes right after its opener, and the opener stays as it was, still
// logged in: loading the new tab's URL in the opener instead lost a login a
// script held there.
type tabStack struct {
	mu    sync.Mutex
	open  []*tab
	front *tab
	// inspecting: the pane shows the front tab's DevTools in its place. The
	// agent still drives the front tab.
	inspecting bool
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

// setFront puts t in front, shown in the pane.
func (s *tabStack) setFront(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.front, s.inspecting = t, false
}

// inspect puts t in front with its DevTools shown in its place, unless t has
// none or left the stack meanwhile.
func (s *tabStack) inspect(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.devtools != nil && slices.Contains(s.open, t) {
		s.front, s.inspecting = t, true
	}
}

// shown is what the pane draws: the front tab's DevTools while inspecting,
// else nil for the front tab itself.
func (s *tabStack) shown() *devtools {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inspecting {
		return s.front.devtools
	}
	return nil
}

// setDevTools gives t its DevTools, false when t has one already or left the
// stack, in which case d is the caller's to close.
func (s *tabStack) setDevTools(t *tab, d *devtools) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.devtools != nil || !slices.Contains(s.open, t) {
		return false
	}
	t.devtools = d
	return true
}

// dropDevTools takes t's DevTools away, and says whether the pane showed it;
// nil when it had none.
func (s *tabStack) dropDevTools(t *tab) (d *devtools, shown bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, t.devtools = t.devtools, nil
	shown = d != nil && s.inspecting && s.front == t
	if shown {
		s.inspecting = false
	}
	return d, shown
}

// inspected is the tab whose DevTools target is id, nil for none.
func (s *tabStack) inspected(id target.ID) *tab {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.open {
		if t.devtools != nil && t.devtools.id == id {
			return t
		}
	}
	return nil
}

// setPaused records whether t is stopped in its debugger, and says whether
// that changed.
func (s *tabStack) setPaused(t *tab, paused bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := t.devtools
	if d == nil || d.paused == paused {
		return false
	}
	d.paused = paused
	return true
}

func (s *tabStack) anyPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.open, func(t *tab) bool { return t.devtools != nil && t.devtools.paused })
}

// inspectedBy is t's DevTools, nil for none.
func (s *tabStack) inspectedBy(t *tab) *devtools {
	s.mu.Lock()
	defer s.mu.Unlock()
	return t.devtools
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
	s.inspecting = false
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

// list is the tabs as the pane shows them, each DevTools tab right after the
// tab it inspects and Front while the pane shows it. It carries no DevTools
// URL: that one drives the whole browser.
func (s *tabStack) list() []api.BrowserTab {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]api.BrowserTab, 0, len(s.open))
	for _, t := range s.open {
		front := t == s.front
		out = append(out, api.BrowserTab{ID: string(t.id), URL: t.url, Title: t.title, Front: front && !s.inspecting})
		if d := t.devtools; d != nil {
			out = append(out, api.BrowserTab{ID: string(d.id), DevTools: true, Paused: d.paused, Front: front && s.inspecting})
		}
	}
	return out
}

// agentTabs is the pane's tabs as the agent sees them: no DevTools tab, and
// in front the tab the agent drives, which a DevTools tab in front inspects.
func agentTabs(pane []api.BrowserTab) []api.BrowserTab {
	out := make([]api.BrowserTab, 0, len(pane))
	for _, t := range pane {
		if !t.DevTools {
			out = append(out, t)
			continue
		}
		if t.Front && len(out) > 0 {
			out[len(out)-1].Front = true
		}
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
