package agentplugin

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// captureLog sends slog to a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// Every release but the current one and the ones live sessions load goes; a
// half-unpacked one is the unpack's, and nothing goes while lich cannot say
// what is in use.
func TestPruneReleases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inUse   []string // versions a live session loads
		noInUse bool     // the terminal never said what is in use
		want    []string // versions left
	}{
		{name: "the current one alone is kept", want: []string{"0.14.0"}},
		{name: "one a session loads is kept", inUse: []string{"0.13.0"}, want: []string{"0.13.0", "0.14.0"}},
		{name: "every older one in use is kept", inUse: []string{"0.13.0", "0.12.0"}, want: []string{"0.12.0", "0.13.0", "0.14.0"}},
		{name: "nothing goes without knowing what is in use", noInUse: true, want: []string{"0.12.0", "0.13.0", "0.14.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := bundleHome(t)
			for _, v := range []string{"0.12.0", "0.13.0", "0.14.0"} {
				plantRelease(t, root, v)
			}
			tmp := filepath.Join(root, bundleTmpPrefix+"0.15.0-1")
			if err := os.MkdirAll(tmp, 0o755); err != nil {
				t.Fatal(err)
			}
			s := &Service{}
			if !tc.noInUse {
				s.SetInUse(func() []string {
					var dirs []string
					for _, v := range tc.inUse {
						dirs = append(dirs, filepath.Join(root, v))
					}
					return dirs
				})
			}

			s.pruneReleases()

			for _, v := range []string{"0.12.0", "0.13.0", "0.14.0"} {
				_, err := os.Stat(filepath.Join(root, v))
				if kept := err == nil; kept != slices.Contains(tc.want, v) {
					t.Errorf("%s kept = %v, want %v", v, kept, slices.Contains(tc.want, v))
				}
			}
			if _, err := os.Stat(tmp); err != nil {
				t.Errorf("a release being unpacked was removed: %v", err)
			}
		})
	}
}

// A release that cannot be removed is logged and left for the next pass,
// and the others still go.
func TestPruneReleasesLogsAFailedRemoval(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory its owner cannot write")
	}
	root := bundleHome(t)
	for _, v := range []string{"0.12.0", "0.13.0", "0.14.0"} {
		plantRelease(t, root, v)
	}
	locked := filepath.Join(root, "0.12.0", "hooks")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	logged := captureLog(t)
	s := &Service{}
	s.SetInUse(func() []string { return nil })

	s.pruneReleases()

	if _, err := os.Stat(filepath.Join(root, "0.12.0")); err != nil {
		t.Errorf("the release that could not go is gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "0.13.0")); !os.IsNotExist(err) {
		t.Errorf("an unused release stayed beside the failure (stat err = %v)", err)
	}
	if !strings.Contains(logged.String(), "remove an unused plugin release") || !strings.Contains(logged.String(), "0.12.0") {
		t.Errorf("log = %q, want the failed removal named", logged)
	}
}

// A pass prunes once the release is unpacked, keeping the one a live session
// loads.
func TestEnsurePrunesAfterUnpacking(t *testing.T) {
	s, _ := fileServerAt(t, "0.14.0", nil)
	s.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	root := bundleHome(t)
	inUse := plantRelease(t, root, "0.13.0")
	plantRelease(t, root, "0.13.5")
	s.SetInUse(func() []string { return []string{inUse} })

	s.Ensure()

	for v, want := range map[string]bool{"0.14.0": true, "0.13.0": true, "0.13.5": false} {
		if _, err := os.Stat(filepath.Join(root, v)); (err == nil) != want {
			t.Errorf("%s kept = %v, want %v", v, err == nil, want)
		}
	}
}
