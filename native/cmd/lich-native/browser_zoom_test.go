package main

import "testing"

func TestZoomChordSpellsWhatPageZoomChordReads(t *testing.T) {
	cases := []struct {
		key   string
		isMac bool
		want  string
	}{
		{"-", false, "Ctrl+-"},
		{"=", false, "Ctrl+="},
		{"0", false, "Ctrl+0"},
		{"0", true, "⌘0"},
	}
	for _, c := range cases {
		if got := zoomChord(c.key, c.isMac); got != c.want {
			t.Errorf("zoomChord(%q, %v) = %q, want %q", c.key, c.isMac, got, c.want)
		}
	}
}
