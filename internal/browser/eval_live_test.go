package browser

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

const evalPage = `<title>Eval</title>
<input id="pw" type="password" value="secret">
<script>window.app = {user: {name: "ana"}, items: [1, 2]}</script>`

func TestChromeEvalAnswersWhatTheScriptReturns(t *testing.T) {
	s, url := liveEvalService(t)
	if _, err := s.Open("s", url); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ script, want string }{
		{`document.title`, "Eval"},
		{`app`, `{"user":{"name":"ana"},"items":[1,2]}`},
		{`app.items.length + 1`, "3"},
		{`let n = 1; n`, "1"},
		{`let n = 2; n`, "2"},
		{`(await fetch("/api")).status`, "201"},
		{`void 0`, "undefined"},
		{`0 / 0`, "NaN"},
		{`pw.value`, "secret"},
	}
	for _, c := range cases {
		got, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: c.script})
		if err != nil {
			t.Fatalf("%s: %v", c.script, err)
		}
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.script, got, c.want)
		}
	}
}

func TestChromeEvalFailsOnAThrowACycleAndADialog(t *testing.T) {
	s, url := liveEvalService(t)
	if _, err := s.Open("s", url); err != nil {
		t.Fatal(err)
	}
	_, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: `throw new Error("boom")`})
	if err == nil || !strings.Contains(err.Error(), "Error: boom") {
		t.Fatalf("a throwing script answered %v, want the exception", err)
	}
	if out, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: `const o = {}; o.o = o; o`}); err == nil {
		t.Fatalf("a cyclic value answered %q, want an error", out)
	}
	if out, _ := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: `document.body`}); out != "{}" {
		t.Errorf("a DOM node answered %q, want {}", out)
	}
	_, err = s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: `alert("hi")`})
	if !errors.Is(err, errDialogOpen) || !strings.Contains(err.Error(), "browser_dialog") {
		t.Fatalf("a script that opened an alert answered %v, want the dialog", err)
	}
}

func liveEvalService(t *testing.T) (*Service, string) {
	t.Helper()
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, evalPage)
		case "/api":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	// Made before the service, so that its removal runs after Cleanup closed
	// Chrome: Windows refuses to delete a file Chrome still holds.
	profile := t.TempDir()
	s := newService(chromeLaunch, func(string, string) (string, error) { return profile, nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	return s, srv.URL + "/"
}

func TestChromePageSeesTheMouseThePaneGivesIt(t *testing.T) {
	s, url := liveEvalService(t)
	if _, err := s.Open("s", url); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"(hover: hover)", "(any-hover: hover)", "(pointer: fine)", "(any-pointer: fine)"} {
		got, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: fmt.Sprintf("matchMedia(%q).matches", query)})
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != "true" {
			t.Errorf("%s: %s, want true", query, got)
		}
	}
}
