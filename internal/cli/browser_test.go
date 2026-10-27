package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestBrowserOpenCallsThisSessionsBrowser(t *testing.T) {
	f := newFakeLich(t, `{"id":"b1","url":"https://example.com","owner":"s1","headed":false}`)

	code, stdout, stderr := run(t, f, "browser", "open", "https://example.com")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	opts := optionsOf[api.BrowserOpenOptions](t, f.only(t))
	if opts.ID != "s1" || opts.URL != "https://example.com" {
		t.Errorf("options = %+v", opts)
	}
	if !strings.Contains(stdout, "https://example.com") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestBrowserOpenNamesAFileItDownloaded(t *testing.T) {
	f := newFakeLich(t, `{"id":"b1","url":"about:blank","owner":"s1",
		"downloads":[{"path":"/home/u/Downloads/r.csv","bytes":12,"state":"completed"}]}`)

	code, stdout, stderr := run(t, f, "browser", "open", "https://example.com/r.csv")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if want := "about:blank\ndownloaded: /home/u/Downloads/r.csv (12 B)\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBrowserClickSendsATarget(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "browser", "click", "--index", "3")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	opts := optionsOf[api.BrowserClickOptions](t, f.only(t))
	if opts.ID != "s1" || opts.Index == nil || *opts.Index != 3 {
		t.Errorf("options = %+v", opts)
	}
}

func TestBrowserPressSendsKeyAndOptionalTarget(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "browser", "press", "--index", "2", "Enter")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	opts := optionsOf[api.BrowserPressOptions](t, f.only(t))
	if opts.ID != "s1" || opts.Key != "Enter" || opts.Index == nil || *opts.Index != 2 {
		t.Errorf("options = %+v", opts)
	}
}

func TestMCPBrowserPressCallsPress(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"browser_press","arguments":{"key":"Tab"}}}`)
	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	opts := optionsOf[api.BrowserPressOptions](t, f.only(t))
	if opts.ID != "s1" || opts.Key != "Tab" {
		t.Errorf("options = %+v", opts)
	}
}

func TestBrowserRefusesWithoutASession(t *testing.T) {
	f := newFakeLich(t, `null`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"browser", "open", "https://example.com"}, &client{
		env: func(key string) string {
			if key == "LICH_SESSION_ID" {
				return ""
			}
			return f.env(key)
		},
		version: testVersion, stdout: &stdout, stderr: &stderr, running: noInstance,
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "inside a lich session") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if len(f.calls) != 0 {
		t.Errorf("reached the app without a session: %+v", f.calls)
	}
}

func TestMCPBrowserOpenUsesTheCaller(t *testing.T) {
	f := newFakeLich(t, `{"id":"b1","url":"https://example.com","owner":"s1","headed":false}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"browser_open","arguments":{"url":"https://example.com"}}}`)
	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	opts := optionsOf[api.BrowserOpenOptions](t, f.only(t))
	if opts.ID != "s1" || opts.URL != "https://example.com" {
		t.Errorf("options = %+v", opts)
	}
}
