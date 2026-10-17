package main

import (
	"context"
	"testing"
)

func TestFooterIsFoundBySearch(t *testing.T) {
	for _, q := range []string{"footer", "status bar"} {
		if got := searchSettings(q); len(got) == 0 || got[0].title != "Footer" || got[0].section != sectionAppearance {
			t.Errorf("%q: %+v", q, got)
		}
	}
}

// Customize footer… opens Settings on Appearance in Global, the Footer row
// waiting to be lit, and remembers the section as any open does.
func TestRevealOpensTheFooterRow(t *testing.T) {
	f, client := newSidebarBackend(t)
	s := newSettingsScreen(context.Background(), client, nil, func() {})
	s.scope = "pg"
	s.reveal(sectionAppearance, "Footer", openTabs)
	if !s.open || s.section != sectionAppearance || s.scope != "" {
		t.Errorf("open %v section %q scope %q", s.open, s.section, s.scope)
	}
	if s.revealing == nil || *s.revealing != (settingHit{section: sectionAppearance, title: "Footer"}) {
		t.Errorf("revealing %+v", s.revealing)
	}
	waitForSetting(t, f, settingsSectionKey+"@", sectionAppearance)
}
