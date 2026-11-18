package main

import "testing"

func TestDockTabReadsBackWhatWasStoredAndFallsBackToReview(t *testing.T) {
	for tab, name := range dockTabNames {
		if got := parseDockTab(name); got != tab {
			t.Errorf("%q read back as %v, want %v", name, got, tab)
		}
	}
	for _, raw := range []string{"", "closed", "ext:tracker/board", "terminal"} {
		if got := parseDockTab(raw); got != dockReview {
			t.Errorf("parseDockTab(%q) = %v, want Review", raw, got)
		}
	}
}

func TestDockTabIsStoredOnceItChanges(t *testing.T) {
	f, client := newSidebarBackend(t)
	if tab, err := loadDockTab(t.Context(), client); err != nil || tab != dockReview {
		t.Fatalf("unset reads %v, %v; want Review", tab, err)
	}
	d := new(dock)
	d.restore(dockReview)
	if _, ok := d.tabToSave(); ok {
		t.Error("the tab it opened on was stored again")
	}
	d.show(dockCode)
	tab, ok := d.tabToSave()
	if !ok || tab != dockCode {
		t.Fatalf("a change to Code asked to store %v, %v", tab, ok)
	}
	if err := saveDockTab(t.Context(), client, tab); err != nil {
		t.Fatal(err)
	}
	if got := f.settings[dockTabKey+"@"]; got != "code" {
		t.Errorf("stored %q under %s", got, dockTabKey)
	}
	if _, ok := d.tabToSave(); ok {
		t.Error("an unchanged tab asked to be stored twice")
	}
	d.showExtension(extensionKey{"tracker", "board"})
	if tab, ok := d.tabToSave(); ok {
		t.Errorf("an extension's tab asked to be stored as %v", tab)
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
