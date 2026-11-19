package newtab

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// kept is a fake settings store.
type kept struct {
	values map[string]string
	saves  int
}

func (k *kept) Load(key string) string { return k.values[key] }
func (k *kept) Save(key, value string) error {
	k.saves++
	k.values[key] = value
	return nil
}
func (k *kept) Drop(key string) error { delete(k.values, key); return nil }

type fixture struct {
	src  Source
	kept *kept
}

func newFixture() *fixture {
	k := &kept{values: map[string]string{}}
	return &fixture{kept: k, src: Source{
		Project:      func(session string) (string, string, bool) { return "todo-api", "p1", session == "s1" },
		Services:     func() ([]api.Service, error) { return nil, nil },
		WorktreePort: func(string) int { return 0 },
		Listening:    func(int) bool { return true },
		Artifacts:    func(string) []Artifact { return nil },
		Resolve:      func(_, raw string) (string, error) { return raw, nil },
		Key:          func(session string) string { return "key-" + session },
		Load:         k.Load, Save: k.Save, Drop: k.Drop,
		Skip: func(raw string) bool { return strings.Contains(raw, "/new-tab/") },
	}}
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestPageOfAnEmptySessionShowsOnlyTheAddressField(t *testing.T) {
	body := get(New(newFixture().src).Handler(), api.NewTabPrefix+"s1").Body.String()
	for _, want := range []string{"lich · todo-api", `placeholder="Search or enter address"`, `action="/new-tab/s1/go"`, "<title>New tab</title>"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	for _, gone := range []string{"Running in this project", "Artifacts of this session", "Recent</h2>"} {
		if strings.Contains(body, gone) {
			t.Errorf("an empty section %q is shown", gone)
		}
	}
}

func TestPageListsWhatLichKnows(t *testing.T) {
	f := newFixture()
	f.src.Services = func() ([]api.Service, error) {
		return []api.Service{
			{ProjectID: "p1", Name: "web", Command: "pnpm dev", State: api.ServiceRunning},
			{ProjectID: "p1", Name: "stopped-one", State: api.ServiceStopped},
			{ProjectID: "p2", Name: "other-project", State: api.ServiceRunning},
		}, nil
	}
	f.src.WorktreePort = func(string) int { return 24796 }
	f.src.Artifacts = func(string) []Artifact {
		return []Artifact{{Name: "report.html", URL: "http://127.0.0.1:26100/report.html", Modified: 1_700_000_000_000}}
	}
	s := New(f.src)
	s.Visit("s1", "https://example.com/docs", "Docs")
	body := get(s.Handler(), api.NewTabPrefix+"s1").Body.String()
	for _, want := range []string{
		"Running in this project", "web", "pnpm dev", `href="http://localhost:24796"`, `>localhost:24796<`, "this worktree&#39;s port",
		"Artifacts of this session", `href="http://127.0.0.1:26100/report.html"`, `datetime="2023-11-14T22:13:20Z"`,
		"Recent", `href="https://example.com/docs"`, "Docs", "example.com/docs",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q:\n%s", want, body)
		}
	}
	for _, gone := range []string{"stopped-one", "other-project"} {
		if strings.Contains(body, gone) {
			t.Errorf("page lists %q", gone)
		}
	}
}

func TestPageLeavesOutAWorktreePortNothingAnswersOn(t *testing.T) {
	f := newFixture()
	f.src.WorktreePort = func(string) int { return 24796 }
	f.src.Listening = func(int) bool { return false }
	if body := get(New(f.src).Handler(), api.NewTabPrefix+"s1").Body.String(); strings.Contains(body, "24796") {
		t.Fatalf("a port nothing listens on is listed:\n%s", body)
	}
}

func TestPageCapsItsArtifacts(t *testing.T) {
	f := newFixture()
	f.src.Artifacts = func(string) []Artifact {
		var out []Artifact
		for i := range 20 {
			out = append(out, Artifact{Name: string(rune('a'+i)) + ".html", URL: "http://127.0.0.1:1/x", Modified: 1})
		}
		return out
	}
	body := get(New(f.src).Handler(), api.NewTabPrefix+"s1").Body.String()
	if n := strings.Count(body, `<time class="r"`); n != maxArtifacts {
		t.Fatalf("%d artifacts listed, want %d", n, maxArtifacts)
	}
}

// What a page titles itself, and what a file is called, is the agent's to write.
func TestPageEscapesEverythingTheAgentWrites(t *testing.T) {
	const hostile = `<script>alert(1)</script>"'`
	f := newFixture()
	f.src.Project = func(string) (string, string, bool) { return hostile, "p1", true }
	f.src.Services = func() ([]api.Service, error) {
		return []api.Service{{ProjectID: "p1", Name: hostile, Command: hostile, State: api.ServiceRunning}}, nil
	}
	f.src.Artifacts = func(string) []Artifact {
		return []Artifact{{Name: hostile, URL: "javascript:alert(1)", Modified: 1}}
	}
	s := New(f.src)
	s.Visit("s1", `https://example.com/"><script>alert(2)</script>`, hostile)
	rec := get(s.Handler(), api.NewTabPrefix+"s1")
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert") || strings.Contains(body, `"><script`) {
		t.Fatalf("markup got through:\n%s", body)
	}
	if strings.Contains(body, `href="javascript:`) {
		t.Fatalf("a javascript: URL got through:\n%s", body)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "nonce-") {
		t.Fatalf("CSP = %q", csp)
	}
	if strings.Contains(csp, "form-action") {
		t.Fatalf("CSP = %q: form-action would block the redirect the address field loads by", csp)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("the page may be cached")
	}
}

