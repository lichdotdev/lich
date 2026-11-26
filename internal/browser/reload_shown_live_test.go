package browser

import (
	"net/http"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// The window's own page follows only a URL that changed, so every load of the
// URL the page is at has to reach it as a reload, whoever asked for it.
func TestEveryLoadAtTheSameURLReachesTheWindow(t *testing.T) {
	t.Parallel()
	s := settleService(t, `<title>page</title>`, http.NotFound)
	url := s.Running()[0].URL
	// A browser in its first seconds reloads the page once more on its own
	// after the agent's reload (startupGrace), which the window follows too.
	time.Sleep(startupGrace)
	steps := []struct {
		name string
		run  func() error
	}{
		{"agent reload", func() error { return s.Reload("s") }},
		{"toolbar reload", func() error { return s.Nav("s", "reload", "") }},
		{"script reload", func() error {
			_, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "location.reload()"})
			return err
		}},
		{"agent opens the same URL", func() error { _, err := s.Open("s", url); return err }},
	}
	for i, step := range steps {
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		waitReloads(t, s, i+1, step.name)
	}
	if _, err := s.Open("s", url+"?other"); err != nil {
		t.Fatal(err)
	}
	waitReloads(t, s, len(steps), "agent opens another URL")
}

// waitReloads waits for the state's Reloads to reach want, then for it to
// hold there a moment, so a step counted twice fails.
func waitReloads(t *testing.T, s *Service, want int, step string) {
	t.Helper()
	deadline := time.Now().Add(pageWait)
	got := -1
	for time.Now().Before(deadline) {
		if got = s.Running()[0].Reloads; got == want {
			break
		}
		time.Sleep(downloadPoll)
	}
	time.Sleep(500 * time.Millisecond)
	if got = s.Running()[0].Reloads; got != want {
		t.Fatalf("after %s: Reloads = %d, want %d", step, got, want)
	}
}
