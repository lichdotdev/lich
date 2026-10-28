package main

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestButtonLayoutChange(t *testing.T) {
	cases := []struct {
		name   string
		body   []any
		layout string
		ok     bool
	}{
		{"buttons moved left", []any{wmPreferencesNS, buttonLayoutKey, dbus.MakeVariant("close,minimize,maximize:")}, "close,minimize,maximize:", true},
		{"every button removed", []any{wmPreferencesNS, buttonLayoutKey, dbus.MakeVariant("appmenu:")}, "appmenu:", true},
		{"another key", []any{wmPreferencesNS, "theme", dbus.MakeVariant("Adwaita")}, "", false},
		{"another namespace", []any{appearanceNS, buttonLayoutKey, dbus.MakeVariant(":close")}, "", false},
		{"not a string", []any{wmPreferencesNS, buttonLayoutKey, dbus.MakeVariant(uint32(1))}, "", false},
		{"short body", []any{wmPreferencesNS}, "", false},
	}
	for _, c := range cases {
		layout, ok := buttonLayoutChange(c.body)
		if layout != c.layout || ok != c.ok {
			t.Errorf("%s: buttonLayoutChange = %q, %v; want %q, %v", c.name, layout, ok, c.layout, c.ok)
		}
	}
}
