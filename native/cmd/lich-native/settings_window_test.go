package main

import "testing"

func TestWindowKindOfOpensTheWebWindowUnlessNativeWasChosen(t *testing.T) {
	for stored, want := range map[string]string{"": "web", "web": "web", "native": "native", "electron": "web"} {
		if got := windowKindOf(stored); got != want {
			t.Errorf("windowKindOf(%q) = %q, want %q", stored, got, want)
		}
	}
}
