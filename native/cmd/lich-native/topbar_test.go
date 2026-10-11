package main

import "testing"

func TestTabWidthShrinksEvenlyToItsFloor(t *testing.T) {
	cases := []struct{ room, count, want int }{
		{1000, 3, 196}, // room to spare: the fixed width
		{800, 5, 160},  // shared evenly
		{800, 8, 118},  // the floor; the strip scrolls past it
		{800, 0, 196},
	}
	for _, c := range cases {
		if got := tabWidth(c.room, c.count, 196, 118); got != c.want {
			t.Errorf("tabWidth(%d, %d) = %d, want %d", c.room, c.count, got, c.want)
		}
	}
}

func TestBrandColumnFollowsTheSidebar(t *testing.T) {
	const rail, panel = 32, 240
	cases := []struct {
		name           string
		open, morphing bool
		p              float32
		width          int
		alpha          float32
	}{
		{"open", true, false, 0, panel, 1},
		{"collapsed", false, false, 0, rail, 0},
		{"expanding, a quarter in", true, true, 0.25, 84, 0.25},
		{"collapsing, a quarter in", false, true, 0.25, 188, 0.75},
		{"collapsing, landed", false, true, 1, rail, 0},
	}
	for _, c := range cases {
		w, a := brandColumn(rail, panel, c.open, c.morphing, c.p)
		if w != c.width || a != c.alpha {
			t.Errorf("%s: width %d alpha %v, want %d %v", c.name, w, a, c.width, c.alpha)
		}
	}
}
