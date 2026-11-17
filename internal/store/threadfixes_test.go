package store

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestThreadFixesRoundTripInSendOrder(t *testing.T) {
	svc := newTestStore(t)
	want := []api.ThreadFix{
		{ThreadID: "PRRT_b", Path: "/repo", Number: 7, CommentID: 11},
		{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 12, Commit: "abc1234567"},
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

func TestSaveThreadFixAgainReplacesTheEarlierOne(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SaveThreadFix(api.ThreadFix{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 11, Commit: "abc1234567"}); err != nil {
		t.Fatal(err)
	}
	again := api.ThreadFix{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 13}
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
	if err := svc.SaveThreadFix(api.ThreadFix{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 11}); err != nil {
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

// A thread sent before the agent named its fix keeps waiting, with no commit
// for a push to resolve it on.
func TestThreadFixSentBeforeCommitsKeepsWaitingUnmarked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v16.db")
	saved := migrations
	migrations = saved[:16]
	old, err := open(path)
	migrations = saved
	if err != nil {
		t.Fatalf("open at version 16: %v", err)
	}
	_, err = old.db.Exec(`INSERT INTO thread_fixes (thread_id, path, number, comment_id, head) VALUES ('PRRT_a', '/repo', 7, 11, 'aaa')`)
	_ = old.Close()
	if err != nil {
		t.Fatalf("seed a waiting thread: %v", err)
	}
	svc, err := open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer svc.Close()
	got, err := svc.ThreadFixes()
	if want := []api.ThreadFix{{ThreadID: "PRRT_a", Path: "/repo", Number: 7, CommentID: 11}}; err != nil || !slices.Equal(got, want) {
		t.Errorf("ThreadFixes() = %+v, %v; want %+v", got, err, want)
	}
}
