package main

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestSchemeChange(t *testing.T) {
	cases := []struct {
		name     string
		body     []any
		dark, ok bool
	}{
		{"dark", []any{appearanceNS, colorSchemeKey, dbus.MakeVariant(uint32(portalPreferDark))}, true, true},
		{"light", []any{appearanceNS, colorSchemeKey, dbus.MakeVariant(uint32(2))}, false, true},
		{"no preference reads light", []any{appearanceNS, colorSchemeKey, dbus.MakeVariant(uint32(portalNoPreference))}, false, true},
		{"another key", []any{appearanceNS, "accent-color", dbus.MakeVariant(uint32(1))}, false, false},
		{"another namespace", []any{"org.gnome.desktop.interface", colorSchemeKey, dbus.MakeVariant(uint32(1))}, false, false},
		{"not a uint", []any{appearanceNS, colorSchemeKey, dbus.MakeVariant("prefer-dark")}, false, false},
		{"short body", []any{appearanceNS}, false, false},
	}
	for _, c := range cases {
		dark, ok := schemeChange(c.body)
		if dark != c.dark || ok != c.ok {
			t.Errorf("%s: schemeChange = %v, %v; want %v, %v", c.name, dark, ok, c.dark, c.ok)
		}
	}
}
