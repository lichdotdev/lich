package browser

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lichdotdev/lich/api"
)

var errNoBrowser = fmt.Errorf("no browser for this session — open one first")

func (s *Service) live(id string) (*session, error) {
	s.mu.Lock()
	defer s.unlock()
	sess := s.lookupLocked(id)
	if sess == nil {
		return nil, errNoBrowser
	}
	return sess, nil
}

// touch marks sess used now, which withdraws the reaper's question about it.
func (s *Service) touch(sess *session) {
	s.mu.Lock()
	registered := s.byID[sess.id] == sess
	withdrawn := registered && sess.asked
	if registered {
		sess.lastUsed, sess.asked = s.now(), false
	}
	s.mu.Unlock()
	if withdrawn {
		s.emitState(sess, true)
	}
}

// Keep answers the reaper's question about owner's browser with keep: it
// stays, and is asked about again once it sits idle another idle window.
func (s *Service) Keep(owner string) error {
	sess, err := s.live(owner)
	if err != nil {
		return err
	}
	s.touch(sess)
	return nil
}

// drop takes out a browser a call failed on, if its Chrome is gone. One that
// is alive stays: a click on a stale index or on a page between documents
// fails and the next call works, and closing the browser lost the person's
// page and the agent's login with it.
func (s *Service) drop(sess *session) {
	if sess.alive() {
		return
	}
	s.mu.Lock()
	defer s.unlock()
	if s.byID[sess.id] == sess {
		s.buryLocked(sess)
	}
}

func (s *Service) remember(sess *session, url, title string) {
	s.mu.Lock()
	if s.byID[sess.id] == sess {
		if url != "" {
			sess.url = url
		}
		sess.title = title
	}
	s.mu.Unlock()
	s.touch(sess)
}

// Info returns the page the agent is looking at. Never includes form values.
func (s *Service) Info(id string) (PageInfo, error) {
	sess, err := s.live(id)
	if err != nil {
		return PageInfo{}, err
	}
	var info PageInfo
	err = runPage(func(ctx context.Context) error {
		var e error
		info, e = sess.page.Info(ctx)
		return e
	})
	if err != nil {
		return PageInfo{}, s.failed(sess, err)
	}
	s.remember(sess, info.URL, info.Title)
	s.agentActed(sess, api.BrowserAgentAct{})
	info.Downloads = s.untold(sess)
	info.ConsoleErrors, info.ConsoleErrorsMore = s.consoleUntold(sess)
	info.Dialogs = s.answeredUntold(sess)
	return info, nil
}

// Click sends a real mouse event at the target, not element.click().
func (s *Service) Click(id string, t Target) error {
	if t.empty() {
		return fmt.Errorf("click needs an index, a selector, or x and y")
	}
	sess, err := s.live(id)
	if err != nil {
		return err
	}
	var x, y float64
	err = runPage(func(ctx context.Context) error {
		var e error
		x, y, e = sess.page.Click(ctx, t)
		return e
	})
	if err != nil {
		return s.failed(sess, err)
	}
	s.touch(sess)
	s.agentActed(sess, api.BrowserAgentAct{Action: "click", X: &x, Y: &y})
	return nil
}

// Type sends keystrokes. clear selects-and-replaces; the typed text never
// comes back through Info.
func (s *Service) Type(id, text string, clear bool, t Target) error {
	return s.actAt(id, api.BrowserAgentAct{Action: "type"}, func(p Page, ctx context.Context) (*point, error) {
		at, err := clickTarget(ctx, p, t)
		if err != nil {
			return nil, err
		}
		return at, p.Type(ctx, text, clear, Target{})
	})
}

// Press sends one named key (Enter, Tab, Escape, …) as a real key event.
// An optional target is focused first, the same way Type focuses before typing.
func (s *Service) Press(id, key string, t Target) error {
	if _, err := keySequence(key); err != nil {
		return err
	}
	return s.actAt(id, api.BrowserAgentAct{Action: "press"}, func(p Page, ctx context.Context) (*point, error) {
		at, err := clickTarget(ctx, p, t)
		if err != nil {
			return nil, err
		}
		return at, p.Press(ctx, key, Target{})
	})
}

// clickTarget clicks t, as typing into or pressing on an element does first,
// and returns where; nil for an empty target, which acts on whatever has
// focus.
func clickTarget(ctx context.Context, p Page, t Target) (*point, error) {
	if t.empty() {
		return nil, nil
	}
	x, y, err := p.Click(ctx, t)
	if err != nil {
		return nil, err
	}
	return &point{X: x, Y: y}, nil
}

