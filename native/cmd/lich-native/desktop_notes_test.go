package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRecapLineFoldsAndCutsTheClosingWords(t *testing.T) {
	if got := recapLine("Done.\n\n  All   green."); got != "Done. All green." {
		t.Fatalf("recapLine folded to %q", got)
	}
	long := strings.Repeat("palavra ", 40)
	got := recapLine(long)
	if n := utf8.RuneCountInString(got); n > noteRecapLimit || !strings.HasSuffix(got, "…") {
		t.Fatalf("recapLine of %d runes gave %d runes %q, want at most %d ending in …", len(long), n, got, noteRecapLimit)
	}
}

func TestANoteSaysWhatTheSessionAsksAndWhereItLives(t *testing.T) {
	r := statusReport{id: "a", projectID: "p", status: statusWaiting, reason: "Allow Bash: git push", project: "lich", branch: "fix/auth", kind: "claude"}
	n := noteFor(r, "Fix auth needs your input")
	if n.body != "Allow Bash: git push" || n.meta != "lich · fix/auth" || n.kind != "claude" {
		t.Fatalf("noteFor = %+v", n)
	}
	r.status, r.branch = statusDone, ""
	if n := noteFor(r, "x"); n.body != "" || n.meta != "lich" {
		t.Fatalf("finished noteFor = %+v, want no reason and the project alone", n)
	}
}
