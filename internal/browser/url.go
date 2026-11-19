package browser

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// allowURL is the only URL the agent browser will load. http, https and
// about:blank are in; file, javascript, data, chrome and everything else are
// out — a file: URL would hand the agent the machine, and javascript: is just
// evaluate by another name.
func allowURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "about:blank") {
		return "about:blank", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return "", fmt.Errorf("url %q has no host", raw)
		}
		return u.String(), nil
	default:
		if u.Scheme == "" {
			return "", fmt.Errorf("url %q is missing a scheme", raw)
		}
		return "", fmt.Errorf("blocked url scheme %q", u.Scheme)
	}
}

// SetLocal sets how a local file, a file: URL or an absolute path, becomes a
// URL a session's browser may load: the http URL that serves it from that
// session's checkout (internal/preview). Unset, a local file is refused like
// any other blocked URL.
func (s *Service) SetLocal(isLocal func(raw string) bool, local func(owner, raw string) (string, error)) {
	s.isLocal, s.local = isLocal, local
}

// checkURL refuses what resolveURL would refuse without knowing the owner: a
// blocked URL fails before the browser is looked up.
func (s *Service) checkURL(raw string) error {
	if s.local != nil && s.isLocal(raw) {
		return nil
	}
	_, err := allowURL(raw)
	return err
}

// resolveURL is the URL owner's browser loads for raw: a local file served
// from owner's checkout, or raw itself when allowURL lets it through.
func (s *Service) resolveURL(owner, raw string) (string, error) {
	if s.local != nil && s.isLocal(raw) {
		return s.local(owner, raw)
	}
	return allowURL(raw)
}

// NewTab is the page a new tab, and the first page of a browser the window
// opened with no address, loads (internal/newtab, served by internal/preview).
type NewTab struct {
	// URL is its URL for owner, Is tells it from any other page, Visit hears
	// every page the browser comes to, and Look takes the window's theme and
	// words for it.
	URL   func(owner string) (string, error)
	Is    func(raw string) bool
	Visit func(owner, url, title string)
	Look  func(api.BrowserNewTabPageOptions) error
}

// SetNewTab sets the new-tab page. Startup wiring. Unset, those open at
// about:blank. The agent's own open and new tab keep about:blank: nobody looks
// at a page it has not asked for.
func (s *Service) SetNewTab(n NewTab) { s.newTab = n }

// blankURL is where owner's browser opens with no address: the new-tab page,
// or about:blank without one.
func (s *Service) blankURL(owner string) string {
	if s.newTab.URL == nil {
		return "about:blank"
	}
	u, err := s.newTab.URL(owner)
	if err != nil {
		slog.Warn("browser: new-tab page", "err", err)
		return "about:blank"
	}
	return u
}

// ResolveURL is the URL owner's browser loads for raw, or the reason it
// refuses it: what Navigate checks, for a caller that sends the browser there
// itself (the new-tab page's address field).
func (s *Service) ResolveURL(owner, raw string) (string, error) {
	if err := s.checkURL(raw); err != nil {
		return "", err
	}
	return s.resolveURL(owner, raw)
}
