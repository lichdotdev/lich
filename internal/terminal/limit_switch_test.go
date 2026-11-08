// stubBins is Unix-tagged (terminal_test.go); nothing here is
// platform-specific.
//go:build !windows

package terminal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// limitRig is a Personal session holding conversation c1, in a machine with a
// Work profile, whose quota answers come from the test.
type limitRig struct {
	svc     *Service
	moved   map[string]string
	resumed *[]string
	work    string
}

func newLimitRig(t *testing.T, hub *events.Hub, quotaLeft func(kind, slug string) bool) limitRig {
	t.Helper()
	_, work := profileMachine(t)
	conversationAt(t, filepath.Join(os.Getenv("HOME"), ".claude"), "c1")
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c1", movedProfiles: moved}, nil, hub)
	svc.SetQuotaLeft(quotaLeft)
	resumed := &[]string{}
	svc.SetResumeNow(func(id string) { *resumed = append(*resumed, id) })
	return limitRig{svc: svc, moved: moved, resumed: resumed, work: work}
}

func (r limitRig) inWork() bool {
	_, err := os.Stat(filepath.Join(r.work, "projects", "-work", "c1.jsonl"))
	return err == nil
}

func onlyWork(_, slug string) bool { return slug == "work" }

// TestALimitParksTheResumeOffersTheProfileAndMovesNothing: a limit waits for its
// reset as ever, and a profile with quota left is only offered.
func TestALimitParksTheResumeOffersTheProfileAndMovesNothing(t *testing.T) {
	profileMachine(t)
	writeTranscript(t, "uuid-abc", claudeLimitLine+"\n")
	hub, rec := newProbeHub(t)
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "uuid-abc", movedProfiles: moved}, nil, hub)
	parks := &parkRecorder{}
	svc.SetUsageLimit(parks.park)
	var asked []string
	svc.SetQuotaLeft(func(kind, slug string) bool {
		asked = append(asked, kind+"/"+slug)
		return slug == "work"
	})

	if !svc.noteLimit("s1") {
		t.Fatal("the limit was not read")
	}

	waitFor(t, func() bool { _, ok := rec.payloadOf(api.SessionLimitOfferEvent.Name); return ok }, "the offer")
	payload, _ := rec.payloadOf(api.SessionLimitOfferEvent.Name)
	if fields, _ := payload.(map[string]any); fields["id"] != "s1" || fields["profile"] != "work" {
		t.Errorf("offer = %v, want work for s1", payload)
	}
	if at, ok := parks.of("s1"); !ok || at != 1788653400 {
		t.Errorf("parked %d, %v; want the wait for the reset to run as ever", at, ok)
	}
	if len(asked) != 1 || asked[0] != "claude/work" {
		t.Errorf("asked about %v, want only claude/work: the profile it ran in is not a target", asked)
	}
	if len(moved) != 0 {
		t.Errorf("rows moved without a click: %v", moved)
	}
}

// TestNoProfileWithQuotaOffersNothing: a "no" is also what a profile whose plan
// cannot be read answers (quota.HasQuotaLeft), so unknown quota offers nothing.
func TestNoProfileWithQuotaOffersNothing(t *testing.T) {
	r := newLimitRig(t, events.New(), func(string, string) bool { return false })

	r.svc.offerProfile("s1")

	if _, err := r.svc.ContinueOffLimit(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); !errors.Is(err, errLimitOfferGone) {
		t.Errorf("click with no offer = %v, want it refused", err)
	}
	if len(r.moved) != 0 || r.inWork() || len(*r.resumed) != 0 {
		t.Errorf("moved %v, in work %v, resumed %v; want nothing", r.moved, r.inWork(), *r.resumed)
	}
}

func TestAnOfferMovesNothingUntilItIsClicked(t *testing.T) {
	r := newLimitRig(t, events.New(), onlyWork)

	r.svc.offerProfile("s1")

	if len(r.moved) != 0 || r.inWork() || len(*r.resumed) != 0 {
		t.Errorf("moved %v, in work %v, resumed %v; want nothing before the click", r.moved, r.inWork(), *r.resumed)
	}
}

// TestTheClickMovesTheSessionAndSendsTheContinuationOnce: one click, one move,
// one continuation; a second click on the same offer is refused.
func TestTheClickMovesTheSessionAndSendsTheContinuationOnce(t *testing.T) {
	r := newLimitRig(t, events.New(), onlyWork)
	r.svc.offerProfile("s1")
	opts := api.ContinueInOptions{ID: "s1", Profile: "work"}

	if _, err := r.svc.ContinueOffLimit(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	if r.moved["s1"] != "work" || !r.inWork() {
		t.Errorf("row %q, in work %v; want the session moved", r.moved["s1"], r.inWork())
	}
	if _, err := r.svc.ContinueOffLimit(t.Context(), opts); !errors.Is(err, errLimitOfferGone) {
		t.Errorf("second click = %v, want it refused", err)
	}
	if len(*r.resumed) != 1 || (*r.resumed)[0] != "s1" {
		t.Errorf("resumed %v, want s1 once", *r.resumed)
	}
}

// TestAStaleClickIsRefused: whatever took the session off its limit since the
// toast went up leaves the offer gone, and the click moves nothing.
func TestAStaleClickIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		since func(t *testing.T, svc *Service)
		slug  string
	}{
		{"a turn started", func(_ *testing.T, svc *Service) {
			svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
		}, "work"},
		{"the session closed", func(t *testing.T, svc *Service) {
			if _, err := svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
				t.Fatal(err)
			}
		}, "work"},
		{"the process went away, as a manual Continue in ends it", func(t *testing.T, svc *Service) {
			if err := svc.endProcess("s1", ""); err != nil {
				t.Fatal(err)
			}
		}, "work"},
		{"another profile than the one offered", func(*testing.T, *Service) {}, "elsewhere"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newLimitRig(t, events.New(), onlyWork)
			r.svc.offerProfile("s1")
			tc.since(t, r.svc)

			if _, err := r.svc.ContinueOffLimit(t.Context(), api.ContinueInOptions{ID: "s1", Profile: tc.slug}); !errors.Is(err, errLimitOfferGone) {
				t.Errorf("click = %v, want it refused", err)
			}
			if len(r.moved) != 0 || r.inWork() || len(*r.resumed) != 0 {
				t.Errorf("moved %v, in work %v, resumed %v; want nothing", r.moved, r.inWork(), *r.resumed)
			}
		})
	}
}

// TestALimitSkipsAProfileALimitStoppedBeforeItsReset: the reading is cached for
// minutes, so a profile a limit already stopped a turn in is not offered until
// it resets.
func TestALimitSkipsAProfileALimitStoppedBeforeItsReset(t *testing.T) {
	profileMachine(t)
	conversationAt(t, filepath.Join(os.Getenv("HOME"), ".claude"), "c1")
	svc := New(stubBins{providerSession: "c1", movedProfiles: map[string]string{"other": "work"}}, nil, events.New())
	svc.SetQuotaLeft(func(string, string) bool { return true })
	svc.markSpent("other", time.Now().Add(time.Hour).Unix())

	if to, ok := svc.spareProfile("claude", ""); ok {
		t.Fatalf("offered %q, a profile spent until its reset", to)
	}
	svc.markSpent("other", time.Now().Add(-time.Minute).Unix())
	if to, ok := svc.spareProfile("claude", ""); !ok || to != "work" {
		t.Errorf("after the reset spareProfile = %q, %v, want work", to, ok)
	}
}
