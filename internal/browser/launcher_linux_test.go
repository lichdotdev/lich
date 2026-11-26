package browser

import (
	"path/filepath"
	"testing"
)

func TestFindLichBrowserNeedsTheBinaryAndADisplay(t *testing.T) {
	bin := fakeBinary(t)
	for _, tc := range []struct {
		name, display, wayland, pinned string
		want                           bool
	}{
		{name: "found on X", display: ":0", pinned: bin, want: true},
		{name: "found on Wayland without X", wayland: "wayland-1", pinned: bin, want: true},
		{name: "no display", pinned: bin},
		{name: "no binary", display: ":0", pinned: filepath.Join(t.TempDir(), "missing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISPLAY", tc.display)
			t.Setenv("WAYLAND_DISPLAY", tc.wayland)
			t.Setenv(BrowserEnv, tc.pinned)
			if got := findLichBrowser(t.TempDir()) != nil; got != tc.want {
				t.Fatalf("found = %v, want %v", got, tc.want)
			}
		})
	}
}
