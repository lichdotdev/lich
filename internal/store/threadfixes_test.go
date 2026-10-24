package store

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestThreadFixesRoundTripInSendOrder(t *testing.T) {
	svc := newTestStore(t)
	want := []api.ThreadFix{
		{ThreadID: "PRRT_b", Path: "/repo", Number: 7, CommentID: 11, Head: "aaa"},
		{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 12, Head: "aaa"},
	}
	for _, f := range want {
		if err := svc.SaveThreadFix(f); err != nil {
			t.Fatal(err)
		}
	}
	got, err := svc.ThreadFixes()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("ThreadFixes() = %+v, want %+v", got, want)
	}
}

func TestSaveThreadFixAgainWaitsOnTheNewHead(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SaveThreadFix(api.ThreadFix{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 11, Head: "aaa"}); err != nil {
		t.Fatal(err)
	}
	again := api.ThreadFix{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 13, Head: "bbb"}
	if err := svc.SaveThreadFix(again); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ThreadFixes()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []api.ThreadFix{again}) {
		t.Errorf("ThreadFixes() = %+v, want only %+v", got, again)
	}
}

func TestDeleteThreadFix(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SaveThreadFix(api.ThreadFix{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 11, Head: "aaa"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := svc.DeleteThreadFix("PRRT_a"); err != nil {
			t.Fatalf("DeleteThreadFix: %v", err)
		}
	}
	if got, err := svc.ThreadFixes(); err != nil || len(got) != 0 {
		t.Errorf("ThreadFixes() = %+v, %v; want none", got, err)
	}
}
