package newtab

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// maxArtifacts is how many of the session's artifacts the page lists, newest
// first; the Artifacts tab holds them all.
const maxArtifacts = 8

// Fallback is the look before the window hands over its own: the dark theme.
var Fallback = api.BrowserNewTabPageOptions{
	Lang: "en",
	Colors: api.BrowserNewTabColors{
		Background: "#101012", Card: "#18181b", Foreground: "#fafafa", Muted: "#a1a1aa",
		Border: "#ffffff1a", Accent: "#27272a", Pass: "#10b981",
	},
	Text: api.BrowserNewTabText{
		Title: "New tab", Address: "Enter an address, or pick below", AddressError: "That address cannot be opened.",
		Running: "Running in this project", Worktree: "Worktree port",
		Artifacts: "Artifacts of this session", Recent: "Recent",
	},
}

var (
	hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)
	langTag  = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
)

func validate(l api.BrowserNewTabPageOptions) error {
	if !langTag.MatchString(l.Lang) {
		return fmt.Errorf("invalid language tag %q", l.Lang)
	}
	c := l.Colors
	for _, v := range []string{c.Background, c.Card, c.Foreground, c.Muted, c.Border, c.Accent, c.Pass} {
		if !hexColor.MatchString(v) {
			return fmt.Errorf("invalid color %q", v)
		}
	}
	return nil
}

type serviceRow struct{ Name, Command string }

type artifactRow struct {
	Name, URL, Stamp, Day string
}

type recentRow struct{ Title, URL, Shown string }

type view struct {
	api.BrowserNewTabPageOptions
	Nonce, Action          string
	Error                  bool
	Services               []serviceRow
	Worktree, WorktreeHost string
	Artifacts              []artifactRow
	Recent                 []recentRow
}

func (s *Service) page(w http.ResponseWriter, session, projectID string, failed bool) {
	v := s.view(session, projectID)
	v.Error = failed
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// No form-action: Chrome applies it to the redirect /go answers with, which
	// leaves for the address the person typed.
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-"+v.Nonce+"'; base-uri 'none'")
	if err := pageTemplate.Execute(w, v); err != nil {
		http.Error(w, "page failed", http.StatusInternalServerError)
	}
}

func (s *Service) view(session, projectID string) view {
	look := s.current()
	v := view{BrowserNewTabPageOptions: look, Nonce: newNonce(), Action: api.NewTabPrefix + url.PathEscape(session) + "/go"}
	if all, err := s.src.Services(); err == nil {
		for _, svc := range all {
			if svc.ProjectID == projectID && svc.State == api.ServiceRunning {
				v.Services = append(v.Services, serviceRow{Name: svc.Name, Command: svc.Command})
			}
		}
	}
	if port := s.src.WorktreePort(session); port != 0 && s.src.Listening(port) {
		v.WorktreeHost = net.JoinHostPort("localhost", strconv.Itoa(port))
		v.Worktree = "http://" + v.WorktreeHost
	}
	artifacts := s.src.Artifacts(session)
	for _, a := range artifacts[:min(maxArtifacts, len(artifacts))] {
		at := time.UnixMilli(a.Modified)
		v.Artifacts = append(v.Artifacts, artifactRow{Name: a.Name, URL: a.URL, Stamp: at.UTC().Format(time.RFC3339), Day: at.UTC().Format("2006-01-02")})
	}
	for _, r := range s.recent(session) {
		v.Recent = append(v.Recent, recentRow{Title: r.Title, URL: r.URL, Shown: shortURL(r.URL)})
	}
	return v
}

// shortURL is a page's host and path, as the pane's address field shows it.
func shortURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host + strings.TrimSuffix(u.EscapedPath(), "/")
}
