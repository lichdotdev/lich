package main

import "testing"

func TestDockTabReadsBackWhatWasStoredAndFallsBackToReview(t *testing.T) {
	for _, c := range []struct {
		tab dockTab
		ext extensionKey
	}{{dockCode, extensionKey{}}, {dockReview, extensionKey{}}, {dockArtifacts, extensionKey{}}, {dockExtension, extensionKey{"tracker", "board"}}} {
		if tab, ext := parseDockTab(formatDockTab(c.tab, c.ext)); tab != c.tab || ext != c.ext {
			t.Errorf("%v %v read back as %v %v", c.tab, c.ext, tab, ext)
		}
	}
	for _, raw := range []string{"", "closed", "ext:", "terminal"} {
		if tab, _ := parseDockTab(raw); tab != dockReview {
			t.Errorf("parseDockTab(%q) = %v, want Review", raw, tab)
		}
	}
}

func TestDockTabIsStoredOnceItChanges(t *testing.T) {
	f, client := newSidebarBackend(t)
	if tab, _, err := loadDockTab(t.Context(), client); err != nil || tab != dockReview {
		t.Fatalf("unset reads %v, %v; want Review", tab, err)
	}
	d := &dock{tab: dockReview, saved: dockReview}
	if _, _, ok := d.tabToSave(); ok {
		t.Error("the tab it opened on was stored again")
	}
	d.show(dockCode)
	tab, ext, ok := d.tabToSave()
	if !ok || tab != dockCode {
		t.Fatalf("a change to Code asked to store %v, %v", tab, ok)
	}
	if err := saveDockTab(t.Context(), client, tab, ext); err != nil {
		t.Fatal(err)
	}
	if got := f.settings[dockTabKey+"@"]; got != "code" {
		t.Errorf("stored %q under %s", got, dockTabKey)
	}
	if _, _, ok := d.tabToSave(); ok {
		t.Error("an unchanged tab asked to be stored twice")
	}
	d.showExtension(extensionKey{"tracker", "board"})
	if tab, ext, ok := d.tabToSave(); !ok || tab != dockExtension || ext.surface != "board" {
		t.Errorf("an extension's tab asked to store %v %v, %v", tab, ext, ok)
	}
}

func TestParseDockWidthClampsAndFallsBack(t *testing.T) {
	cases := map[string]float64{"": 28, "junk": 28, "-3": 28, "NaN": 28, "12": 20, "35.5": 35.5, "90": 60}
	for raw, want := range cases {
		if got := parseDockWidth(raw); got != want {
			t.Errorf("parseDockWidth(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestDockWidthPersistsUnderTheWebsKey(t *testing.T) {
	f, client := newSidebarBackend(t)
	ctx := t.Context()
	if err := saveDockWidth(ctx, client, 33); err != nil {
		t.Fatal(err)
	}
	if got := f.settings[dockWidthKey+"@"]; got != "33" {
		t.Errorf("stored %q under %s", got, dockWidthKey)
	}
	if got, err := loadDockWidth(ctx, client); err != nil || got != 33 {
		t.Errorf("load = %v, %v", got, err)
	}
}
