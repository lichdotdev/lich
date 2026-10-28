package browser

import (
	"context"
	"errors"

	"github.com/lichdotdev/lich/api"
)

// The methods below are the RPC surface (api.Browser*). The ones they call
// keep the shape the package tests. Neither Cleanup nor CloseOwnedBy is
// routed: one ends every browser, the other is the session-close hook.

// Show brings this session's browser on screen.
func (s *Service) Show(_ context.Context, opts api.SessionIDOptions) (api.BrowserHandle, error) {
	h, err := s.OpenVisible(opts.ID)
	return handleAPI(h), err
}

// OpenURL navigates this session's browser, creating a headless one when the
// card has not opened a window.
func (s *Service) OpenURL(_ context.Context, opts api.BrowserOpenOptions) (api.BrowserHandle, error) {
	h, err := s.Open(opts.ID, opts.URL)
	if err != nil {
		return api.BrowserHandle{}, err
	}
	return s.withDownloads(handleAPI(h)), nil
}

// Read returns the page the agent is looking at, once the text it waits for
// came or went. A wait that gives up still answers with the page.
func (s *Service) Read(_ context.Context, opts api.BrowserInfoOptions) (api.BrowserPageInfo, error) {
	timedOut, err := s.waitText(opts)
	if err != nil {
		return api.BrowserPageInfo{}, err
	}
	info, err := s.Info(opts.ID)
	if errors.Is(err, errDialogOpen) {
		return s.heldPage(opts.ID)
	}
	out := infoAPI(info)
	out.WaitTimedOut = timedOut
	return out, err
}

// ClickAt clicks in this session's browser.
func (s *Service) ClickAt(_ context.Context, opts api.BrowserClickOptions) (api.BrowserPageInfo, error) {
	if err := s.Click(opts.ID, targetOf(opts.BrowserTarget)); err != nil {
		return s.heldOr(opts.ID, err)
	}
	return s.pageAfter(opts.ID)
}

// TypeText types into this session's browser.
func (s *Service) TypeText(_ context.Context, opts api.BrowserTypeOptions) (api.None, error) {
	return api.None{}, s.Type(opts.ID, opts.Text, opts.Clear, targetOf(opts.BrowserTarget))
}

// PressKey presses one named key in this session's browser.
func (s *Service) PressKey(_ context.Context, opts api.BrowserPressOptions) (api.BrowserPageInfo, error) {
	if err := s.Press(opts.ID, opts.Key, targetOf(opts.BrowserTarget)); err != nil {
		return s.heldOr(opts.ID, err)
	}
	return s.pageAfter(opts.ID)
}

// pageAfter is the page an action left, read once a navigation it started has
// its document. A read that fails keeps the browser: the action itself went
// through, and the page may be between two documents.
func (s *Service) pageAfter(id string) (api.BrowserPageInfo, error) {
	var info PageInfo
	sess, err := s.onPage(id, pageWait, func(p Page, ctx context.Context) error {
		if err := p.Settle(ctx); err != nil {
			return err
		}
		var err error
		info, err = p.Info(ctx)
		return err
	})
	if err != nil {
		return s.heldOr(id, err)
	}
	s.remember(sess, info.URL, info.Title)
	info.Downloads = s.downloadsAfter(sess, downloadWait)
	info.ConsoleErrors, info.ConsoleErrorsMore = s.consoleUntold(sess)
	info.Dialogs = s.answeredUntold(sess)
	return infoAPI(info), nil
}

// heldOr is the page an action that opened a dialog left, which it answers
// with rather than fail, or err.
func (s *Service) heldOr(id string, err error) (api.BrowserPageInfo, error) {
	if errors.Is(err, errDialogOpen) {
		return s.heldPage(id)
	}
	return api.BrowserPageInfo{}, err
}

// Capture writes a PNG of this session's page and returns its path.
func (s *Service) Capture(_ context.Context, opts api.BrowserScreenshotOptions) (string, error) {
	return s.Screenshot(opts.ID, opts.Path)
}

// Go loads a URL in this session's already-open browser.
func (s *Service) Go(_ context.Context, opts api.BrowserOpenOptions) (api.BrowserHandle, error) {
	h, err := s.Navigate(opts.ID, opts.URL)
	if err != nil {
		return api.BrowserHandle{}, err
	}
	return s.withDownloads(handleAPI(h)), nil
}

// withDownloads adds to h the file its navigation downloaded, as a click's
// answer does.
func (s *Service) withDownloads(h api.BrowserHandle) api.BrowserHandle {
	if sess := s.current(h.Owner); sess != nil {
		h.Downloads = s.downloadsAfter(sess, downloadWait)
	}
	return h
}

