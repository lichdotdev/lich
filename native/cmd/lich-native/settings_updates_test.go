package main

import (
	"reflect"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

func TestSplitLeadInTakesTheBoldSentence(t *testing.T) {
	for _, tc := range []struct{ in, headline, body string }{
		{"**Faster start.** It opens in half the time.", "Faster start.", "It opens in half the time."},
		{"No bold at all", "No bold at all", ""},
		{"**Unclosed lead-in", "**Unclosed lead-in", ""},
		{"**Only a lead-in**", "Only a lead-in", ""},
	} {
		headline, body := splitLeadIn(tc.in)
		if headline != tc.headline || body != tc.body {
			t.Errorf("splitLeadIn(%q) = %q, %q; want %q, %q", tc.in, headline, body, tc.headline, tc.body)
		}
	}
}

func TestLayoutHighlightsDemotesASecondImportant(t *testing.T) {
	first := lichclient.PatchNotesHighlight{Kind: "important", Text: "one"}
	note := lichclient.PatchNotesHighlight{Kind: "note", Text: "n"}
	second := lichclient.PatchNotesHighlight{Kind: "important", Text: "two"}
	lead, callouts := layoutHighlights([]lichclient.PatchNotesHighlight{note, first, second})
	if lead == nil || lead.Text != "one" {
		t.Fatalf("lead = %v, want the first important", lead)
	}
	if want := []lichclient.PatchNotesHighlight{note, second}; !reflect.DeepEqual(callouts, want) {
		t.Fatalf("callouts = %v, want %v", callouts, want)
	}
	if lead, callouts := layoutHighlights(nil); lead != nil || callouts != nil {
		t.Fatalf("no highlights gave %v, %v", lead, callouts)
	}
}

func TestInlineWordsKeepsPunctuationWithItsCodeSpan(t *testing.T) {
	got := inlineWords("Run `lich rage`, then **retry** it.")
	want := [][]inlineRun{
		{{"Run", runPlain}},
		{{"lich", runCode}},
		{{"rage", runCode}, {",", runPlain}},
		{{"then", runPlain}},
		{{"retry", runBold}},
		{{"it.", runPlain}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inlineWords = %v, want %v", got, want)
	}
}

func TestPluginRowsSayWhatTheMachineHas(t *testing.T) {
	base := lichclient.PluginStatus{Provider: "claude", Name: "Claude Code", Available: true, Compatible: true, LatestVersion: "0.9.0"}
	installed := base
	installed.Installed, installed.InstalledVersion = true, "0.9.0"
	outdated := installed
	outdated.InstalledVersion, outdated.UpdateAvailable = "0.8.0", true
	unsupported := outdated
	unsupported.InstalledVersion, unsupported.Compatible = "0.14.0", false
	missing := lichclient.PluginStatus{Provider: "codex"}

	for _, tc := range []struct {
		name    string
		st      lichclient.PluginStatus
		text    string
		action  string
		variant ui.Variant
	}{
		{"cli missing", missing, "CLI not installed", "", ui.VariantDefault},
		{"not installed", base, "Plugin not installed", "Install", ui.VariantDefault},
		{"installed", installed, "v0.9.0", "", ui.VariantDefault},
		{"outdated", outdated, "v0.8.0", "Update to v0.9.0", ui.VariantOutline},
		{"unsupported", unsupported, "v0.14.0, not supported by this lich", "Install v0.9.0", ui.VariantOutline},
	} {
		if got := pluginVersionText(tc.st); got != tc.text {
			t.Errorf("%s: text = %q, want %q", tc.name, got, tc.text)
		}
		label, variant, ok := pluginAction(tc.st)
		if label != tc.action || ok != (tc.action != "") || (ok && variant != tc.variant) {
			t.Errorf("%s: action = %q %v %v, want %q %v", tc.name, label, variant, ok, tc.action, tc.variant)
		}
	}
}

func TestOutdatedPluginsSkipsWhatIsNotInstalled(t *testing.T) {
	statuses := []lichclient.PluginStatus{
		{Provider: "claude", Installed: true, UpdateAvailable: true},
		{Provider: "codex", Installed: true, UpdateAvailable: true, Compatible: false},
		{Provider: "cursor", Installed: true},
		{Provider: "crush", UpdateAvailable: true},
	}
	var got []string
	for _, st := range outdatedPlugins(statuses) {
		got = append(got, st.Provider)
	}
	if want := []string{"claude", "codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("outdated = %v, want %v", got, want)
	}
}

func TestPluginOutcomeReadsTheLastCheck(t *testing.T) {
	for _, tc := range []struct {
		checked, failed bool
		want            string
	}{{false, true, ""}, {true, false, "Checked."}, {true, true, "Check failed — are you online?"}} {
		if got := pluginOutcome(tc.checked, tc.failed); got != tc.want {
			t.Errorf("pluginOutcome(%v, %v) = %q, want %q", tc.checked, tc.failed, got, tc.want)
		}
	}
}
