package browser

import (
	"context"
	"fmt"
	"slices"

	"github.com/lichdotdev/lich/api"
)

// tabsSink records the page's tabs and tells the window once the browser is
// registered.
func (s *Service) tabsSink(sess *session) func([]api.BrowserTab) {
	return func(tabs []api.BrowserTab) {
		s.mu.Lock()
		sess.tabs, sess.paneTabs = agentTabs(tabs), tabs
		registered := s.byID[sess.id] == sess
		s.mu.Unlock()
		if registered {
			s.emitState(sess, true)
		}
	}
}

// Tabs is this session's browser's tabs, in the pane's order.
func (s *Service) Tabs(id string) ([]api.BrowserTab, error) {
	sess, err := s.live(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]api.BrowserTab{}, sess.tabs...), nil
}

// TabAct is the agent opening, selecting or closing a tab, which the pane
// plays as its cursor on the header's tabs. A new tab with a URL then loads
// it, as Navigate does. A DevTools tab is not one the agent can name, and
// selecting the tab it drives leaves the pane on that tab's DevTools.
func (s *Service) TabAct(opts api.BrowserTabOptions) error {
	fn, err := tabStep(opts)
	if err != nil {
		return err
	}
	sess, err := s.live(opts.ID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pane, agent := sess.paneTabs, sess.tabs
	s.mu.Unlock()
	if slices.ContainsFunc(pane, func(t api.BrowserTab) bool { return t.DevTools && t.ID == opts.Tab }) {
		return errNoTab
	}
	if opts.Action == api.TabSelect && slices.ContainsFunc(agent, func(t api.BrowserTab) bool { return t.Front && t.ID == opts.Tab }) {
		fn = func(Page, context.Context) error { return nil }
	}
	act := api.BrowserAgentAct{Action: "tab-" + opts.Action, Tab: opts.Tab}
	if err := s.act(opts.ID, act, fn); err != nil || opts.Action != api.TabNew || opts.URL == "" {
		return err
	}
	_, err = s.Navigate(opts.ID, opts.URL)
	return err
}

// DevTools is the person opening DevTools for the front tab in the pane, or
// selecting it.
func (s *Service) DevTools(owner string) error {
	if s.inWindow(owner) {
		return errInWindow
	}
	_, err := s.steer(owner, pageWait, func(p Page, ctx context.Context) error { return p.DevTools(ctx) })
	return err
}

// TabNav is the person opening, selecting or closing a tab in the pane.
func (s *Service) TabNav(opts api.BrowserTabOptions) error {
	fn, err := tabStep(opts)
	if err != nil {
		return err
	}
	_, err = s.steer(opts.ID, pageWait, fn)
	return err
}

func tabStep(opts api.BrowserTabOptions) (func(Page, context.Context) error, error) {
	switch opts.Action {
	case api.TabNew:
		return func(p Page, ctx context.Context) error { return p.NewTab(ctx) }, nil
	case api.TabSelect:
		return func(p Page, ctx context.Context) error { return p.SelectTab(ctx, opts.Tab) }, nil
	case api.TabClose:
		return func(p Page, ctx context.Context) error { return p.CloseTab(ctx, opts.Tab) }, nil
	}
	return nil, fmt.Errorf("unknown tab action %q", opts.Action)
}
