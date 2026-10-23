package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestDecidePluginAction(t *testing.T) {
	missing := backend.PluginStatus{Provider: "codex", Available: true}
	outdated := backend.PluginStatus{Provider: "claude", Available: true, Installed: true, Compatible: true, UpdateAvailable: true, LatestVersion: "2.0.0"}
	incompatible := backend.PluginStatus{Provider: "omp", Available: true, Installed: true, LatestVersion: "2.0.0"}
	cursor := backend.PluginStatus{Provider: "cursor", Available: true, Installed: true}
	noCLI := backend.PluginStatus{Provider: "crush"}
	cases := []struct {
		name            string
		statuses        []backend.PluginStatus
		installDismiss  bool
		updateDismissed string
		want            pluginActionKind
		version         string
	}{
		{"incompatible wins over dismissals", []backend.PluginStatus{missing, outdated, incompatible}, true, "2.0.0", pluginActionIncompatible, "2.0.0"},
		{"cursor never counts as incompatible", []backend.PluginStatus{cursor}, false, "", pluginActionNone, ""},
		{"missing comes before an update", []backend.PluginStatus{missing, outdated}, false, "", pluginActionInstall, ""},
		{"install dismissed falls to the update", []backend.PluginStatus{missing, outdated}, true, "", pluginActionUpdate, "2.0.0"},
		{"update dismissed for its version", []backend.PluginStatus{outdated}, false, "2.0.0", pluginActionNone, ""},
		{"update dismissed for an older version", []backend.PluginStatus{outdated}, false, "1.9.0", pluginActionUpdate, "2.0.0"},
		{"a CLI not on the machine is never offered", []backend.PluginStatus{noCLI}, false, "", pluginActionNone, ""},
	}
	for _, c := range cases {
		got := decidePluginAction(c.statuses, c.installDismiss, c.updateDismissed)
		if got.kind != c.want || got.version != c.version {
			t.Errorf("%s: got (%d, %q), want (%d, %q)", c.name, got.kind, got.version, c.want, c.version)
		}
	}
}
