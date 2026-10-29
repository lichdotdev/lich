package browser

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// tabsSink records the page's tabs and tells the window once the browser is
// registered.
func (s *Service) tabsSink(sess *session) func([]api.BrowserTab) {
	return func(tabs []api.BrowserTab) {
		s.mu.Lock()
		sess.tabs = tabs
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
// it, as Navigate does.
func (s *Service) TabAct(opts api.BrowserTabOptions) error {
	fn, err := tabStep(opts)
	if err != nil {
		return err
	}
	act := api.BrowserAgentAct{Action: "tab-" + opts.Action, Tab: opts.Tab}
	if err := s.act(opts.ID, act, fn); err != nil || opts.Action != api.TabNew || opts.URL == "" {
		return err
	}
	_, err = s.Navigate(opts.ID, opts.URL)
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
