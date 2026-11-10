package project

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestFillPRReadTakesTheLatestSubmittedReview(t *testing.T) {
	t.Parallel()
	var read api.PRRead
	out := `{"headRefOid":"abc","statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"},{"status":"IN_PROGRESS"}],
		"reviews":[{"id":"R1","state":"APPROVED"},{"id":"R2","state":"COMMENTED"},{"id":"R3","state":"PENDING"}]}`
	if err := fillPRRead(&read, []byte(out)); err != nil {
		t.Fatal(err)
	}
	want := api.PRRead{Head: "abc", Review: "R2", Checks: api.ChecksRollup{Failed: 1, Pending: 1, Total: 2}}
	if read != want {
		t.Fatalf("read = %+v, want %+v", read, want)
	}
	if err := fillPRRead(&read, []byte(`not json`)); err == nil {
		t.Fatal("malformed answer: want a decode error")
	}
}

func TestOnlyAWatchedCheckoutPaysForTheSecondRead(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	forge.set("/repo", featOpen)
	w := newWatching(forge)
	var reads []api.PRRead
	watched := false
	w.SetPullRequestWatch(func(string) bool { return watched }, func(r api.PRRead) { reads = append(reads, r) })

	w.recordBranchPR("/repo")
	if n := forge.readsOf("/repo"); n != 1 || len(reads) != 0 {
		t.Fatalf("unwatched: %d gh calls and %d reports, want 1 and 0", n, len(reads))
	}

	watched = true
	w.recordBranchPR("/repo")
	if n := forge.readsOf("/repo"); n != 3 || len(reads) != 1 || !reads[0].Open || reads[0].Number != 42 {
		t.Fatalf("watched: %d gh calls and reports %+v", n, reads)
	}
}

func TestAClosedPullRequestIsReportedWithoutReadingItsChecks(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	forge.set("/repo", featMerged)
	w := newWatching(forge)
	var reads []api.PRRead
	w.SetPullRequestWatch(func(string) bool { return true }, func(r api.PRRead) { reads = append(reads, r) })

	w.recordBranchPR("/repo")

	if n := forge.readsOf("/repo"); n != 1 || len(reads) != 1 || reads[0].Open {
		t.Fatalf("%d gh calls and reports %+v, want one call and a closed read", n, reads)
	}
}
