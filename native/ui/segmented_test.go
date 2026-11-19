package ui

import "testing"

func TestSegmentsLitRoundsAndKeepsAnyShareVisible(t *testing.T) {
	for _, tc := range []struct{ percent, want int }{{0, 0}, {1, 1}, {2, 1}, {4, 1}, {50, 10}, {57, 11}, {97, 19}, {98, 20}, {100, 20}, {140, 20}, {-5, 0}} {
		if got := SegmentsLit(tc.percent, 20); got != tc.want {
			t.Errorf("%d%% lights %d of 20, want %d", tc.percent, got, tc.want)
		}
	}
}
