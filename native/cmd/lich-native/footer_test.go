package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

func slotIDs(slots []footerSlot) []string {
	ids := []string{}
	for _, s := range slots {
		ids = append(ids, s.ID)
	}
	return ids
}

func allItems() []string {
	ids := make([]string, len(footerItems))
	for i, item := range footerItems {
		ids[i] = item.ID
	}
	return ids
}

func busySession() footerInput {
	return footerInput{
		sessionID: "s1", provider: "claude", path: "/home/me/repo", checkout: "/home/me/repo",
		git: lichclient.DiffStats{Branch: "main", Files: 2}, isGit: true,
		pr:    &lichclient.PullRequest{Number: 7, URL: "https://example.com/pr/7"},
		usage: sessionUsage{Percent: 42, Tokens: 1000, Window: 200000, Model: "claude-opus-4-8", CostUSD: cost(0.5)}, hasUsage: true,
		plans:   []lichclient.QuotaPlan{quotaPlan(lichclient.QuotaWindow{Label: "Session", Seconds: 5 * hour, Percent: 28})},
		handsOn: 600, cost: true, now: time.Date(2026, 10, 11, 14, 30, 0, 0, time.UTC),
	}
}

func TestFooterShowsEveryItemOnceTheSessionHasReportedIt(t *testing.T) {
	got := slotIDs(footerSlots(allItems(), busySession()))
	if !reflect.DeepEqual(got, allItems()) {
		t.Errorf("slots %v, want every item %v", got, allItems())
	}
}

func TestFooterWithoutASessionShowsOnlyTheCheckoutAndTheClock(t *testing.T) {
	in := footerInput{path: "/home/me/repo", now: time.Now()}
	got := slotIDs(footerSlots(allItems(), in))
	want := []string{"attach", "files", "checkout", "path", "clock"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("slots %v, want %v", got, want)
	}
	if got := slotIDs(footerSlots(allItems(), footerInput{})); !reflect.DeepEqual(got, []string{"attach", "clock"}) {
		t.Errorf("with no project either: %v", got)
	}
}

func TestFooterDropsReadingsTheSessionHasNot(t *testing.T) {
	cases := map[string]func(*footerInput){
		"changes": func(in *footerInput) { in.isGit = false },
		"pr":      func(in *footerInput) { in.pr = nil },
		"model":   func(in *footerInput) { in.usage.Model = "" },
		"context": func(in *footerInput) { in.usage.Window = 0 },
		"plan":    func(in *footerInput) { in.provider = "codex" },
		"cost":    func(in *footerInput) { in.cost = false },
		"handsOn": func(in *footerInput) { in.handsOn = 59 },
	}
	for id, change := range cases {
		in := busySession()
		change(&in)
		if got := slotIDs(footerSlots([]string{id}, in)); len(got) != 0 {
			t.Errorf("%s shown as %v", id, got)
		}
	}
}

func TestFooterSlotsKeepTheLayoutOrder(t *testing.T) {
	ids := []string{"clock", "pr", "attach"}
	if got := slotIDs(footerSlots(ids, busySession())); !reflect.DeepEqual(got, ids) {
		t.Errorf("slots %v, want %v", got, ids)
	}
}

func TestFooterHandsOnFailureStillShows(t *testing.T) {
	in := busySession()
	in.handsOn, in.handsOnErr = 0, errors.New("down")
	slots := footerSlots([]string{"handsOn"}, in)
	if len(slots) != 1 || slots[0].Readout.Text != "—" {
		t.Errorf("slots %+v, want the dash readout", slots)
	}
}

func TestFooterCheckoutTexts(t *testing.T) {
	in := busySession()
	slots := footerSlots([]string{"checkout", "path"}, in)
	if slots[0].Text != "main" || slots[1].Text != "~/repo" || slots[1].Hint != "/home/me/repo" {
		t.Errorf("slots %+v", slots)
	}
	in.git.Branch, in.host = "", "tmux"
	slots = footerSlots([]string{"checkout", "path"}, in)
	if slots[0].Text != "repo" || slots[1].Text != unknownCwd("tmux") {
		t.Errorf("without a branch, inside tmux: %+v", slots)
	}
}

