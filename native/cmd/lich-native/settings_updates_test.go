package main

import (
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

func TestSplitLeadInTakesTheBoldSentence(t *testing.T) {
	for _, tc := range []struct{ in, headline, body string }{
		{"**Faster start.** It opens in half the time.", "Faster start.", "It opens in half the time."},
		{"No bold at all", "No bold at all", ""},
		{"First sentence. The rest. Of it.", "First sentence.", "The rest. Of it."},
		{"See `lich-native.log` for it. Then retry.", "See `lich-native.log` for it.", "Then retry."},
		{"Reads `a. b` whole", "Reads `a. b` whole", ""},
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
	first := backend.PatchNotesHighlight{Kind: "important", Text: "one"}
	note := backend.PatchNotesHighlight{Kind: "note", Text: "n"}
	second := backend.PatchNotesHighlight{Kind: "important", Text: "two"}
	lead, callouts := layoutHighlights([]backend.PatchNotesHighlight{note, first, second})
	if lead == nil || lead.Text != "one" {
		t.Fatalf("lead = %v, want the first important", lead)
	}
	if want := []backend.PatchNotesHighlight{note, second}; !reflect.DeepEqual(callouts, want) {
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

// The row says the release sessions run, the first provider that has it
// speaking for all, and that there is none when no provider has it.
func TestPluginVersionText(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []backend.PluginStatus
		want     string
	}{
		{"none read", nil, "Plugin not installed"},
		{"none installed", []backend.PluginStatus{{Provider: "claude"}, {Provider: "codex", Available: true}}, "Plugin not installed"},
		{"the first installed", []backend.PluginStatus{{Provider: "claude"}, {Provider: "codex", Installed: true, InstalledVersion: "0.23.0"}, {Provider: "kiro", Installed: true, InstalledVersion: "0.22.0"}}, "v0.23.0"},
	} {
		if got := pluginVersionText(tc.statuses); got != tc.want {
			t.Errorf("%s: pluginVersionText = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCalloutToneDrawsCautionAndWarningInTheDangerColor(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	for kind, wantIcon := range map[string]*icons.Icon{"caution": icNotesCaution, "warning": icNotesWarning} {
		icon, ink, danger := calloutTone(th, kind)
		if icon != wantIcon || ink != th.Destructive || !danger {
			t.Errorf("%s: icon %p ink %v danger %v, want its own glyph in Destructive", kind, icon, ink, danger)
		}
	}
	if icon, ink, danger := calloutTone(th, "note"); icon != icNotesNote || ink != th.MutedForeground || danger {
		t.Errorf("note: icon %p ink %v danger %v, want the muted note", icon, ink, danger)
	}
}

// A caution block is a callout, never the headline: important keeps that.
func TestLayoutHighlightsKeepsCautionAsACallout(t *testing.T) {
	caution := backend.PatchNotesHighlight{Kind: "caution", Text: "c"}
	lead, callouts := layoutHighlights([]backend.PatchNotesHighlight{caution})
	if lead != nil || !reflect.DeepEqual(callouts, []backend.PatchNotesHighlight{caution}) {
		t.Fatalf("lead %v callouts %v, want caution as the one callout", lead, callouts)
	}
}

// Settings › Updates names the plugin's release and offers nothing to press:
// lich sets the plugin up itself.
func TestUpdatesShowsThePluginReleaseReadOnly(t *testing.T) {
	_, r := settingsFrames(t, sectionUpdates)
	r.waitFor(t, "v0.23.0")
	if !r.showsAll(i18n.T("settings.pluginSetting.title"), "v0.23.0") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	for _, gone := range []string{"Install", "Update all", "Update to v0.23.0", "Re-register hooks"} {
		if r.shows(gone) {
			t.Errorf("Updates still offers %q", gone)
		}
	}
	writeShot(t, r, "settings-updates.png")
}
