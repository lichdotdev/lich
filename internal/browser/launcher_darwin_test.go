package browser

import (
	"path/filepath"
	"testing"
)

func TestFindLichBrowserNeedsTheBinary(t *testing.T) {
	for _, tc := range []struct {
		name, pinned string
		want         bool
	}{
		{name: "found", pinned: fakeBinary(t), want: true},
		{name: "no binary", pinned: filepath.Join(t.TempDir(), "missing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(BrowserEnv, tc.pinned)
			if got := findLichBrowser(t.TempDir()) != nil; got != tc.want {
				t.Fatalf("found = %v, want %v", got, tc.want)
			}
		})
	}
}
