package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestThreadsPromptNamesWhereAndWhatWasSaid(t *testing.T) {
	d := backend.PRDetail{Number: 933, HeadRefName: "fix/kiro-paste", URL: "https://github.com/o/r/pull/933"}
	threads := []backend.PRReviewThread{
		{ID: "PRRT_1", Path: "internal/relay/submit.go", StartLine: 41, Line: 58, Comments: []backend.PRThreadComment{
			{ID: 2201, Author: "reviewer", Body: "Wait for the echo?\nOnly on Kiro."},
			{ID: 2202, Body: "Agreed."},
		}},
		{ID: "PRRT_2", Path: "pulls.go", Line: 112, IsOutdated: true, Comments: []backend.PRThreadComment{{ID: 3301, Author: "reviewer", Body: "Duplicated rule."}}},
	}
	want := "Review thread on pull request #933 (fix/kiro-paste), internal/relay/submit.go:41-58:\n" +
		"Thread id: PRRT_1\n" +
		"https://github.com/o/r/pull/933#discussion_r2201\n\n" +
		"- @reviewer: Wait for the echo?\n  Only on Kiro.\n" +
		"- @ghost: Agreed.\n\n" +
		"Review thread on pull request #933 (fix/kiro-paste), pulls.go:112 (outdated, check the current code):\n" +
		"Thread id: PRRT_2\n" +
		"https://github.com/o/r/pull/933#discussion_r3301\n\n" +
		"- @reviewer: Duplicated rule.\n\n" +
		"Go through each thread. One that asks for a change: make it, commit it and call mark_thread_fixed with that thread's id and the commit, " +
		"and lich resolves the thread and replies on GitHub once that commit is pushed. One that asks a question or raises a point for the team: " +
		"change nothing for it, and bring it to me with your view. Never reply to or resolve a thread on GitHub yourself."
	if got := threadsPrompt(d, threads); got != want {
		t.Errorf("threadsPrompt =\n%s\nwant\n%s", got, want)
	}
	one := threadsPrompt(d, threads[1:])
	if wantTail := "If it asks a question or raises a point for the team, change nothing: bring it to me with your view. " +
		"Never reply to or resolve the thread on GitHub yourself."; one[len(one)-len(wantTail):] != wantTail {
		t.Errorf("one thread's ask = %q", one)
	}
}

func TestOnlyOpenUnsentThreadsGo(t *testing.T) {
	p := &pullsScreen{threadState: func(id string) (bool, bool) { return id == "sent" || id == "fixed", id == "fixed" }}
	said := []backend.PRThreadComment{{ID: 1}}
	conv := &backend.PRConversation{Threads: []backend.PRReviewThread{
		{ID: "open", Comments: said},
		{ID: "resolved", IsResolved: true, Comments: said},
		{ID: "sent", Comments: said},
		{ID: "fixed", Comments: said},
		{ID: "empty"},
	}}
	got := p.sendableThreads(conv)
	if len(got) != 1 || got[0].ID != "open" {
		t.Errorf("sendableThreads = %+v, want only the open one", got)
	}
	if len((&pullsScreen{}).sendableThreads(conv)) != 3 {
		t.Error("with nothing sent, every open thread with a comment goes")
	}
	if !p.threadSendable(conv.Threads[2]) || p.threadSendable(conv.Threads[3]) {
		t.Error("a sent thread its agent never marked goes again on its own; a marked one waits for its push")
	}
	for id, want := range map[string]string{"open": "", "sent": "Sent to session", "fixed": "Fixed · waiting for a push"} {
		if got := p.threadSentLabel(id); got != want {
			t.Errorf("threadSentLabel(%s) = %q, want %q", id, got, want)
		}
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
