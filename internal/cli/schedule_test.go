package cli

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

func TestScheduleOptions(t *testing.T) {
	now := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	got, err := scheduleOptions(scheduleRequest{when: "2h", prompt: "x"}, "s1", now)
	if err != nil || got.At != now.Add(2*time.Hour).Unix() || got.From != "s1" || got.On != "" {
		t.Fatalf("when: %+v, %v", got, err)
	}
	got, err = scheduleOptions(scheduleRequest{on: api.WatchReview, prompt: "x"}, "s1", now)
	if err != nil || got.On != api.WatchReview || got.At != 0 {
		t.Fatalf("on: %+v, %v", got, err)
	}
	got, err = scheduleOptions(scheduleRequest{clear: true, prompt: "ignored"}, "s1", now)
	if err != nil || got.At != 0 || got.On != "" || got.Prompt != "" {
		t.Fatalf("clear: %+v, %v", got, err)
	}
	for name, req := range map[string]scheduleRequest{
		"nothing":    {prompt: "x"},
		"both":       {when: "2h", on: api.WatchReview, prompt: "x"},
		"bad time":   {when: "soon", prompt: "x"},
		"clear when": {clear: true, when: "2h"},
	} {
		if _, err := scheduleOptions(req, "s1", now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestScheduledTextStatesTheLimit(t *testing.T) {
	want := "Armed. Your prompt will be typed in this session on review, at most 3 times on one pull request."
	if got := scheduledText(prompt.English, api.ScheduleSelfOptions{On: api.WatchReview}); got != want {
		t.Fatalf("got %q", got)
	}
}

// Nothing the wire tests drive parks a prompt or arms a watch.
func (wiredSessions) SetLichSchedule(string, int64, string) error { return nil }

func (wiredSessions) SetAgentSchedule(string, int64, string) error { return nil }

func (wiredSessions) SetSessionWatch(string, api.SessionWatch) error { return nil }
