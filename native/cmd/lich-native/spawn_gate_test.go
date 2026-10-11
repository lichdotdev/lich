package main

import (
	"context"
	"errors"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
)

var errProbe = errors.New("rpc down")

// fakeProbe answers the gate's backend reads from fields and records writes.
type fakeProbe struct {
	live                         bool
	missing, available           bool
	missingErr, availErr, getErr error
	choice                       string
	set                          map[string]string
}

func (f *fakeProbe) Live(context.Context, string) (bool, error) { return f.live, nil }
func (f *fakeProbe) WorkdirMissing(context.Context, string) (bool, error) {
	return f.missing, f.missingErr
}
func (f *fakeProbe) ResumeAvailable(context.Context, string, string, string) (bool, error) {
	return f.available, f.availErr
}
func (f *fakeProbe) GetSetting(_ context.Context, key, _ string) (string, error) {
	return f.choice, f.getErr
}
func (f *fakeProbe) SetSetting(_ context.Context, key, _, value string) error {
	if f.set == nil {
		f.set = map[string]string{}
	}
	f.set[key] = value
	return nil
}

var restored = lichclient.Session{ID: "s1", Kind: "claude", Label: "Claude 1", ProviderSessionID: "conv-1"}

func TestSpawnDecision(t *testing.T) {
	cases := []struct {
		name    string
		probe   fakeProbe
		session lichclient.Session
		want    spawnVerdict
	}{
		{"missing workdir parks", fakeProbe{missing: true, available: true}, restored, spawnPark},
		{"workdir check error spawns on", fakeProbe{missing: true, missingErr: errProbe, available: true, choice: "resume"}, restored, spawnResume},
		{"no conversation spawns", fakeProbe{available: true}, lichclient.Session{Kind: "claude"}, spawnPlain},
		{"shell never resumes", fakeProbe{available: true}, lichclient.Session{Kind: "shell", ProviderSessionID: "x"}, spawnPlain},
		{"gone conversation spawns fresh", fakeProbe{}, restored, spawnFresh},
		{"resume check error asks", fakeProbe{availErr: errProbe}, restored, spawnAsk},
		{"choice resume", fakeProbe{available: true, choice: "resume"}, restored, spawnResume},
		{"choice fresh", fakeProbe{available: true, choice: "fresh"}, restored, spawnPlain},
		{"choice ask", fakeProbe{available: true, choice: "ask"}, restored, spawnAsk},
		{"unset choice asks", fakeProbe{available: true}, restored, spawnAsk},
		{"choice read error asks", fakeProbe{available: true, choice: "fresh", getErr: errProbe}, restored, spawnAsk},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := spawnDecision(context.Background(), &c.probe, "/repo", c.session); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// askingGate is a gate whose decision for restored raised the prompt.
func askingGate(t *testing.T, probe *fakeProbe) *spawnGate {
	t.Helper()
	g := &spawnGate{probe: probe, invalidate: func() {}, park: func(string) { t.Error("parked") }}
	g.decide(context.Background(), sessionView{Session: restored, path: "/repo"})
	if p := g.prompt(); p == nil || p.ID != restored.ID {
		t.Fatalf("prompt = %v, want %s", p, restored.ID)
	}
	return g
}

func released(t *testing.T, g *spawnGate) string {
	t.Helper()
	resume, ok := g.release(context.Background(), sessionView{Session: restored})
	if !ok {
		t.Fatal("spawn still held")
	}
	return resume
}

func TestAnswerResumeRemembers(t *testing.T) {
	probe := &fakeProbe{available: true}
	g := askingGate(t, probe)
	g.answer(context.Background(), restoreResume, true)
	if got := released(t, g); got != "conv-1" {
		t.Errorf("resume = %q, want conv-1", got)
	}
	if got := probe.set["provider.claude.on-restore"]; got != "resume" {
		t.Errorf("stored %q, want resume", got)
	}
	if g.prompt() != nil {
		t.Error("prompt still up")
	}
}

// A dismiss is Start new without remember (resumeDialog.layout).
func TestDismissStartsNewAndStoresNothing(t *testing.T) {
	probe := &fakeProbe{available: true}
	g := askingGate(t, probe)
	g.answer(context.Background(), restoreFresh, false)
	if got := released(t, g); got != "" {
		t.Errorf("resume = %q, want fresh", got)
	}
	if len(probe.set) != 0 {
		t.Errorf("stored %v on a dismiss", probe.set)
	}
}

func TestStartNewRememberStoresFresh(t *testing.T) {
	probe := &fakeProbe{available: true}
	g := askingGate(t, probe)
	g.answer(context.Background(), restoreFresh, true)
	if got := probe.set["provider.claude.on-restore"]; got != "fresh" {
		t.Errorf("stored %q, want fresh", got)
	}
}

func TestLiveSessionSkipsPrompt(t *testing.T) {
	g := &spawnGate{probe: &fakeProbe{live: true, available: true}, invalidate: func() {}}
	g.decide(context.Background(), sessionView{Session: restored})
	if g.prompt() != nil {
		t.Error("asked about a running session")
	}
	if got := released(t, g); got != "" {
		t.Errorf("resume = %q, want none", got)
	}
}

func TestParkClosesInsteadOfSpawning(t *testing.T) {
	var parked string
	g := &spawnGate{probe: &fakeProbe{missing: true}, invalidate: func() {}, park: func(id string) { parked = id }}
	g.decide(context.Background(), sessionView{Session: restored})
	if parked != restored.ID {
		t.Errorf("parked %q", parked)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.released[restored.ID]; ok {
		t.Error("parked session released to spawn")
	}
}

func TestForgetSendsReturningIDThroughGate(t *testing.T) {
	g := askingGate(t, &fakeProbe{available: true})
	g.answer(context.Background(), restoreFresh, false)
	g.forget(restored.ID)
	g.mu.Lock()
	_, ok := g.released[restored.ID]
	g.mu.Unlock()
	if ok {
		t.Error("forgotten session still released")
	}
}
