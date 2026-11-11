package ui

import (
	"image"
	"testing"
)

func TestFileIconTakesItsSizeWithOrWithoutADevicon(t *testing.T) {
	th := testTheme()
	for _, path := range []string{"main.go", "no-icon.xyz"} {
		if got := layoutAt(th.FileIcon(path, Space(4)).Layout, 0, 100).Size; got != image.Pt(16, 16) {
			t.Errorf("%s: size = %v", path, got)
		}
	}
}
