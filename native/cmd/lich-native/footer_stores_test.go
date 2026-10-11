package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

func footerEvent(name, data string) lichclient.Event {
	return lichclient.Event{Name: name, Data: json.RawMessage(data)}
}

const opusReport = `{"id":"s1","percent":42,"tokens":84000,"window":200000,"model":"claude-opus-4-8","effort":"high"}`

func TestFooterUsageKeepsTheLastReportPerSession(t *testing.T) {
	f := newFooterUsage()
	if _, ok := f.get("s1"); ok {
		t.Fatal("a report before any was sent")
	}
	for _, data := range []string{opusReport, `{"id":"s2","percent":5,"tokens":50000,"window":1000000,"model":"m","effort":""}`} {
		if ok, err := f.apply(footerEvent("session-usage", data)); !ok || err != nil {
			t.Fatalf("apply = %v, %v", ok, err)
		}
	}
	u, _ := f.get("s1")
	want := sessionUsage{Percent: 42, Tokens: 84000, Window: 200000, Model: "claude-opus-4-8", Effort: "high"}
	if u != want {
		t.Errorf("s1 = %+v, want %+v", u, want)
	}
	if u, _ := f.get("s2"); u.Window != 1000000 {
		t.Errorf("s2 = %+v", u)
	}
}

func TestFooterUsageClearsWhenTheProviderChangesOrLeaves(t *testing.T) {
	clears := []lichclient.Event{
		footerEvent("session-agent", `{"id":"s1","agent":"codex"}`),
		footerEvent("session-agent", `{"id":"s1","agent":""}`), // a respawn
		footerEvent("session-status", `{"id":"s1","state":"idle"}`),
	}
	for _, ev := range clears {
		f := newFooterUsage()
		_, _ = f.apply(footerEvent("session-usage", opusReport))
		if ok, err := f.apply(ev); !ok || err != nil {
			t.Fatalf("%s: apply = %v, %v", ev.Data, ok, err)
		}
		if _, ok := f.get("s1"); ok {
			t.Errorf("%s left the report standing", ev.Data)
		}
	}
	f := newFooterUsage()
	_, _ = f.apply(footerEvent("session-usage", opusReport))
	_, _ = f.apply(footerEvent("session-status", `{"id":"s1","state":"busy"}`))
	_, _ = f.apply(footerEvent("session-agent", `{"id":"s2","agent":"codex"}`))
	if _, ok := f.get("s1"); !ok {
		t.Error("a busy status or another session's agent cleared s1")
	}
}

func TestFooterUsageCost(t *testing.T) {
	base := `"id":"s1","percent":10,"tokens":1,"window":2,"model":"m","effort":""`
	cases := []struct {
		extra    string
		wantCost *float64
		wantMiss string
	}{
		{`,"costUsd":1.25`, cost(1.25), ""},
		{``, nil, ""},
		{`,"costUsd":"1.25"`, nil, ""}, // unusable reads as absent
		{`,"costMiss":"mixed-models"`, nil, "mixed-models"},
		{`,"costMiss":"from-the-future"`, nil, ""}, // a reason this build cannot name
	}
	for _, c := range cases {
		f := newFooterUsage()
		if _, err := f.apply(footerEvent("session-usage", "{"+base+c.extra+"}")); err != nil {
			t.Fatal(err)
		}
		u, _ := f.get("s1")
		costOK := (u.CostUSD == nil) == (c.wantCost == nil) && (u.CostUSD == nil || *u.CostUSD == *c.wantCost)
		if !costOK || u.CostMiss != c.wantMiss {
			t.Errorf("%s: cost %v, miss %q", c.extra, u.CostUSD, u.CostMiss)
		}
	}
}

func TestFooterUsageIgnoresIncompleteReportsAndOtherEvents(t *testing.T) {
	f := newFooterUsage()
	if ok, _ := f.apply(footerEvent("session-usage", `{"id":"s1","percent":10}`)); !ok {
		t.Error("session-usage not taken")
	}
	if _, ok := f.get("s1"); ok {
		t.Error("an incomplete report was stored")
	}
	if ok, _ := f.apply(footerEvent("session-title", `{"id":"s1"}`)); ok {
		t.Error("session-title claimed")
	}
}

// quotaRig drives planQuota with a hand-turned clock and fetch.
type quotaRig struct {
	q       *planQuota
	tick    chan time.Time
	changed chan struct{}
	mu      sync.Mutex
	reads   int
	reply   []lichclient.QuotaPlan
	fail    error
}

