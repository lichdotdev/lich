package main

import (
	"testing"

	"github.com/omartelo/lich/native/ui"
)

func TestDiffStatColorsFollowThePalette(t *testing.T) {
	cases := []struct {
		name           string
		palette        ui.Palette
		added, deleted [3]float64
	}{
		{"light: emerald-600 / red-600", ui.Light, [3]float64{0.596, 0.145, 163.225}, [3]float64{0.577, 0.245, 27.325}},
		{"dark: emerald-400 / red-400", ui.Dark, [3]float64{0.765, 0.177, 163.223}, [3]float64{0.704, 0.191, 22.216}},
	}
	for _, tc := range cases {
		added, deleted := diffStatColors(&ui.Theme{Palette: tc.palette})
		wantAdded := ui.OKLCH(tc.added[0], tc.added[1], tc.added[2], 1)
		wantDeleted := ui.OKLCH(tc.deleted[0], tc.deleted[1], tc.deleted[2], 1)
		if added != wantAdded || deleted != wantDeleted {
			t.Errorf("%s: got %v / %v, want %v / %v", tc.name, added, deleted, wantAdded, wantDeleted)
		}
	}
}
