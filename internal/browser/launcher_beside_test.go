//go:build darwin || windows

package browser

import (
	"os"
	"path/filepath"
	"testing"
)

// On macOS and Windows a lich-browser beside lich is all it takes.
func TestFindLichBrowserNeedsTheBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "lich-browser.exe")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, pinned string
		want         bool
	}{
		{name: "found", pinned: bin, want: true},
		{name: "no binary", pinned: filepath.Join(t.TempDir(), "missing.exe")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(BrowserEnv, tc.pinned)
			if got := findLichBrowser(t.TempDir()) != nil; got != tc.want {
				t.Fatalf("found = %v, want %v", got, tc.want)
			}
		})
	}
}
