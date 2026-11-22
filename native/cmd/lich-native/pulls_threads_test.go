package main

import (
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// The window hands the backend each thread where it sits, its link and its
// comments; the backend words them and the ask (internal/relay, threadsText).
func TestThreadsRequestCarriesWhereAndWhatWasSaid(t *testing.T) {
	d := backend.PRDetail{Number: 933, HeadRefName: "fix/kiro-paste", URL: "https://github.com/o/r/pull/933"}
	threads := []backend.PRReviewThread{
		{ID: "PRRT_1", Path: "internal/relay/submit.go", StartLine: 41, Line: 58, Comments: []backend.PRThreadComment{
			{ID: 2201, Author: "reviewer", Body: "Wait for the echo?"}, {ID: 2202, Body: "Agreed."},
		}},
		{ID: "PRRT_2", Path: "pulls.go", Line: 112, IsOutdated: true, Comments: []backend.PRThreadComment{{ID: 3301, Author: "reviewer", Body: "Duplicated rule."}}},
	}
	want := api.RequestOptions{Topic: api.RequestReviewThread, PR: 933, Branch: "fix/kiro-paste", Items: []api.RequestItem{
		{Path: "internal/relay/submit.go", Lines: "41-58", ID: "PRRT_1", URL: "https://github.com/o/r/pull/933#discussion_r2201",
			Comments: []api.RequestComment{{Author: "reviewer", Body: "Wait for the echo?"}, {Body: "Agreed."}}},
		{Path: "pulls.go", Lines: "112", ID: "PRRT_2", Outdated: true, URL: "https://github.com/o/r/pull/933#discussion_r3301",
			Comments: []api.RequestComment{{Author: "reviewer", Body: "Duplicated rule."}}},
	}}
	if got := threadsRequest(d, threads); !reflect.DeepEqual(got, want) {
		t.Errorf("threadsRequest = %+v\nwant %+v", got, want)
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
