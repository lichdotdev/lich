package main

import "testing"

func TestAppsUseLightThemeDark(t *testing.T) {
	for value, want := range map[uint64]bool{0: true, 1: false} {
		if got := appsUseLightThemeDark(value); got != want {
			t.Errorf("appsUseLightThemeDark(%d) = %v; want %v", value, got, want)
		}
	}
}

func TestInterfaceStyleDark(t *testing.T) {
	for out, want := range map[string]bool{"Dark\n": true, "Dark": true, "": false, "Light\n": false} {
		if got := interfaceStyleDark(out); got != want {
			t.Errorf("interfaceStyleDark(%q) = %v; want %v", out, got, want)
		}
	}
}
