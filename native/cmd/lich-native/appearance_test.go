package main

import "testing"

func TestStoredThemeMapsOnlyLightAndDark(t *testing.T) {
	for stored, light := range map[string]bool{"light": true, "dark": false, "system": false, "": false, "rose-pine": false} {
		if got := storedLight(stored); got != light {
			t.Errorf("storedLight(%q) = %v, want %v", stored, got, light)
		}
	}
	if storedTheme(true) != "light" || storedTheme(false) != "dark" {
		t.Errorf("toggle writes %q / %q, want light / dark", storedTheme(true), storedTheme(false))
	}
}
