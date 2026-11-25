package ui

import "testing"

// A light theme takes the lighter shadows of every elevation, a dark one the
// darker: the same height reads on either ground.
func TestElevationFollowsTheThemesGround(t *testing.T) {
	dark, light := &Theme{Palette: Dark}, &Theme{Palette: Light}
	for name, e := range map[string]Elevation{"low": ElevationLow, "mid": ElevationMid, "high": ElevationHigh} {
		d, l := e.on(dark), e.on(light)
		if len(d) == 0 || len(l) == 0 || l[0].Alpha >= d[0].Alpha {
			t.Errorf("%s: dark %v, light %v: want the light theme's shadow lighter", name, d, l)
		}
	}
}