// Screenshot writes a PNG to dest (created if empty: inside the profile dir)
// and returns that path. The bytes stay off the transcript.
func (s *Service) Screenshot(id, dest string) (string, error) {
	sess, err := s.live(id)
	if err != nil {
		return "", err
	}
	if dest == "" {
		dest = filepath.Join(sess.dir, "screenshot.png")
	} else {
		abs, err := filepath.Abs(dest)
		if err != nil {
			return "", err
		}
		dest = abs
	}
	err = runPage(func(ctx context.Context) error {
		return sess.page.Screenshot(ctx, dest)
	})
	if err != nil {
		return "", s.failed(sess, err)
	}
	s.touch(sess)
	s.agentActed(sess, api.BrowserAgentAct{})
	return dest, nil
}

// Navigate loads url in this session's browser.
func (s *Service) Navigate(id, rawURL string) (Handle, error) {
	if err := s.checkURL(rawURL); err != nil {
		return Handle{}, err
	}
	sess, err := s.live(id)
	if err != nil {
		return Handle{}, err
	}
	u, err := s.resolveURL(sess.owner, rawURL)
	if err != nil {
		return Handle{}, err
	}
	if err := s.goTo(sess, u); err != nil {
		return Handle{}, err
	}
	return s.handleOf(sess), nil
}

// goTo is the agent loading u in sess's page. A download leaves the page
// where it was, so its URL is not remembered as the page's.
func (s *Service) goTo(sess *session, u string) error {
	var download bool
	if err := runPage(func(ctx context.Context) error {
		var err error
		download, err = s.navigate(ctx, sess, u)
		return err
	}); err != nil {
		return s.failed(sess, err)
	}
	if download {
		s.touch(sess)
	} else {
		s.remember(sess, u, "")
	}
	s.agentActed(sess, api.BrowserAgentAct{Action: "navigate", URL: u})
	return nil
}

// Reload reloads the current page.
func (s *Service) Reload(id string) error {
	return s.act(id, api.BrowserAgentAct{Action: "reload"}, func(p Page, ctx context.Context) error { return p.Reload(ctx) })
}

// Back goes back in history.
func (s *Service) Back(id string) error {
	return s.act(id, api.BrowserAgentAct{Action: "back"}, func(p Page, ctx context.Context) error { return p.Back(ctx) })
}

// Forward goes forward in history.
func (s *Service) Forward(id string) error {
	return s.act(id, api.BrowserAgentAct{Action: "forward"}, func(p Page, ctx context.Context) error { return p.Forward(ctx) })
}

// Scroll moves the viewport by dy CSS pixels (positive is down).
// It turns the wheel where the agent last acted, else in the middle.
func (s *Service) Scroll(id string, dy int) error {
	return s.actAt(id, api.BrowserAgentAct{Action: "scroll", DY: dy}, func(p Page, ctx context.Context) (*point, error) {
		x, y, err := p.Scroll(ctx, s.agentPoint(id), dy)
		if err != nil {
			return nil, err
		}
		return &point{X: x, Y: y}, nil
	})
}

// act runs one agent action on its page and tells the window the agent did
// act; drive runs it, dropping a browser whose page failed it. The window's
// own calls go through steer instead.
func (s *Service) act(id string, act api.BrowserAgentAct, fn func(Page, context.Context) error) error {
	sess, err := s.drive(id, fn)
	if err == nil {
		s.agentActed(sess, act)
	}
	return err
}

// actAt is act for act, made at the point fn returns (nil for none).
func (s *Service) actAt(id string, act api.BrowserAgentAct, fn func(Page, context.Context) (*point, error)) error {
	var at *point
	sess, err := s.drive(id, func(p Page, ctx context.Context) error {
		var err error
		at, err = fn(p, ctx)
		return err
	})
	if err != nil {
		return err
	}
	if at != nil {
		act.X, act.Y = &at.X, &at.Y
	}
	s.agentActed(sess, act)
	return nil
}

func (s *Service) drive(id string, fn func(Page, context.Context) error) (*session, error) {
	sess, err := s.live(id)
	if err != nil {
		return nil, err
	}
	err = runPage(func(ctx context.Context) error { return fn(sess.page, ctx) })
	if err != nil {
		return nil, s.failed(sess, err)
	}
	s.touch(sess)
	return sess, nil
}
