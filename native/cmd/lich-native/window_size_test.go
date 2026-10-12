package main

import "testing"

func TestWindowSizeFallsBackToTheDefault(t *testing.T) {
	cases := []struct {
		spec string
		w, h int
	}{
		{"1100x700", 1100, 700},
		{"", 1400, 900},
		{"wide", 1400, 900},
		{"0x500", 1400, 900},
		{"800x-1", 1400, 900},
	}
	for _, c := range cases {
		w, h := windowSize(c.spec)
		if int(w) != c.w || int(h) != c.h {
			t.Errorf("windowSize(%q) = %vx%v, want %dx%d", c.spec, w, h, c.w, c.h)
		}
	}
}
