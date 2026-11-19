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
	Flame: api.BrowserNewTabFlame{
		Rows: "......... | ...3..... | ......... | ....3.... | ...33.... | ...333... | ..332331. | ..32423.. | 33224223. | .3224223. | ..11111..",
		Core: "#fafafa", Body: "#d4d4d8", Edge: "#a1a1aa", Base: "#27272a",
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
	flameRow = regexp.MustCompile(`^[.1-4]+$`)
)

func validate(l api.BrowserNewTabPageOptions) error {
	if !langTag.MatchString(l.Lang) {
		return fmt.Errorf("invalid language tag %q", l.Lang)
	}
	c, f := l.Colors, l.Flame
	for _, v := range []string{c.Background, c.Card, c.Foreground, c.Muted, c.Border, c.Accent, c.Pass, f.Core, f.Body, f.Edge, f.Base} {
		if !hexColor.MatchString(v) {
			return fmt.Errorf("invalid color %q", v)
		}
	}
	for row := range strings.SplitSeq(f.Rows, " | ") {
		if !flameRow.MatchString(row) {
			return fmt.Errorf("invalid flame row %q", row)
		}
	}
	return nil
}

type pixel struct {
	X, Y   int
	Symbol string
}

type serviceRow struct{ Name, Command string }

type artifactRow struct {
	Name, URL, Stamp, Day string
}

type recentRow struct{ Title, URL, Shown string }

type view struct {
	api.BrowserNewTabPageOptions
	Nonce, Project, Action string
	Error                  bool
	Cols, Rows             int
	Pixels                 []pixel
	Services               []serviceRow
	Worktree, WorktreeHost string
	Artifacts              []artifactRow
	Recent                 []recentRow
}

func (s *Service) page(w http.ResponseWriter, session, project, projectID string, failed bool) {
	v := s.view(session, project, projectID)
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

func (s *Service) view(session, project, projectID string) view {
	look := s.current()
	v := view{BrowserNewTabPageOptions: look, Nonce: newNonce(), Project: project, Action: api.NewTabPrefix + url.PathEscape(session) + "/go"}
	v.Pixels, v.Cols, v.Rows = pixels(look.Flame.Rows)
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

// pixels is the sprite's painted cells, and its size.
func pixels(rows string) (cells []pixel, cols, height int) {
	for y, row := range strings.Split(rows, " | ") {
		cols = max(cols, len(row))
		height = y + 1
		for x, c := range row {
			if c != '.' {
				cells = append(cells, pixel{X: x, Y: y, Symbol: string(c)})
			}
		}
	}
	return cells, cols, height
}

// shortURL is a page's host and path, as the pane's address field shows it.
func shortURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host + strings.TrimSuffix(u.EscapedPath(), "/")
}
