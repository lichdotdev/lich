package main

import (
	"image"
	"testing"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/api"
)

func TestTheSessionViewRowStoresTheChoiceInTheFrameThatTookTheClick(t *testing.T) {
	s, r, f := settingsFramesOn(t, sectionProviders)
	if err := s.loadSessionView(); err != nil {
		t.Fatal(err)
	}
	r.settle(t)
	if !r.shows("New sessions open as") {
		t.Fatalf("the Providers page has no session view row:\n%s", r.drawn())
	}
	if s.sessionView.value != api.ViewChat {
		t.Fatalf("an unset choice reads %q, want the chat", s.sessionView.value)
	}
	r.click(t, paneLabel(t, r, "Terminal"))
	if s.sessionView.value != api.ViewTerminal {
		t.Fatalf("the click left %q, want the terminal", s.sessionView.value)
	}
	waitForSetting(t, f, sessionViewKey+"@", api.ViewTerminal)
}

// paneLabel is at for the last thing drawn as label, the pane's rather than
// the nav's when both show it.
func paneLabel(t *testing.T, r *frameRig, label string) f32.Point {
	t.Helper()
	var glyph image.Point
	found := false
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			glyph, found = n.Desc.Bounds.Min.Add(image.Pt(2, 2)), true
		}
	}
	if !found {
		t.Fatalf("nothing drawn as %q:\n%s", label, r.drawn())
	}
	if b, ok := r.smallestClickAt(glyph); ok {
		return centre(b)
	}
	return f32.Pt(float32(glyph.X), float32(glyph.Y))
}

func TestSearchFindsTheSessionViewOnTheProvidersPage(t *testing.T) {
	if got := searchSettings("open as"); len(got) == 0 || got[0].title != "New sessions open as" || got[0].section != sectionProviders {
		t.Errorf("open as: %+v, want the session view row on Providers", got)
	}
}
