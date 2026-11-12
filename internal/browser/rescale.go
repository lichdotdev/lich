package browser

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
)

// rescaleWait is how long the window's own scale holds still before a pane's
// browser relaunches at it. A window dragged across several monitors changes
// scale once per crossing: the wait relaunches it once, where it stops.
const rescaleWait = time.Second

// scheduleRescaleLocked (re)starts the countdown to rescaleWatched. The caller holds
// mu.
func (s *Service) scheduleRescaleLocked() {
	if s.stopped {
		return
	}
	if s.rescaleTimer != nil {
		s.rescaleTimer.Stop()
	}
	s.rescaleTimer = time.AfterFunc(s.rescaleWait, s.rescaleWatched)
}

// rescaleWatched relaunches each browser a pane shows that was launched
// under another window scale than the window's: the screencast copies the
// browser's own surface, whose scale only a launch sets. One a dialog or the
// debugger holds is tried again later.
func (s *Service) rescaleWatched() {
	s.mu.Lock()
	var owners []string
	for _, sess := range s.byID {
		if sess.watching {
			owners = append(owners, sess.owner)
		}
	}
	s.mu.Unlock()
	retry := false
	for _, owner := range owners {
		held, err := s.rescale(owner)
		if err != nil {
			slog.Warn("browser: relaunch at the window's scale", "owner", owner, "err", err)
		}
		retry = retry || held
	}
	if retry {
		s.mu.Lock()
		s.scheduleRescaleLocked()
		s.mu.Unlock()
	}
}

// rescale relaunches owner's headless browser when it is stale, once no
// agent call runs on it. A dialog open on the page or a tab paused in the
// debugger holds it off, which it reports. Looking first, before the wait,
// keeps a pane that opens from waiting on the agent for nothing.
func (s *Service) rescale(owner string) (held bool, err error) {
	if sess, held := s.stale(owner); sess == nil || held {
		return held, nil
	}
	turn := s.turn(owner)
	turn.Lock()
	defer turn.Unlock()
	lock := s.ownerLock(owner)
	lock.Lock()
	defer lock.Unlock()
	sess, held := s.stale(owner)
	if sess == nil || held {
		return held, nil
	}
	return false, s.relaunch(sess)
}

// stale is owner's headless browser when it was launched under another
// window scale than the window's and a launch now would draw at another
// scale, and whether a dialog or the debugger holds it. Only the window's own
// scale counts: a relaunch on a page zoom would cost the page its scroll and
// a half-filled form for a slight blur. A move the clamp hides (1.5 to 2)
// keeps the browser under the new window scale, or a zoom after it would
// relaunch.
func (s *Service) stale(owner string) (*session, bool) {
	s.mu.Lock()
	defer s.unlock()
	sess := s.liveByOwner(owner)
	if sess == nil || sess.headed || sess.window == s.window {
		return nil, false
	}
	if sess.scale == launchScale(s.defaultView.Scale) {
		sess.window = s.window
		return nil, false
	}
	paused := slices.ContainsFunc(sess.paneTabs, func(t api.BrowserTab) bool { return t.Paused })
	return sess, sess.dialog != nil || paused
}

// relaunch starts sess's browser again on its profile, at the scale a launch
// draws at now, under the same id, with its tabs, their DevTools, cookies and
// element counter. What lives only in the page (a half-filled form,
// sessionStorage, where it was scrolled) does not survive.
func (s *Service) relaunch(sess *session) error {
	next := sess.page.IndexNext()
	s.mu.Lock()
	tabs := append([]api.BrowserTab{}, sess.paneTabs...)
	var view *Viewport
	if sess.watching {
		v := sess.view
		view = &v
	}
	s.mu.Unlock()
	spec, err := s.handOver(sess)
	if err != nil {
		return err
	}
	spec.id, spec.idxNext, spec.view, spec.tabs = sess.id, next, view, tabs
	if i := slices.IndexFunc(tabs, func(t api.BrowserTab) bool { return !t.DevTools }); i >= 0 {
		spec.url = tabs[i].URL
	}
	if _, err := s.start(spec); err != nil {
		s.emitState(sess, false)
		return err
	}
	return nil
}

// reopenTabs opens the page tabs of pane past the first, which the launch
// loaded, then DevTools for each tab that had it, and brings back to the
// front what was there. The new tabs are new targets, so they are found by
// their place. A tab that fails to load stays open where it failed, and
// DevTools that fails to open stays closed: the rest is still the person's.
func (s *Service) reopenTabs(ctx context.Context, sess *session, pane []api.BrowserTab) error {
	var pages []api.BrowserTab
	inspected := map[int]bool{}
	front, frontDevTools := 0, false
	for _, t := range pane {
		if !t.DevTools {
			pages = append(pages, t)
		} else if len(pages) > 0 {
			inspected[len(pages)-1] = true
		}
		if t.Front {
			front, frontDevTools = max(len(pages)-1, 0), t.DevTools
		}
	}
	for _, t := range pages[min(1, len(pages)):] {
		if err := sess.page.NewTab(ctx); err != nil {
			return err
		}
		if _, err := s.navigate(ctx, sess, t.URL); err != nil {
			slog.Warn("browser: reopen a tab", "url", t.URL, "err", err)
		}
	}
	if len(pages) < 2 && len(inspected) == 0 {
		return nil
	}
	s.mu.Lock()
	now := sess.tabs
	s.mu.Unlock()
	if len(now) != len(pages) {
		return nil
	}
	for i := range pages {
		if inspected[i] {
			if err := s.inspectAgain(ctx, sess, now[i].ID); err != nil {
				slog.Warn("browser: reopen DevTools", "err", err)
				frontDevTools = frontDevTools && i != front
			}
		}
	}
	if err := sess.page.SelectTab(ctx, now[front].ID); err != nil {
		return err
	}
	if frontDevTools {
		return sess.page.DevTools(ctx)
	}
	return nil
}

// inspectAgain opens DevTools for the tab id, which brings that tab to the
// front and its DevTools in front of it.
func (s *Service) inspectAgain(ctx context.Context, sess *session, id string) error {
	if err := sess.page.SelectTab(ctx, id); err != nil {
		return err
	}
	return sess.page.DevTools(ctx)
}

// agentTurn holds a relaunch off id's browser until the release it returns,
// so an agent's call, the read of the page it left included, runs on one
// browser from start to end.
func (s *Service) agentTurn(id string) func() {
	s.mu.Lock()
	owner := id
	if sess := s.byID[id]; sess != nil {
		owner = sess.owner
	}
	s.mu.Unlock()
	turn := s.turn(owner)
	turn.RLock()
	return turn.RUnlock
}

func (s *Service) turn(owner string) *sync.RWMutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	turn := s.turns[owner]
	if turn == nil {
		turn = &sync.RWMutex{}
		s.turns[owner] = turn
	}
	return turn
}
