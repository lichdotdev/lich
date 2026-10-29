package browser

import (
	"fmt"
	"net/url"
	"strings"
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
