//go:build linux || darwin

package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/browserwire"
)

// fakeBinary is a lich-browser built with this lich: it writes to its
// stderr, answers hello with this wire's version and waits for its stdin to
// close.
func fakeBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lich-browser")
	script := fmt.Sprintf("#!/bin/sh\necho engine says hi >&2\nread line\necho '{\"id\":1,\"result\":{\"version\":%d}}'\nread line\n", browserwire.Version)
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
