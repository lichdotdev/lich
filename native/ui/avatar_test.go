package ui

import (
	"image"
	"testing"
)

func TestAvatarImageScalesOncePerPixelSize(t *testing.T) {
	a := NewAvatarImage(image.NewNRGBA(image.Rect(0, 0, 96, 96)))
	first := a.at(16)
	if a.at(16) != first {
		t.Error("the same size scaled twice")
	}
	a.at(32)
	if len(a.sized) != 2 {
		t.Errorf("sizes cached = %d, want 2", len(a.sized))
	}
}

func TestAvatarDrawsTheInitialOnlyWithoutAnImage(t *testing.T) {
	th := testTheme()
	got := layoutAt(th.Avatar(nil, "M", 24).Layout, 0, 100).Size
	if got != image.Pt(24, 24) {
		t.Errorf("size = %v", got)
	}
}
