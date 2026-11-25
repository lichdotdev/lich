package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/browserwire"
)

// fakeBinary is a lich-browser that writes to its stderr, answers hello and
// waits for its stdin to close.
func fakeBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lich-browser")
	script := "#!/bin/sh\necho engine says hi >&2\nread line\necho '{\"id\":1,\"result\":{\"version\":1}}'\nread line\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSpawnProcessSpeaksOverItsPipesAndLogsItsStderr(t *testing.T) {
	t.Setenv("LICH_DEV", "")
	logDir := t.TempDir()
	c, err := spawnProcess(fakeBinary(t), logDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := hello(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	c.fail(context.Canceled)
	if err := c.call(t.Context(), browserwire.Clock, nil, nil); err != errBrowserGone {
		t.Fatalf("a call after the end = %v, want errBrowserGone", err)
	}
	waitFor(t, "the stderr in the log", func() bool {
		b, _ := os.ReadFile(filepath.Join(logDir, "lich-browser.log"))
		return strings.Contains(string(b), "engine says hi")
	})
}

func TestFindLichBrowserNeedsTheBinaryAndAnXDisplay(t *testing.T) {
	bin := fakeBinary(t)
	for _, tc := range []struct {
		name, display, pinned string
		want                  bool
	}{
		{name: "found on X", display: ":0", pinned: bin, want: true},
		{name: "no X display", pinned: bin},
		{name: "no binary", display: ":0", pinned: filepath.Join(t.TempDir(), "missing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISPLAY", tc.display)
			t.Setenv(BrowserEnv, tc.pinned)
			if got := findLichBrowser(t.TempDir()) != nil; got != tc.want {
				t.Fatalf("found = %v, want %v", got, tc.want)
			}
		})
	}
}
