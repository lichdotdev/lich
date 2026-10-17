package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
)

func TestDecidePluginAction(t *testing.T) {
	missing := lichclient.PluginStatus{Provider: "codex", Available: true}
	outdated := lichclient.PluginStatus{Provider: "claude", Available: true, Installed: true, Compatible: true, UpdateAvailable: true, LatestVersion: "2.0.0"}
	incompatible := lichclient.PluginStatus{Provider: "omp", Available: true, Installed: true, LatestVersion: "2.0.0"}
	cursor := lichclient.PluginStatus{Provider: "cursor", Available: true, Installed: true}
	noCLI := lichclient.PluginStatus{Provider: "crush"}
	cases := []struct {
		name            string
		statuses        []lichclient.PluginStatus
		installDismiss  bool
		updateDismissed string
		want            pluginActionKind
		version         string
	}{
		{"incompatible wins over dismissals", []lichclient.PluginStatus{missing, outdated, incompatible}, true, "2.0.0", pluginActionIncompatible, "2.0.0"},
		{"cursor never counts as incompatible", []lichclient.PluginStatus{cursor}, false, "", pluginActionNone, ""},
		{"missing comes before an update", []lichclient.PluginStatus{missing, outdated}, false, "", pluginActionInstall, ""},
		{"install dismissed falls to the update", []lichclient.PluginStatus{missing, outdated}, true, "", pluginActionUpdate, "2.0.0"},
		{"update dismissed for its version", []lichclient.PluginStatus{outdated}, false, "2.0.0", pluginActionNone, ""},
		{"update dismissed for an older version", []lichclient.PluginStatus{outdated}, false, "1.9.0", pluginActionUpdate, "2.0.0"},
		{"a CLI not on the machine is never offered", []lichclient.PluginStatus{noCLI}, false, "", pluginActionNone, ""},
	}
	for _, c := range cases {
		got := decidePluginAction(c.statuses, c.installDismiss, c.updateDismissed)
		if got.kind != c.want || got.version != c.version {
			t.Errorf("%s: got (%d, %q), want (%d, %q)", c.name, got.kind, got.version, c.want, c.version)
		}
	}
}
