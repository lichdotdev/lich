package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func thread(id, at string, resolved bool) backend.PRReviewThread {
	return backend.PRReviewThread{ID: id, IsResolved: resolved, Comments: []backend.PRThreadComment{{Date: at}}}
}

func TestConversationTimelineOrdersOldestFirstAndSetsResolvedApart(t *testing.T) {
	c := &backend.PRConversation{
		Reviews:  []backend.PRReview{{ID: "r", Date: "2026-01-03T00:00:00Z"}},
		Comments: []backend.PRComment{{Author: "c", Date: "2026-01-01T00:00:00Z"}},
		Threads: []backend.PRReviewThread{
			thread("open", "2026-01-02T00:00:00Z", false),
			thread("old", "2026-01-01T00:00:00Z", true),
			thread("new", "2026-01-05T00:00:00Z", true),
		},
	}
	items, resolved := conversationTimeline(c)
	if len(items) != 3 || items[0].comment == nil || items[1].thread == nil || items[2].review == nil {
		t.Fatalf("items out of order: %+v", items)
	}
	if len(resolved) != 2 || resolved[0].ID != "new" || resolved[1].ID != "old" {
		t.Errorf("resolved = %+v, want newest first", resolved)
	}
	if open, total := threadTally(c); open != 1 || total != 3 {
		t.Errorf("tally = %d of %d, want 1 of 3", open, total)
	}
	if items, resolved := conversationTimeline(nil); items != nil || resolved != nil {
		t.Error("no conversation, no timeline")
	}
}

func TestLineSpan(t *testing.T) {
	cases := map[string][2]int{"19": {0, 19}, "12-30": {12, 30}}
	for in, want := range cases {
		if start, line := lineSpan(in); start != want[0] || line != want[1] {
			t.Errorf("lineSpan(%q) = %d,%d, want %v", in, start, line, want)
		}
	}
}

func TestPendingReviewCollectsComments(t *testing.T) {
	var r pendingReview
	r.add("a.go", "3-5", "first")
	r.add("b.go", "9", "second")
	got := r.snapshot()
	if len(got) != 2 || got[0].StartLine != 3 || got[0].Line != 5 || got[1].StartLine != 0 || got[1].Side != "RIGHT" {
		t.Fatalf("comments = %+v", got)
	}
	r.remove(0)
	if got := r.snapshot(); len(got) != 1 || got[0].Path != "b.go" {
		t.Errorf("after remove = %+v", got)
	}
	r.clear()
	if len(r.snapshot()) != 0 {
		t.Error("clear left comments")
	}
}

// A review taken by GitHub drops what it carried, and only that: a comment
// written while the submit was on its way stays pending.
func TestSubmittedReviewDropsOnlyWhatWasSent(t *testing.T) {
	var r pendingReview
	r.add("a.go", "3", "sent")
	sent := r.snapshot()
	r.add("b.go", "9", "written meanwhile")
	r.drop(sent)
	if got := r.snapshot(); len(got) != 1 || got[0].Body != "written meanwhile" {
		t.Fatalf("pending after the submit = %+v, want only the later comment", got)
	}
}

func TestPendingReviewEditsOneComment(t *testing.T) {
	var r pendingReview
	r.add("a.go", "3-5", "first")
	r.add("b.go", "9", "second")
	r.edit(0, "first, reworded\non two lines")
	got := r.snapshot()
	if len(got) != 2 || got[0].Body != "first, reworded\non two lines" || got[0].Path != "a.go" || got[0].StartLine != 3 || got[0].Line != 5 || got[1].Body != "second" {
		t.Fatalf("after edit = %+v, want the first reworded in place, the second untouched", got)
	}
}
