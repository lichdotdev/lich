package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// inputWait bounds one of the pane's input events. Chrome answers one in
// ~15ms, but an event dispatched while a navigation swaps the document can go
// unanswered (as Puppeteer has seen); the pane's queue waits on it, so it
// must give up long before pageWait.
const inputWait = 2 * time.Second

// errInWindow: the pane draws a headless browser, and this one moved to its
// own window, where the person already sees it.
var errInWindow = errors.New("this browser is open in its own window")

// Attach shows this session's browser in a pane sized v, starting a headless
// one at about:blank when there is none. The page the agent left stays as it
// is: the pane only starts watching it.
func (s *Service) Attach(owner string, v Viewport) (Handle, error) {
	if owner == "" {
		return Handle{}, errNoOwner
	}
	lock := s.ownerLock(owner)
	lock.Lock()
	defer lock.Unlock()
	if s.current(owner) == nil {
		if _, err := s.launchFresh(owner, "about:blank", false, false); err != nil {
			return Handle{}, err
		}
	}
	sess := s.current(owner)
	if sess == nil {
		return Handle{}, errNoBrowser
	}
	if sess.headed {
		return Handle{}, errInWindow
	}
	if err := runPage(func(ctx context.Context) error { return sess.page.Screencast(ctx, v) }); err != nil {
		return Handle{}, s.failed(sess, err)
	}
	s.mu.Lock()
	sess.watching = true
	s.mu.Unlock()
	return s.handleOf(sess), nil
}

// Detach stops the frames of a pane that closed or went off screen. The
// browser keeps running for the agent, at the window's size again rather than
// the pane's.
func (s *Service) Detach(owner string) error {
	view := s.unwatchedView()
	sess, err := s.steer(owner, inputWait, func(p Page, ctx context.Context) error {
		if err := p.StopScreencast(ctx); err != nil || view == (Viewport{}) {
			return err
		}
		return p.Resize(ctx, view)
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	sess.watching = false
	s.mu.Unlock()
	return nil
}

// Resize lays the page out again at the pane's new size. A pane showing it
// gets its frames at that size too: the screencast keeps the size it started
// at, and the window would stretch the smaller frames over the larger pane.
func (s *Service) Resize(owner string, v Viewport) error {
	sess, err := s.live(owner)
	if err != nil {
		return err
	}
	s.mu.Lock()
	watching := sess.watching
	s.mu.Unlock()
	_, err = s.steer(owner, inputWait, func(p Page, ctx context.Context) error {
		if watching {
			return p.Screencast(ctx, v)
		}
		return p.Resize(ctx, v)
	})
	return err
}

// SetDefaultView sets the size a headless page no pane shows lays out at, and
// lays out the running ones there. It is not a use: a window resized does not
// keep an idle browser from the reaper.
func (s *Service) SetDefaultView(v Viewport) {
	s.mu.Lock()
	s.defaultView = v
	var pages []Page
	for _, sess := range s.byID {
		if sess.alive() && !sess.headed && !sess.watching {
			pages = append(pages, sess.page)
		}
	}
	s.mu.Unlock()
	for _, p := range pages {
		ctx, cancel := context.WithTimeout(context.Background(), inputWait)
		if err := p.Resize(ctx, v); err != nil && !errors.Is(err, errDialogOpen) {
			slog.Warn("browser: lay a page out at the window's size", "err", err)
		}
		cancel()
	}
}

func (s *Service) unwatchedView() Viewport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultView
}

// Nav runs the pane's toolbar: back, forward, reload, or go to url.
func (s *Service) Nav(owner, action, rawURL string) error {
	var fn func(Page, context.Context) error
	switch action {
	case "back":
		fn = func(p Page, ctx context.Context) error { return p.Back(ctx) }
	case "forward":
		fn = func(p Page, ctx context.Context) error { return p.Forward(ctx) }
	case "reload":
		fn = func(p Page, ctx context.Context) error { return p.Reload(ctx) }
	case "go":
		u, err := allowURL(rawURL)
		if err != nil {
			return err
		}
		sess, err := s.live(owner)
		if err != nil {
			return err
		}
		fn = func(_ Page, ctx context.Context) error {
			_, err := s.navigate(ctx, sess, u)
			return err
		}
	default:
		return fmt.Errorf("unknown browser action %q", action)
	}
	_, err := s.steer(owner, pageWait, fn)
	return err
}

// steer runs one of the window's own calls on owner's page, bounded by wait.
// Unlike an agent action, a call that fails leaves the browser running
// unless its process is gone: the person is still looking at the page. A
// call a dialog held off does nothing and is no error: the pane shows the
// dialog, and answering it is what the person can do.
func (s *Service) steer(owner string, wait time.Duration, fn func(Page, context.Context) error) (*session, error) {
	sess, err := s.onPage(owner, wait, fn)
	if errors.Is(err, errDialogOpen) {
		return sess, nil
	}
	return sess, err
}

// onPage runs fn on owner's page, bounded by wait, and keeps the browser
// when it fails, unless its process is gone.
func (s *Service) onPage(owner string, wait time.Duration, fn func(Page, context.Context) error) (*session, error) {
	sess, err := s.live(owner)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if err := fn(sess.page, ctx); err != nil {
		return sess, s.failed(sess, err)
	}
	s.touch(sess)
	return sess, nil
}

// Running is every browser this process runs, for a window catching up.
func (s *Service) Running() []api.BrowserState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []api.BrowserState{}
	for _, sess := range s.byID {
		if sess.alive() {
			out = append(out, sess.stateLocked(true))
		}
	}
	return out
}

func (s *Service) frameSink(sess *session) func(string) {
	frames := api.BrowserFrameEvents.Of(sess.owner)
	return func(data string) { events.Send(s.emit, frames, data) }
}

// infoSink records the page's URL and title as it navigates on its own (a
// link the person or the agent clicked), and tells the window once the
// browser is registered.
func (s *Service) infoSink(sess *session) func(url, title string) {
	return func(url, title string) {
		s.mu.Lock()
		if url != "" {
			sess.url = url
		}
		sess.title = title
		registered := s.byID[sess.id] == sess
		s.mu.Unlock()
		if registered {
			s.emitState(sess, true)
		}
	}
}

func (s *Service) emitState(sess *session, open bool) {
	s.mu.Lock()
	state := sess.stateLocked(open)
	s.mu.Unlock()
	events.Send(s.emit, api.BrowserStateEvent, state)
}

// agentActed tells the window the agent acted on sess's page, and keeps the
// point it acted at for its next scroll.
func (s *Service) agentActed(sess *session, act api.BrowserAgentAct) {
	act.Owner = sess.owner
	if act.X != nil && act.Y != nil {
		s.mu.Lock()
		sess.agentAt = &point{X: *act.X, Y: *act.Y}
		s.mu.Unlock()
	}
	events.Send(s.emit, api.BrowserAgentEvent, act)
}

// agentPoint is where the agent last acted on owner's page, nil before it
// acted at a point.
func (s *Service) agentPoint(owner string) *point {
	s.mu.Lock()
	defer s.unlock()
	sess := s.lookupLocked(owner)
	if sess == nil || sess.agentAt == nil {
		return nil
	}
	at := *sess.agentAt
	return &at
}

func (s *session) stateLocked(open bool) api.BrowserState {
	return api.BrowserState{
		Owner: s.owner, Open: open, Headed: s.headed, ByAgent: s.byAgent, URL: s.url, Title: s.title,
		Dialog: s.dialog,
	}
}