func newQuotaRig() *quotaRig {
	r := &quotaRig{tick: make(chan time.Time), changed: make(chan struct{}, 8)}
	r.q = newPlanQuota(func(string) ([]lichclient.QuotaPlan, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.reads++
		return r.reply, r.fail
	}, func() { r.changed <- struct{}{} })
	r.q.after = func(d time.Duration) <-chan time.Time {
		if d != planQuotaRefresh {
			panic("unexpected cadence")
		}
		return r.tick
	}
	return r
}

func (r *quotaRig) set(plans []lichclient.QuotaPlan, err error) {
	r.mu.Lock()
	r.reply, r.fail = plans, err
	r.mu.Unlock()
}

func (r *quotaRig) readCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

func waitChanged(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no change published")
	}
}

func TestPlanQuotaReadsOnFirstWatchAndShares(t *testing.T) {
	r := newQuotaRig()
	r.set([]lichclient.QuotaPlan{quotaPlan(lichclient.QuotaWindow{Label: "Session", Percent: 10})}, nil)
	off1 := r.q.watch("s1")
	off2 := r.q.watch("s1")
	waitChanged(t, r.changed)
	if got := r.q.plans("s1"); len(got) != 1 || got[0].Windows[0].Percent != 10 {
		t.Errorf("plans = %+v", got)
	}
	if r.readCount() != 1 {
		t.Errorf("two watchers read %d times, want one shared read", r.readCount())
	}
	if r.q.plans("other") != nil {
		t.Error("another key shares s1's reading")
	}
	off1()
	off2()
}

func TestPlanQuotaPublishesOnlyAChangedReading(t *testing.T) {
	r := newQuotaRig()
	reading := []lichclient.QuotaPlan{quotaPlan(lichclient.QuotaWindow{Label: "Session", Percent: 10})}
	r.set(reading, nil)
	defer r.q.watch("")()
	waitChanged(t, r.changed)

	// A tick is taken only once the poll is back waiting, so the second one
	// proves the first's read of the same numbers has finished.
	r.tick <- time.Time{}
	r.tick <- time.Time{}
	if len(r.changed) != 0 {
		t.Error("an unchanged reading was published")
	}
	r.set([]lichclient.QuotaPlan{quotaPlan(lichclient.QuotaWindow{Label: "Session", Percent: 11})}, nil)
	r.tick <- time.Time{}
	waitChanged(t, r.changed)
	if got := r.q.plans(""); got[0].Windows[0].Percent != 11 {
		t.Errorf("plans = %+v", got)
	}
}

func TestPlanQuotaKeepsTheLastReadingWhenAReadFails(t *testing.T) {
	r := newQuotaRig()
	r.set([]lichclient.QuotaPlan{quotaPlan(lichclient.QuotaWindow{Label: "Session", Percent: 10})}, nil)
	defer r.q.watch("s1")()
	waitChanged(t, r.changed)
	r.set(nil, errors.New("down"))
	r.tick <- time.Time{}
	r.tick <- time.Time{} // the failed read has finished once the next wait is taken
	if got := r.q.plans("s1"); len(got) != 1 {
		t.Errorf("a failed read blanked the reading: %+v", got)
	}
}

func TestPlanQuotaStopsPollingWithNoWatcherAndKeepsTheReading(t *testing.T) {
	r := newQuotaRig()
	r.set([]lichclient.QuotaPlan{quotaPlan()}, nil)
	off := r.q.watch("s1")
	waitChanged(t, r.changed)
	off()
	off() // a second call is a no-op
	reads := r.readCount()
	for range 2 {
		select {
		case r.tick <- time.Time{}:
		case <-time.After(50 * time.Millisecond):
		}
	}
	if r.readCount() != reads {
		t.Errorf("read %d more times after the last watcher left", r.readCount()-reads)
	}
	if r.q.plans("s1") == nil {
		t.Error("the reading went with its watchers")
	}
}

// handsOnRig drives handsOn with a fetch the test answers.
type handsOnRig struct {
	h       *handsOn
	changed chan struct{}
	mu      sync.Mutex
	answer  map[string]int64
	fail    error
	// gate, when set, holds every read until the test closes it.
	gate chan struct{}
}

