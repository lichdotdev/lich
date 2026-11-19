// Package newtab serves the page a session's browser opens on: what lich
// already knows of the session (the project's running services, the
// session's artifacts, the pages its browser visited last) and an address
// field. It is drawn with the window's theme and words, which the window
// hands over (api.BrowserNewTabPage), since only it knows the resolved theme
// and the interface language.
package newtab

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// Source is what the page reads, wired at startup.
type Source struct {
	// Project is the id of the project session belongs to; ok is false for a
	// session lich does not know.
	Project func(session string) (projectID string, ok bool)
	// Services is every project's services (internal/services).
	Services func() ([]api.Service, error)
	// WorktreePort is the dev-server port the session's checkout reserves, 0
	// for none, and Listening whether something answers on it.
	WorktreePort func(session string) int
	Listening    func(port int) bool
	// Artifacts is the session's artifacts, newest first, each with the URL
	// its browser loads it by.
	Artifacts func(session string) []Artifact
	// Resolve is the URL the session's browser loads for raw (internal/browser),
	// which refuses what the agent's tools refuse.
	Resolve func(session, raw string) (string, error)
	// Key is the session's files key, which its recent pages are kept under;
	// Load and Save read and write them, Drop forgets them.
	Key  func(session string) string
	Load func(key string) string
	Save func(key, value string) error
	Drop func(key string) error
	// Skip reports a URL that is no page to remember: the new-tab page itself.
	Skip func(raw string) bool
}

// Artifact is one of the session's artifacts as the page lists it.
type Artifact struct {
	Name     string
	URL      string
	Modified int64
}

// Service renders the page and remembers the pages a browser visited.
type Service struct {
	src Source

	mu      sync.Mutex
	look    api.BrowserNewTabPageOptions
	recents map[string][]Recent
}

// New returns a Service reading src. Until the window hands over its look the
// page is drawn with Fallback.
func New(src Source) *Service {
	return &Service{src: src, look: Fallback, recents: map[string][]Recent{}}
}

// Look takes the window's theme and words. A color or language that is not
// one is refused, as they are written into the page's CSS and markup.
func (s *Service) Look(look api.BrowserNewTabPageOptions) error {
	if err := validate(look); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.look = look
	return nil
}

func (s *Service) current() api.BrowserNewTabPageOptions {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.look
}

// Handler serves api.NewTabPrefix + session, and its /go, which loads what the
// address field holds in the session's browser.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, api.NewTabPrefix), "/")
		if !strings.HasPrefix(r.URL.Path, api.NewTabPrefix) || session == "" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		projectID, known := s.src.Project(session)
		if !known {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		switch rest {
		case "":
			s.page(w, session, projectID, r.URL.Query().Get("err") != "")
		case "go":
			s.goTo(w, r, session)
		default:
			http.NotFound(w, r)
		}
	})
}

// goTo sends the browser where the address field said, by the same rule the
// pane's own address bar loads by; a refused address brings the page back with
// a line saying so.
func (s *Service) goTo(w http.ResponseWriter, r *http.Request, session string) {
	back := api.NewTabPrefix + url.PathEscape(session)
	text := strings.TrimSpace(r.URL.Query().Get("q"))
	if text == "" {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	target, err := s.src.Resolve(session, api.TypedURL(text))
	if err != nil || !isWeb(target) {
		http.Redirect(w, r, back+"?err=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func isWeb(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
