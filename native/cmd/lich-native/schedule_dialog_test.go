package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func TestScheduleEchoSaysWhereTheTimeLands(t *testing.T) {
	now := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	for typed, want := range map[string]string{
		"2h30":             "Today at 18:30, in 2h 30m",
		"09:00":            "Tomorrow at 09:00, in 17h",
		"2026-11-12 14:30": "2026-11-12 at 14:30, in 4d",
	} {
		if got, ok := scheduleEcho(typed, now); !ok || got != want {
			t.Errorf("scheduleEcho(%q) = %q, %v, want %q", typed, got, ok, want)
		}
	}
	if got, ok := scheduleEcho("soon", now); ok || !strings.Contains(got, "2h30") {
		t.Errorf("scheduleEcho(soon) = %q, %v, want how to write one", got, ok)
	}
	if got, ok := scheduleEcho("  ", now); !ok || strings.TrimSpace(got) != "" {
		t.Errorf("an empty field reads as a mistake: %q, %v", got, ok)
	}
}

func TestScheduleDialogKeepsWhatThePersonWrote(t *testing.T) {
	var d scheduleDialog
	d.show(sessionView{Session: backend.Session{ID: "s1"}})

	d.pick(api.WatchChecksFailed)
	if got := d.prompt.Text(); got != schedulePromptFor(api.WatchChecksFailed) {
		t.Fatalf("prompt = %q, want the condition's own", got)
	}
	d.pick(api.WatchChecksPassed)
	if got := d.prompt.Text(); got != "" {
		t.Fatalf("prompt = %q, want it gone with the condition it came from", got)
	}
	d.prompt.SetText("merge when it is green")
	d.pick(api.WatchReview)
	if got := d.prompt.Text(); got != "merge when it is green" {
		t.Fatalf("prompt = %q, want what was written kept", got)
	}
}

func TestScheduleDialogOpensOnTheWatchFirst(t *testing.T) {
	var d scheduleDialog
	d.show(sessionView{Session: backend.Session{
		ID: "s1", ScheduledAt: 5000, ScheduledPrompt: "continue", ScheduledBy: api.ScheduledByLich,
		Watch: api.SessionWatch{On: api.WatchReview, Prompt: "address it", By: api.ScheduledByAgent},
	}})
	if d.kind != api.WatchReview || d.prompt.Text() != "address it" || d.by != api.ScheduledByAgent || !d.armed {
		t.Fatalf("dialog = kind %q prompt %q by %q armed %v", d.kind, d.prompt.Text(), d.by, d.armed)
	}
}

func TestScheduleOptionsNeedAPromptAndAReadableTime(t *testing.T) {
	now := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	var d scheduleDialog
	d.show(sessionView{Session: backend.Session{ID: "s1"}})
	d.when.SetText("2h")
	if _, ok := d.sendOptions(now); ok {
		t.Fatal("sendable with no prompt")
	}
	d.prompt.SetText("go")
	opts, ok := d.sendOptions(now)
	if !ok || opts.At != now.Add(2*time.Hour).Unix() || opts.On != "" || opts.SessionID != "s1" {
		t.Fatalf("options = %+v, %v", opts, ok)
	}
	d.when.SetText("soon")
	if _, ok := d.sendOptions(now); ok {
		t.Fatal("sendable with an unreadable time")
	}
	d.pick(api.WatchReview)
	if opts, ok := d.sendOptions(now); !ok || opts.On != api.WatchReview || opts.At != 0 {
		t.Fatalf("watch options = %+v, %v", opts, ok)
	}
}

func TestScheduleMarkPrefersTheWatch(t *testing.T) {
	sv := &sessionView{}
	if scheduleMark(sv) != nil {
		t.Fatal("a session holding nothing earned a mark")
	}
	sv.ScheduledPrompt = "later"
	if scheduleMark(sv) != icSchedule {
		t.Fatal("want the clock for a time")
	}
	sv.Watch.On = api.WatchReview
	if scheduleMark(sv) != icWatch {
		t.Fatal("want the radar for a watch")
	}
}