// footerRig is a footer over fakeSettings, started like connect starts it.
type footerRig struct {
	f        *footer
	settings *fakeSettings
	changed  chan struct{}
}

func newFooterRig(t *testing.T, values map[string]string) *footerRig {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := &footerRig{settings: &fakeSettings{values: values}, changed: make(chan struct{}, 64)}
	invalidate := func() {
		select {
		case r.changed <- struct{}{}:
		default:
		}
	}
	r.f = newFooter(ctx, r.settings.client(t), &ui.Root{}, newFooterUsage(), newPlanQuota(quotaReader(r.settings.client(t)), invalidate), invalidate)
	r.f.toaster = &ui.Toaster{Invalidate: invalidate}
	if err := r.f.start(); err != nil {
		t.Fatal(err)
	}
	return r
}

// settle waits until the layout change in flight is written.
func (r *footerRig) settle(t *testing.T) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if _, saving := r.f.snapshot(); !saving {
			return
		}
		select {
		case <-r.changed:
		case <-deadline:
			t.Fatal("the layout change never finished")
		}
	}
}

func TestEditorDropAppliesAndSavesTheLayout(t *testing.T) {
	r := newFooterRig(t, map[string]string{})
	r.f.drop(ui.SortDrop{ID: "clock", Over: "attach"})
	r.settle(t)
	want := footerLayout{
		Left:  []string{"clock", "attach", "files", "changes", "pr"},
		Right: []string{"checkout", "model", "context", "plan", "handsOn"},
	}
	if l, _ := r.f.snapshot(); !reflect.DeepEqual(l, want) {
		t.Errorf("applied %+v, want %+v", l, want)
	}
	saved, ok := parseFooterLayout(r.settings.get(footerLayoutKey))
	if !ok || !reflect.DeepEqual(saved, want) {
		t.Errorf("saved %q, want %+v", r.settings.get(footerLayoutKey), want)
	}
}

func TestEditorDropOfCostTurnsTheReadoutOn(t *testing.T) {
	r := newFooterRig(t, map[string]string{})
	r.f.drop(ui.SortDrop{ID: "cost", Over: footerRight})
	r.settle(t)
	if !r.f.cost.on() || r.settings.get(costReadoutKey) != "true" {
		t.Errorf("cost on %v, stored %q", r.f.cost.on(), r.settings.get(costReadoutKey))
	}
	r.f.drop(ui.SortDrop{ID: "cost", Over: footerAvailable})
	r.settle(t)
	if r.f.cost.on() || r.settings.get(costReadoutKey) != "false" {
		t.Errorf("after taking it out: cost on %v, stored %q", r.f.cost.on(), r.settings.get(costReadoutKey))
	}
}

func TestEditorKeepsTheLayoutWhenTheCostWriteFails(t *testing.T) {
	r := newFooterRig(t, map[string]string{})
	before, _ := r.f.snapshot()
	r.settings.mu.Lock()
	r.settings.failing = true
	r.settings.mu.Unlock()
	r.f.drop(ui.SortDrop{ID: "cost", Over: footerRight})
	r.settle(t)
	if l, _ := r.f.snapshot(); !reflect.DeepEqual(l, before) || r.f.cost.on() {
		t.Errorf("layout %+v, cost %v after a failed write; want both unchanged", l, r.f.cost.on())
	}
}

func TestEditorRestoreDefault(t *testing.T) {
	r := newFooterRig(t, map[string]string{"/" + footerLayoutKey: `{"left":[],"right":["clock"]}`})
	if l, _ := r.f.snapshot(); !reflect.DeepEqual(l, footerLayout{Left: []string{}, Right: []string{"clock"}}) {
		t.Fatalf("loaded %+v", l)
	}
	r.f.change(defaultFooterLayout())
	r.settle(t)
	if l, _ := r.f.snapshot(); !reflect.DeepEqual(l, defaultFooterLayout()) {
		t.Errorf("restored %+v", l)
	}
}

func TestAvailableItemsAreTheUnplacedInCatalogOrder(t *testing.T) {
	got := availableItems(defaultFooterLayout())
	if want := []string{"path", "cost", "clock"}; !reflect.DeepEqual(got, want) {
		t.Errorf("available %v, want %v", got, want)
	}
}
