package browser

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// dialogPage opens one dialog per button and titles the page with what the
// dialog answered, so the test sees the page ran on past it.
const dialogPage = `<title>Dialogs</title><style>button{display:block;font-size:30px}</style>
<button id="alert" onclick="alert('hi'); document.title = 'alert done'">alert</button>
<button id="confirm" onclick="document.title = 'confirm ' + confirm('Delete?')">confirm</button>
<button id="prompt" onclick="document.title = 'prompt ' + prompt('Name?', 'Ada')">prompt</button>
<script>addEventListener('beforeunload', e => e.preventDefault())</script>`

func TestChromeADialogWaitsForTheAgentsAnswer(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	testDownloads(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, dialogPage)
	}))
	t.Cleanup(srv.Close)
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	if _, err := s.Open("s", srv.URL); err != nil {
		t.Fatal(err)
	}
	bob := "Bob"
	for _, c := range []struct {
		id, message, title string
		answer             api.BrowserDialogOptions
	}{
		{"alert", "hi", "alert done", api.BrowserDialogOptions{ID: "s", Accept: true}},
		{"confirm", "Delete?", "confirm false", api.BrowserDialogOptions{ID: "s"}},
		{"prompt", "Name?", "prompt Bob", api.BrowserDialogOptions{ID: "s", Accept: true, Text: &bob}},
	} {
		t.Run(c.id, func(t *testing.T) {
			start := time.Now()
			info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#" + c.id}})
			if err != nil {
				t.Fatalf("click after %s: %v", time.Since(start), err)
			}
			if since := time.Since(start); info.Dialog == nil || info.Dialog.Type != c.id || info.Dialog.Message != c.message || since > 5*time.Second {
				t.Fatalf("click answered %+v after %s, want the open %s at once", info.Dialog, since, c.id)
			}
			after, err := s.AnswerDialog(t.Context(), c.answer)
			if err != nil {
				t.Fatal(err)
			}
			if after.Title != c.title || after.Dialog != nil {
				t.Fatalf("after the answer: title %q, dialog %+v; want %q and none", after.Title, after.Dialog, c.title)
			}
		})
	}
	t.Run("beforeunload", func(t *testing.T) {
		next := srv.URL + "/next"
		// The clicks above gave the page the user activation Chrome asks of a
		// beforeunload dialog.
		if _, err := s.Go(t.Context(), api.BrowserOpenOptions{ID: "s", URL: next}); !errors.Is(err, errDialogOpen) {
			t.Fatalf("navigate: err = %v, want the beforeunload held open", err)
		}
		after, err := s.AnswerDialog(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true})
		if err != nil {
			t.Fatal(err)
		}
		if after.URL != next || after.Dialog != nil {
			t.Fatalf("after Leave: on %s with dialog %+v, want %s", after.URL, after.Dialog, next)
		}
	})
}
