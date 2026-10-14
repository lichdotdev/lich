package main

import "testing"

func TestDecideProviderSetup(t *testing.T) {
	claude := providerInfo{launchProvider: launchProvider{id: "claude", enabled: true}}
	codex := providerInfo{launchProvider: launchProvider{id: "codex", installed: true}}
	cases := []struct {
		name      string
		m         providersMachine
		want      setupDecision
		preselect string
	}{
		{"detection not answered", providersMachine{}, setupWait, ""},
		{"default already chosen", providersMachine{loaded: true, list: []providerInfo{codex}, globals: map[string]string{providerDefaultKey: "codex"}}, setupSkip, ""},
		{"first installed is preselected", providersMachine{loaded: true, list: []providerInfo{claude, codex}, globals: map[string]string{}}, setupShow, "codex"},
		{"nothing installed", providersMachine{loaded: true, list: []providerInfo{claude}, globals: map[string]string{}}, setupShow, ""},
	}
	for _, c := range cases {
		got, preselect := decideProviderSetup(c.m)
		if got != c.want || preselect != c.preselect {
			t.Errorf("%s: got (%d, %q), want (%d, %q)", c.name, got, preselect, c.want, c.preselect)
		}
	}
}