func newHandsOnRig() *handsOnRig {
	r := &handsOnRig{changed: make(chan struct{}, 8), answer: map[string]int64{}}
	r.h = newHandsOn(func(id string) (int64, error) {
		r.mu.Lock()
		gate := r.gate
		r.mu.Unlock()
		if gate != nil {
			<-gate
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.answer[id], r.fail
	}, func() { r.changed <- struct{}{} })
	return r
}

func TestHandsOnFollowsTheSessionAndBlanksOnSwitch(t *testing.T) {
	r := newHandsOnRig()
	r.answer["s1"], r.answer["s2"] = 3900, 120
	r.h.follow("s1")
	waitChanged(t, r.changed) // blanked
	waitChanged(t, r.changed) // read
	if s, err := r.h.read(); s != 3900 || err != nil {
		t.Errorf("s1 = %d, %v", s, err)
	}
	gate := make(chan struct{})
	r.mu.Lock()
	r.gate = gate
	r.mu.Unlock()
	r.h.follow("s2")
	waitChanged(t, r.changed)
	if s, _ := r.h.read(); s != 0 {
		t.Errorf("s2 shows s1's figure while its own is read: %d", s)
	}
	close(gate)
	waitChanged(t, r.changed)
	if s, _ := r.h.read(); s != 120 {
		t.Errorf("s2 = %d", s)
	}
}

func TestHandsOnRereadsOnTheMinuteAndReportsFailure(t *testing.T) {
	r := newHandsOnRig()
	tick := make(chan time.Time)
	at := time.Date(2026, 10, 11, 14, 30, 45, 0, time.UTC)
	r.h.now = func() time.Time { return at }
	r.h.after = func(d time.Duration) <-chan time.Time {
		if d != 15*time.Second {
			panic("not aligned to the minute")
		}
		return tick
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.h.run(ctx)
	r.answer["s1"] = 60
	r.h.follow("s1")
	waitChanged(t, r.changed)
	waitChanged(t, r.changed)

	r.mu.Lock()
	r.fail = errors.New("down")
	r.mu.Unlock()
	tick <- at
	waitChanged(t, r.changed)
	if s, err := r.h.read(); s != 0 || err == nil {
		t.Errorf("a failed read = %d, %v; want 0 and the error", s, err)
	}
}

func TestCostGateLoadsOffUntilTheStoredValueSaysOn(t *testing.T) {
	cases := map[string]bool{"true": true, "false": false, "": false}
	for stored, want := range cases {
		f := &fakeSettings{values: map[string]string{"/" + costReadoutKey: stored}}
		c := f.client(t)
		g := &costGate{get: c.GetSetting, set: c.SetSetting, onChange: func() {}}
		if g.on() || g.isReady() {
			t.Fatal("on or ready before loading")
		}
		if err := g.load(context.Background()); err != nil {
			t.Fatal(err)
		}
		if g.on() != want || !g.isReady() {
			t.Errorf("stored %q: on %v, ready %v", stored, g.on(), g.isReady())
		}
	}
}

func TestCostGateFailedLoadStaysOffButReady(t *testing.T) {
	f := &fakeSettings{values: map[string]string{}, failing: true}
	c := f.client(t)
	g := &costGate{get: c.GetSetting, set: c.SetSetting, onChange: func() {}}
	if err := g.load(context.Background()); err == nil {
		t.Error("a failed load reported no error")
	}
	if g.on() || !g.isReady() {
		t.Errorf("on %v, ready %v", g.on(), g.isReady())
	}
}

func TestCostGateWritesThroughAndRollsBackAFailedWrite(t *testing.T) {
	f := &fakeSettings{values: map[string]string{}}
	c := f.client(t)
	changes := 0
	g := &costGate{get: c.GetSetting, set: c.SetSetting, onChange: func() { changes++ }}
	if err := g.setOn(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if !g.on() || f.get(costReadoutKey) != "true" || changes != 1 {
		t.Errorf("on %v, stored %q, changes %d", g.on(), f.get(costReadoutKey), changes)
	}
	f.mu.Lock()
	f.failing = true
	f.mu.Unlock()
	if err := g.setOn(context.Background(), false); err == nil {
		t.Error("a failed write reported no error")
	}
	if !g.on() || changes != 3 {
		t.Errorf("after a failed write: on %v, changes %d; want the old value back", g.on(), changes)
	}
}

func TestCostGateToggleDuringLoadWins(t *testing.T) {
	f := &fakeSettings{values: map[string]string{"/" + costReadoutKey: "false"}}
	c := f.client(t)
	g := &costGate{set: c.SetSetting, onChange: func() {}}
	g.get = func(ctx context.Context, key, scope string) (string, error) {
		if err := g.setOn(ctx, true); err != nil {
			return "", err
		}
		return "false", nil
	}
	if err := g.load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !g.on() {
		t.Error("a load that started before the toggle overwrote it")
	}
}
