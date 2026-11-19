package preview

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestNewTabURLServesThePageOnItsOwnListener(t *testing.T) {
	s := newTestService(t)
	s.SetNewTab(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("tab " + r.URL.Path)) }))
	u, err := s.NewTabURL("s1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "http://127.0.0.1:") || !strings.HasSuffix(u, api.NewTabPrefix+"s1") {
		t.Fatalf("URL = %q", u)
	}
	if code, body := get(t, u); code != http.StatusOK || body != "tab /new-tab/s1" {
		t.Fatalf("GET %s = %d %q", u, code, body)
	}
	if again, _ := s.NewTabURL("s2"); !strings.HasPrefix(again, u[:strings.Index(u, api.NewTabPrefix)]) {
		t.Fatalf("a second session got another listener: %q and %q", u, again)
	}
}

func TestNewTabListenerRefusesAForeignHost(t *testing.T) {
	s := newTestService(t)
	s.SetNewTab(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("tab")) }))
	u, err := s.NewTabURL("s1")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Host = "evil.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a rebound host got %d", resp.StatusCode)
	}
}

func TestNewTabURLWithoutAPageIsAnError(t *testing.T) {
	if _, err := newTestService(t).NewTabURL("s1"); err == nil {
		t.Fatal("no page, no error")
	}
}

func TestIsNewTabIsTheListenersOwnOriginOnly(t *testing.T) {
	s := newTestService(t)
	if s.IsNewTab("http://127.0.0.1:27000/new-tab/s1") {
		t.Fatal("true before the listener started")
	}
	s.SetNewTab(http.NotFoundHandler())
	u, err := s.NewTabURL("s1")
	if err != nil {
		t.Fatal(err)
	}
	host := u[:strings.Index(u, api.NewTabPrefix)]
	for raw, want := range map[string]bool{
		u:                                 true,
		host + api.NewTabPrefix + "s2/go": true,
		host + "/other":                   false,
		"http://127.0.0.1:1" + api.NewTabPrefix + "s1":  false,
		"https://example.com" + api.NewTabPrefix + "s1": false,
		"about:blank": false,
		"::bad":       false,
	} {
		if got := s.IsNewTab(raw); got != want {
			t.Errorf("IsNewTab(%q) = %v", raw, got)
		}
	}
}