func TestPageIsDrawnWithTheWindowsTheme(t *testing.T) {
	s := New(newFixture().src)
	look := Fallback
	look.Lang = "pt-BR"
	look.Colors.Background, look.Colors.Card = "#f0f0f2", "#fafafa"
	look.Colors.Border = "#ffffff1a"
	look.Text.Title, look.Text.Recent = "Nova aba", "Recentes"
	if err := s.Look(look); err != nil {
		t.Fatal(err)
	}
	s.Visit("s1", "https://example.com/", "")
	body := get(s.Handler(), api.NewTabPrefix+"s1").Body.String()
	for _, want := range []string{`lang="pt-BR"`, "--bg:#f0f0f2", "--card:#fafafa", "--border:#ffffff1a", "<title>Nova aba</title>", "Recentes"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Fatalf("html/template filtered a value:\n%s", body)
	}
}

func TestPageDrawsTheFlameOnce(t *testing.T) {
	body := get(New(newFixture().src).Handler(), api.NewTabPrefix+"s1").Body.String()
	if !strings.Contains(body, `viewBox="0 0 9 11"`) || !strings.Contains(body, `class="f4"`) {
		t.Fatalf("no flame:\n%s", body)
	}
}

func TestLookRefusesWhatCouldBreakOutOfTheStyleOrTheMarkup(t *testing.T) {
	for name, edit := range map[string]func(*api.BrowserNewTabPageOptions){
		"color":       func(l *api.BrowserNewTabPageOptions) { l.Colors.Muted = "red;}</style><script>" },
		"flame color": func(l *api.BrowserNewTabPageOptions) { l.Flame.Core = "#12" },
		"language":    func(l *api.BrowserNewTabPageOptions) { l.Lang = `en"><script>` },
		"flame rows":  func(l *api.BrowserNewTabPageOptions) { l.Flame.Rows = "..x.." },
	} {
		look := Fallback
		edit(&look)
		if err := New(newFixture().src).Look(look); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := New(newFixture().src).Look(Fallback); err != nil {
		t.Fatalf("the fallback is not valid: %v", err)
	}
}

func TestHandlerAnswers404ForWhatItDoesNotServe(t *testing.T) {
	h := New(newFixture().src).Handler()
	for _, target := range []string{"/new-tab/unknown", "/new-tab/", "/other", "/new-tab/s1/elsewhere", "/new-tab/unknown/go?q=x"} {
		if rec := get(h, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s answered %d", target, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/new-tab/s1", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST answered %d", rec.Code)
	}
}

func TestGoLoadsWhatTheAddressBarWould(t *testing.T) {
	f := newFixture()
	var resolved []string
	f.src.Resolve = func(_, raw string) (string, error) {
		resolved = append(resolved, raw)
		if raw == "file:///etc/passwd" {
			return "", errors.New("outside the checkout")
		}
		return "http://resolved.test/", nil
	}
	h := New(f.src).Handler()
	for _, c := range [][2]string{
		{"/new-tab/s1/go?q=localhost:3000", "http://resolved.test/"},
		{"/new-tab/s1/go?q=example.com", "http://resolved.test/"},
		{"/new-tab/s1/go?q=", "/new-tab/s1"},
		{"/new-tab/s1/go", "/new-tab/s1"},
		{"/new-tab/s1/go?q=file:///etc/passwd", "/new-tab/s1?err=1"},
	} {
		target, want := c[0], c[1]
		rec := get(h, target)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
			t.Errorf("%s: %d to %q, want %q", target, rec.Code, rec.Header().Get("Location"), want)
		}
	}
	if strings.Join(resolved, " ") != "http://localhost:3000 https://example.com file:///etc/passwd" {
		t.Errorf("resolved %q", resolved)
	}
	if body := get(h, "/new-tab/s1?err=1").Body.String(); !strings.Contains(body, "That address cannot be opened.") {
		t.Errorf("no error line:\n%s", body)
	}
}

func TestGoRefusesATargetThatIsNotAWebPage(t *testing.T) {
	f := newFixture()
	f.src.Resolve = func(_, _ string) (string, error) { return "about:blank", nil }
	rec := get(New(f.src).Handler(), "/new-tab/s1/go?q=x")
	if loc := rec.Header().Get("Location"); loc != "/new-tab/s1?err=1" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestSessionIDsAreEscapedInTheFormAction(t *testing.T) {
	f := newFixture()
	f.src.Project = func(string) (string, string, bool) { return "p", "p1", true }
	body := get(New(f.src).Handler(), "/new-tab/a%20b").Body.String()
	if !strings.Contains(body, `action="/new-tab/a%20b/go"`) {
		t.Fatalf("action not escaped:\n%s", body)
	}
}
