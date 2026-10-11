package main

import "testing"

func TestToggledClosesOnTheTabShownAndSwitchesOtherwise(t *testing.T) {
	cases := []struct{ from, to, want dockTab }{
		{dockClosed, dockCode, dockCode},
		{dockCode, dockCode, dockClosed},
		{dockCode, dockReview, dockReview},
		{dockReview, dockReview, dockClosed},
	}
	for _, c := range cases {
		if got := c.from.toggled(c.to); got != c.want {
			t.Errorf("%v.toggled(%v) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestParseDockWidthClampsAndFallsBack(t *testing.T) {
	cases := map[string]float64{"": 28, "junk": 28, "-3": 28, "NaN": 28, "12": 20, "35.5": 35.5, "90": 60}
	for raw, want := range cases {
		if got := parseDockWidth(raw); got != want {
			t.Errorf("parseDockWidth(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestDockWidthPersistsUnderTheWebsKey(t *testing.T) {
	f, client := newSidebarBackend(t)
	ctx := t.Context()
	if err := saveDockWidth(ctx, client, 33); err != nil {
		t.Fatal(err)
	}
	if got := f.settings[dockWidthKey+"@"]; got != "33" {
		t.Errorf("stored %q under %s", got, dockWidthKey)
	}
	if got, err := loadDockWidth(ctx, client); err != nil || got != 33 {
		t.Errorf("load = %v, %v", got, err)
	}
}