// ReloadPage reloads this session's page.
func (s *Service) ReloadPage(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	return api.None{}, s.Reload(opts.ID)
}

// GoBack goes back in this session's history.
func (s *Service) GoBack(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	return api.None{}, s.Back(opts.ID)
}

// GoForward goes forward in this session's history.
func (s *Service) GoForward(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	return api.None{}, s.Forward(opts.ID)
}

// ScrollBy scrolls this session's page.
func (s *Service) ScrollBy(_ context.Context, opts api.BrowserScrollOptions) (api.None, error) {
	return api.None{}, s.Scroll(opts.ID, opts.DY)
}

// Handles returns this session's browser, or an empty list.
func (s *Service) Handles(_ context.Context, opts api.SessionIDOptions) ([]api.BrowserHandle, error) {
	list, err := s.List(opts.ID)
	if err != nil {
		return nil, err
	}
	out := make([]api.BrowserHandle, len(list))
	for i, h := range list {
		out[i] = handleAPI(h)
	}
	return out, nil
}

// CloseOne tears down this session's browser.
func (s *Service) CloseOne(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	return api.None{}, s.Close(opts.ID)
}

func handleAPI(h Handle) api.BrowserHandle {
	return api.BrowserHandle{ID: h.ID, URL: h.URL, Title: h.Title, Owner: h.Owner, Headed: h.Headed}
}

func infoAPI(p PageInfo) api.BrowserPageInfo {
	out := api.BrowserPageInfo{URL: p.URL, Title: p.Title, Text: p.Text, Truncated: p.Truncated, Downloads: p.Downloads,
		ConsoleErrors: p.ConsoleErrors, ConsoleErrorsMore: p.ConsoleErrorsMore, Dialogs: p.Dialogs}
	if p.Interactive == nil {
		return out
	}
	out.Interactive = make([]api.BrowserInteractive, len(p.Interactive))
	for i, n := range p.Interactive {
		out.Interactive[i] = api.BrowserInteractive{
			Index: n.Index, Tag: n.Tag, Type: n.Type, Role: n.Role, Text: n.Text, Href: n.Href,
			Rect:    api.BrowserRect{X: n.Rect.X, Y: n.Rect.Y, W: n.Rect.W, H: n.Rect.H},
			Filled:  n.Filled,
			Checked: n.Checked,
		}
	}
	return out
}

func targetOf(t api.BrowserTarget) Target {
	return Target{Index: t.Index, Selector: t.Selector, X: t.X, Y: t.Y}
}

// RunningStates is every browser this process runs, for a window catching up.
func (s *Service) RunningStates(_ context.Context, _ api.None) ([]api.BrowserState, error) {
	return s.Running(), nil
}

// AttachView shows this session's page in a pane.
func (s *Service) AttachView(_ context.Context, opts api.BrowserViewOptions) (api.BrowserHandle, error) {
	h, err := s.Attach(opts.ID, viewportOf(opts))
	return handleAPI(h), err
}

// DetachView stops a pane's frames.
func (s *Service) DetachView(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	return api.None{}, s.Detach(opts.ID)
}

// ResizeView lays the page out at the pane's new size.
func (s *Service) ResizeView(_ context.Context, opts api.BrowserViewOptions) (api.None, error) {
	return api.None{}, s.Resize(opts.ID, viewportOf(opts))
}

// MouseInput passes one pointer event from the pane.
func (s *Service) MouseInput(_ context.Context, opts api.BrowserMouseOptions) (api.None, error) {
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error { return p.Mouse(ctx, opts) })
	return api.None{}, err
}

// KeyInput passes one key from the pane.
func (s *Service) KeyInput(_ context.Context, opts api.BrowserKeyOptions) (api.None, error) {
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error { return p.Key(ctx, opts) })
	return api.None{}, err
}

// TextInput types text the pane's input method committed.
func (s *Service) TextInput(_ context.Context, opts api.BrowserTextOptions) (api.None, error) {
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error { return p.InsertText(ctx, opts.Text) })
	return api.None{}, err
}

// SelectedText is the text selected in this session's page.
func (s *Service) SelectedText(_ context.Context, opts api.SessionIDOptions) (string, error) {
	var text string
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		var err error
		text, err = p.Selection(ctx)
		return err
	})
	return text, err
}

// NavView runs the pane's toolbar.
func (s *Service) NavView(_ context.Context, opts api.BrowserNavOptions) (api.None, error) {
	return api.None{}, s.Nav(opts.ID, opts.Action, opts.URL)
}

func viewportOf(opts api.BrowserViewOptions) Viewport {
	return Viewport{W: opts.W, H: opts.H, Scale: opts.Scale}
}
