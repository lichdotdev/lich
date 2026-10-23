package main

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// attentionRig is a model over two open projects whose status reports reach
// the attention toast, with the toasts and Opens recorded.
type attentionRig struct {
	m      *model
	toasts []ui.Toast
	opened [][2]string
}

func newAttentionRig() *attentionRig {
	r := &attentionRig{m: newModel(nil, func() {})}
	r.m.projects = []backend.Project{
		{ID: "p", Name: "lich", Sessions: []backend.Session{{ID: "a", Label: "fix the build"}, {ID: "b", Label: "Shell 2"}}},
		{ID: "q", Name: "skipo", Sessions: []backend.Session{{ID: "c", Label: "Claude 1"}}},
	}
	r.m.activeProject, r.m.active = "p", "a"
	a := attentionToast{show: func(t ui.Toast) { r.toasts = append(r.toasts, t) },
		open: func(projectID, id string) { r.opened = append(r.opened, [2]string{projectID, id}) }}
	r.m.notify = a.report
	return r
}

func (r *attentionRig) report(t *testing.T, ev statusEvent) {
	t.Helper()
	if err := r.m.applyStatus(context.Background(), statusEv(t, ev)); err != nil {
		t.Fatal(err)
	}
}

func TestAttentionToastSkipsTheSessionOnScreen(t *testing.T) {
	r := newAttentionRig()
	r.report(t, statusEvent{ID: "a", State: statusWaiting})
	if len(r.toasts) != 0 {
		t.Fatalf("toasted %v for the session whose terminal shows the prompt", r.toasts)
	}
}

func TestAttentionToastRaisesForEveryWaitingReportOffScreen(t *testing.T) {
	r := newAttentionRig()
	r.report(t, statusEvent{ID: "b", State: statusBusy})
	r.report(t, statusEvent{ID: "b", State: statusWaiting})
	r.report(t, statusEvent{ID: "b", State: statusWaiting})
	r.report(t, statusEvent{ID: "c", State: statusWaiting})
	r.report(t, statusEvent{ID: "gone", State: statusWaiting})
	if len(r.toasts) != 3 {
		t.Fatalf("raised %d toasts, want one per waiting report on b and c, none for a session nowhere open", len(r.toasts))
	}
	if r.toasts[0].Title != "Shell 2 needs your input" || r.toasts[2].Title != "Claude 1 needs your input" {
		t.Fatalf("titles %q, %q", r.toasts[0].Title, r.toasts[2].Title)
	}
}

func TestAttentionToastOnHomeRaisesEvenForTheProjectsActiveSession(t *testing.T) {
	r := newAttentionRig()
	r.m.showHome()
	r.report(t, statusEvent{ID: "a", State: statusWaiting})
	if len(r.toasts) != 1 {
		t.Fatalf("raised %d toasts with Home on screen, want 1", len(r.toasts))
	}
}

func TestAttentionToastSaysWhyAndWhereAndOpensTheSession(t *testing.T) {
	r := newAttentionRig()
	r.report(t, statusEvent{ID: "c", State: statusWaiting, Reason: "Approve Bash: rm -rf build"})
	r.report(t, statusEvent{ID: "b", State: statusWaiting})

	withReason, without := r.toasts[0], r.toasts[1]
	if len(withReason.Lines) != 2 || withReason.Lines[0].Text != "Approve Bash: rm -rf build" ||
		withReason.Lines[1].Text != "skipo" || withReason.Lines[1].Icon == nil {
		t.Fatalf("lines %+v, want the reason then the project with its folder", withReason.Lines)
	}
	if len(without.Lines) != 1 || without.Lines[0].Text != "lich" {
		t.Fatalf("lines %+v, want only the project when the provider gave no reason", without.Lines)
	}
	if withReason.Duration != attentionToastDuration || withReason.Icon == nil {
		t.Fatalf("duration %v, icon %v", withReason.Duration, withReason.Icon)
	}
	withReason.Action.OnClick()
	if len(r.opened) != 1 || r.opened[0] != [2]string{"q", "c"} {
		t.Fatalf("Open routed to %v, want q/c", r.opened)
	}
}
