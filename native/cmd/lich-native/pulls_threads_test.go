package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestThreadsPromptNamesWhereAndWhatWasSaid(t *testing.T) {
	d := backend.PRDetail{Number: 933, HeadRefName: "fix/kiro-paste", URL: "https://github.com/o/r/pull/933"}
	threads := []backend.PRReviewThread{
		{Path: "internal/relay/submit.go", StartLine: 41, Line: 58, Comments: []backend.PRThreadComment{
			{ID: 2201, Author: "reviewer", Body: "Wait for the echo?\nOnly on Kiro."},
			{ID: 2202, Body: "Agreed."},
		}},
		{Path: "pulls.go", Line: 112, IsOutdated: true, Comments: []backend.PRThreadComment{{ID: 3301, Author: "reviewer", Body: "Duplicated rule."}}},
	}
	want := "Review thread on pull request #933 (fix/kiro-paste), internal/relay/submit.go:41-58:\n" +
		"https://github.com/o/r/pull/933#discussion_r2201\n\n" +
		"- @reviewer: Wait for the echo?\n  Only on Kiro.\n" +
		"- @ghost: Agreed.\n\n" +
		"Review thread on pull request #933 (fix/kiro-paste), pulls.go:112 (outdated, check the current code):\n" +
		"https://github.com/o/r/pull/933#discussion_r3301\n\n" +
		"- @reviewer: Duplicated rule.\n\n" +
		"Address them and commit the fixes. lich replies to and resolves the threads on GitHub once the fixes are pushed, so don't."
	if got := threadsPrompt(d, threads); got != want {
		t.Errorf("threadsPrompt =\n%s\nwant\n%s", got, want)
	}
	one := threadsPrompt(d, threads[1:])
	if wantTail := "Address it and commit the fix. lich replies to and resolves the thread on GitHub once the fix is pushed, so don't."; one[len(one)-len(wantTail):] != wantTail {
		t.Errorf("one thread's ask = %q", one)
	}
}

func TestOnlyOpenUnsentThreadsGo(t *testing.T) {
	p := &pullsScreen{waiting: func(id string) bool { return id == "sent" }}
	said := []backend.PRThreadComment{{ID: 1}}
	conv := &backend.PRConversation{Threads: []backend.PRReviewThread{
		{ID: "open", Comments: said},
		{ID: "resolved", IsResolved: true, Comments: said},
		{ID: "sent", Comments: said},
		{ID: "empty"},
	}}
	got := p.sendableThreads(conv)
	if len(got) != 1 || got[0].ID != "open" {
		t.Errorf("sendableThreads = %+v, want only the open one", got)
	}
	if len((&pullsScreen{}).sendableThreads(conv)) != 2 {
		t.Error("with nothing waiting, every open thread with a comment goes")
	}
}

func TestCanSendThreadsNeedsAnOpenPushableBranch(t *testing.T) {
	cases := []struct {
		d    backend.PRDetail
		want bool
	}{
		{backend.PRDetail{State: "OPEN"}, true},
		{backend.PRDetail{State: "MERGED"}, false},
		{backend.PRDetail{State: "OPEN", IsCrossRepository: true}, false},
		{backend.PRDetail{State: "OPEN", IsCrossRepository: true, MaintainerCanModify: true}, true},
	}
	for _, c := range cases {
		if got := canSendThreads(c.d); got != c.want {
			t.Errorf("canSendThreads(%+v) = %v, want %v", c.d, got, c.want)
		}
	}
}
