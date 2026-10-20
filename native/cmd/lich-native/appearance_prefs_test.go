package main

import (
	"context"
	"slices"
	"testing"
)

func TestAppearanceClamps(t *testing.T) {
	for in, want := range map[float64]float64{0.1: zoomMin, 1.04: 1, 1.06: 1.1, 5: zoomMax} {
		if got := clampZoom(in); got != want {
			t.Errorf("clampZoom(%v) = %v, want %v", in, got, want)
		}
	}
	for in, want := range map[int]int{2: terminalFontSizeMin, 14: 14, 99: terminalFontSizeMax} {
		if got := clampTerminalFontSize(in); got != want {
			t.Errorf("clampTerminalFontSize(%v) = %v, want %v", in, got, want)
		}
	}
	for in, want := range map[float64]float64{-5: 0, 12.345: 12.35, 40: 40} {
		if got := clampCostBudget(in); got != want {
			t.Errorf("clampCostBudget(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestCloseAnswers(t *testing.T) {
	for stored, want := range map[string]string{"": closeAsk, "ask": closeAsk, "background": closeBackground, "quit": closeQuit, "tray": closeAsk} {
		if got := closeActionOf(stored); got != want {
			t.Errorf("closeActionOf(%q) = %q, want %q", stored, got, want)
		}
	}
	cases := []struct {
		action string
		live   int
		quits  bool
	}{{closeAsk, 0, true}, {closeAsk, 2, false}, {closeQuit, 2, true}}
	for _, c := range cases {
		if got := closeQuits(c.action, c.live); got != c.quits {
			t.Errorf("closeQuits(%q, %d) = %v", c.action, c.live, got)
		}
	}
}

// Nothing stored reads the web's defaults and the window's own font; a
// stored value that is out of range or not a number reads clamped or as
// the default.
func TestLoadAppearancePrefs(t *testing.T) {
	f, client := newSidebarBackend(t)
	p, err := loadAppearancePrefs(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	want := appearancePrefs{zoom: 1, fontSize: defaultTerminalSize, editorFontSize: defaultEditorSize, closeAction: closeAsk}
	if p != want {
		t.Errorf("defaults %+v, want %+v", p, want)
	}
	f.settings[zoomKey+"@"], f.settings[terminalFontSizeKey+"@"] = "3", "x"
	f.settings[costBudgetKey+"@"], f.settings[closeActionKey+"@"] = "25.5", "quit"
	f.settings[terminalFontKey+"@"] = "Iosevka"
	p, _ = loadAppearancePrefs(context.Background(), client)
	want = appearancePrefs{zoom: zoomMax, fontSize: defaultTerminalSize, font: "Iosevka", editorFontSize: defaultEditorSize, budget: 25.5, closeAction: closeQuit}
	if p != want {
		t.Errorf("stored %+v, want %+v", p, want)
	}
}

// Every font is System until one is stored: the system's monospace for the
// terminal and the code, at 14 for the code, and its sans for the interface.
func TestFontsAreSystemUntilChosen(t *testing.T) {
	f, client := newSidebarBackend(t)
	p, err := loadAppearancePrefs(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if p.terminalFamily() != "monospace" || p.codeFamily() != "monospace" || p.uiFont != "" || p.editorFontSize != 14 {
		t.Errorf("unchosen: terminal %q, code %q at %d, interface %q", p.terminalFamily(), p.codeFamily(), p.editorFontSize, p.uiFont)
	}
	f.settings[editorFontKey+"@"], f.settings[editorFontSizeKey+"@"], f.settings[uiFontKey+"@"] = "Fira Code", "99", "Onest"
	p, _ = loadAppearancePrefs(context.Background(), client)
	if p.codeFamily() != "Fira Code" || p.editorFontSize != editorFontSizeMax || p.uiFont != "Onest" || p.terminalFamily() != "monospace" {
		t.Errorf("chosen: code %q at %d, interface %q, terminal %q", p.codeFamily(), p.editorFontSize, p.uiFont, p.terminalFamily())
	}
}

func TestFontOptions(t *testing.T) {
	families := []string{"Fira Code", "Iosevka", "JetBrains Mono"}
	if got := fontOptions("Custom Mono", families, ""); !slices.Equal(got, []string{"", "Custom Mono", "Fira Code", "Iosevka", "JetBrains Mono"}) {
		t.Errorf("unfiltered %v, want System, then the current family", got)
	}
	if got := fontOptions("", families, "sys"); !slices.Equal(got, []string{""}) {
		t.Errorf("System is found by its label: %v", got)
	}
	if got := fontOptions("Iosevka", families, " MONO "); !slices.Equal(got, []string{"JetBrains Mono"}) {
		t.Errorf("filtered %v", got)
	}
}

func TestAppearanceRowsAreFoundBySearch(t *testing.T) {
	for q, title := range map[string]string{"zoom": "Zoom", "text size": "Terminal text size", "typeface": "Terminal font",
		"budget": "Spend ceiling"} {
		if got := searchSettings(q); len(got) == 0 || got[0].title != title || got[0].section != sectionAppearance {
			t.Errorf("%q: %+v, want %q", q, got, title)
		}
	}
	// The Hotkeys rows are searchable as the web's are, so "Quit lich" is a
	// prefix match and outranks the close setting, found by its also words.
	if got := searchSettings("quit"); !slices.ContainsFunc(got, func(h settingHit) bool {
		return h.title == "When the window closes" && h.path() == "General › Window"
	}) {
		t.Errorf("quit: %+v, want the close setting under General › Window", got)
	}
}

// The ceiling colours the cost readout: amber from 80%, red from 95%.
func TestCostSlotColoursAgainstTheCeiling(t *testing.T) {
	in := footerInput{cost: true, hasUsage: true, usage: sessionUsage{CostUSD: cost(8.5)}, budget: 10}
	if s, ok := footerSlotOf("cost", in); !ok || s.Readout.Tone != toneWait {
		t.Errorf("85%% of the ceiling: %+v", s.Readout)
	}
	in.budget = 0
	if s, _ := footerSlotOf("cost", in); s.Readout.Tone != toneMuted {
		t.Errorf("no ceiling: %+v", s.Readout)
	}
}
