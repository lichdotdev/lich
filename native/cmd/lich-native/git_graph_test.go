package main

import (
	"reflect"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
)

func commits(spec ...[]string) []backend.GitCommit {
	out := make([]backend.GitCommit, len(spec))
	for i, s := range spec {
		out[i] = backend.GitCommit{Hash: s[0], Parents: s[1:]}
	}
	return out
}

func TestLayoutGitGraphKeepsAStraightHistoryOnOneLane(t *testing.T) {
	rows, lanes := layoutGitGraph(commits([]string{"c", "b"}, []string{"b", "a"}, []string{"a"}))
	if lanes != 1 {
		t.Fatalf("lanes = %d, want 1", lanes)
	}
	for i, r := range rows {
		if r.lane != 0 || r.color != 0 || len(r.through) != 0 {
			t.Errorf("row %d = %+v", i, r)
		}
	}
	if len(rows[0].up) != 0 || len(rows[2].down) != 0 {
		t.Errorf("the tip has nothing above and the root nothing below: %+v", rows)
	}
}

func TestLayoutGitGraphGivesABranchItsOwnLaneAndColor(t *testing.T) {
	rows, lanes := layoutGitGraph(commits([]string{"f", "p"}, []string{"g", "p"}, []string{"p"}))
	if lanes != 2 {
		t.Fatalf("lanes = %d, want 2", lanes)
	}
	if rows[1].lane != 1 || rows[1].color == rows[0].color {
		t.Errorf("second branch = %+v", rows[1])
	}
	if want := []gitLink{{0, rows[0].color}}; !reflect.DeepEqual(rows[1].through, want) {
		t.Errorf("first branch must pass by: %+v", rows[1].through)
	}
	if want := []gitLink{{0, rows[0].color}, {1, rows[1].color}}; rows[2].lane != 0 || !reflect.DeepEqual(rows[2].up, want) {
		t.Errorf("both branches must meet at their parent: %+v", rows[2])
	}
}

func TestLayoutGitGraphDrawsAMergeToBothParents(t *testing.T) {
	rows, _ := layoutGitGraph(commits([]string{"m", "a", "b"}, []string{"a", "r"}, []string{"b", "r"}, []string{"r"}))
	if len(rows[0].down) != 2 || rows[0].down[0].lane != 0 || rows[0].down[1].lane != 1 {
		t.Fatalf("merge = %+v", rows[0])
	}
	if rows[0].down[0].color != rows[0].color || rows[0].down[1].color == rows[0].color {
		t.Errorf("the first parent keeps the color, the second takes its own: %+v", rows[0].down)
	}
	if rows[2].lane != 1 {
		t.Errorf("second parent lane = %d", rows[2].lane)
	}
}

func TestLayoutGitGraphReusesAFreedLane(t *testing.T) {
	rows, lanes := layoutGitGraph(commits([]string{"x"}, []string{"y", "z"}, []string{"z"}))
	if rows[1].lane != 0 || rows[2].lane != 0 || lanes != 1 {
		t.Errorf("rows %+v, lanes %d", rows, lanes)
	}
}

func TestLayoutGitGraphKeepsAParentNotLoadedYet(t *testing.T) {
	rows, _ := layoutGitGraph(commits([]string{"a", "off-page"}))
	if len(rows[0].down) != 1 {
		t.Errorf("a parent past the page still leaves a line: %+v", rows[0])
	}
}

func TestLayoutGitGraphReusesTheLaneOfAParentAlreadyWaited(t *testing.T) {
	rows, _ := layoutGitGraph(commits([]string{"t", "p"}, []string{"m", "x", "p"}, []string{"x", "p"}, []string{"p"}))
	if rows[1].down[1].lane != 0 {
		t.Errorf("merge into a waited parent = %+v", rows[1].down)
	}
}

func TestGitRefLabelOf(t *testing.T) {
	local := func(n string, head bool) backend.GitRef {
		return backend.GitRef{Name: n, Kind: backend.GitRefLocal, Head: head}
	}
	remote := func(n string) backend.GitRef { return backend.GitRef{Name: n, Kind: backend.GitRefRemote} }
	tag := backend.GitRef{Name: "v1", Kind: backend.GitRefTag}
	cases := []struct {
		name string
		refs []backend.GitRef
		want gitRefLabel
	}{
		{"head wins", []backend.GitRef{local("a", false), local("b", true)}, gitRefLabel{text: "b", branch: "b", kind: backend.GitRefLocal, head: true, extra: 1}},
		{"local and its remote read as one", []backend.GitRef{remote("origin/main"), local("main", false)}, gitRefLabel{text: "main & origin", branch: "main", kind: backend.GitRefLocal}},
		{"remote before tag", []backend.GitRef{tag, remote("origin/x")}, gitRefLabel{text: "origin/x", kind: backend.GitRefRemote, extra: 1}},
		{"tag alone", []backend.GitRef{tag}, gitRefLabel{text: "v1", kind: backend.GitRefTag}},
		{"a remote of another name is not merged", []backend.GitRef{local("a", false), remote("origin/b")}, gitRefLabel{text: "a", branch: "a", kind: backend.GitRefLocal, extra: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, ok := gitRefLabelOf(c.refs); !ok || got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
	if _, ok := gitRefLabelOf(nil); ok {
		t.Error("no refs, no label")
	}
}

func TestGitWhen(t *testing.T) {
	now := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	cases := map[time.Time]string{
		now.Add(-time.Hour):   "17:00",
		now.AddDate(0, 0, -2): "Nov 8",
		now.AddDate(-1, 0, 0): "Nov 10, 2025",
	}
	for at, want := range cases {
		if got := gitWhen(at.Unix(), now); got != want {
			t.Errorf("gitWhen(%v) = %q, want %q", at, got, want)
		}
	}
}

func TestGitTreeRowsGroupsAndFilters(t *testing.T) {
	refs := []backend.GitRef{
		{Name: "feat/x", Kind: backend.GitRefLocal, Head: true},
		{Name: "origin/feat/x", Kind: backend.GitRefRemote},
		{Name: "upstream/main", Kind: backend.GitRefRemote},
		{Name: "v1", Kind: backend.GitRefTag},
	}
	rows := gitTreeRows(refs, "")
	labels := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.label
	}
	want := []string{"Local", "feat/x", "origin", "feat/x", "upstream", "main", "Tags", "v1"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v", labels)
	}
	if rows[3].ref != "refs/remotes/origin/feat/x" || rows[1].branch != "feat/x" || rows[3].branch != "" {
		t.Errorf("rows = %+v", rows)
	}
	if got := gitTreeRows(refs, "V1"); len(got) != 2 || !got[0].group || !got[1].tag {
		t.Errorf("filtered = %+v, a group nothing matched is left out", got)
	}
}

func TestGitLogFilterNarrowed(t *testing.T) {
	if (gitLogFilter{ref: "refs/heads/x"}).narrowed() {
		t.Error("a ref keeps the whole ancestry, and so the graph")
	}
	for _, f := range []gitLogFilter{{query: "q"}, {author: "a"}, {file: "f"}, {since: 1}} {
		if !f.narrowed() {
			t.Errorf("%+v leaves parents out", f)
		}
	}
}

func TestParseGitHeight(t *testing.T) {
	cases := map[string]float64{"": gitDefaultRem, "x": gitDefaultRem, "-3": gitDefaultRem, "1": gitMinRem, "999": gitMaxRem, "20": 20}
	for raw, want := range cases {
		if got := parseGitHeight(raw); got != want {
			t.Errorf("parseGitHeight(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestFooterInputInRepository(t *testing.T) {
	cases := []struct {
		name string
		in   footerInput
		want bool
	}{
		{"not answered yet", footerInput{}, false},
		{"a plain folder answers with nothing", footerInput{isGit: true}, false},
		{"on a branch", footerInput{isGit: true, git: backend.DiffStats{Branch: "main"}}, true},
		{"detached", footerInput{isGit: true, git: backend.DiffStats{Head: "abc"}}, true},
	}
	for _, c := range cases {
		if got := c.in.inRepository(); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestGitColumnStopsAtTheRoomTheLogLeaves(t *testing.T) {
	gtx := C{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Ops: new(op.Ops), Source: input.Source{}}
	c := gitColumn{rem: 30, minRem: 9, maxRem: 32}
	if px, _ := c.width(gtx, 320); px != 320 {
		t.Errorf("width = %d, want the room left, 320", px)
	}
	c = gitColumn{rem: 30, minRem: 9, maxRem: 32}
	if px, _ := c.width(gtx, 10); px != 144 {
		t.Errorf("width = %d, want the column's floor even with no room", px)
	}
}

func TestParseGitRemKeepsTheFallbackForGarbage(t *testing.T) {
	if got := parseGitRem("nope", 12.5, 9, 32); got != 12.5 {
		t.Errorf("got %v", got)
	}
	if got := parseGitRem("99", 12.5, 9, 32); got != 32 {
		t.Errorf("got %v", got)
	}
}

func TestGitFilesTotal(t *testing.T) {
	a, d := gitFilesTotal([]backend.GitCommitFile{{Added: 3, Deleted: 1}, {Binary: true}, {Added: 2}})
	if a != 5 || d != 1 {
		t.Errorf("got +%d -%d", a, d)
	}
}

func TestGitBranchMenu(t *testing.T) {
	acts := func(items []gitMenuItem) []gitBranchAct {
		out := make([]gitBranchAct, len(items))
		for i, it := range items {
			out[i] = it.act
		}
		return out
	}
	cases := []struct {
		name    string
		row     gitTreeRow
		session bool
		want    []gitBranchAct
		blocked gitBlocked
	}{
		{"a local branch", gitTreeRow{}, false, []gitBranchAct{gitActCheckout, gitActNewBranch, gitActRename, gitActDelete}, gitFree},
		{"the branch you are on", gitTreeRow{head: true, worktree: "/here"}, false, []gitBranchAct{gitActNewBranch, gitActRename, gitActDelete}, gitOnBranch},
		{"held by a worktree with a session", gitTreeRow{worktree: "/wt"}, true, []gitBranchAct{gitActOpenSession, gitActNewBranch, gitActRename, gitActDelete}, gitInWorktree},
		{"held by a worktree without one", gitTreeRow{worktree: "/wt"}, false, []gitBranchAct{gitActNewBranch, gitActRename, gitActDelete}, gitInWorktree},
		{"a remote branch", gitTreeRow{remote: true}, false, []gitBranchAct{gitActCheckoutRemote, gitActNewBranch}, gitFree},
		{"a tag", gitTreeRow{tag: true}, false, []gitBranchAct{gitActNewBranch}, gitFree},
	}
	for _, c := range cases {
		items := gitBranchMenu(c.row, c.session)
		if got := acts(items); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		if last := items[len(items)-1]; last.act == gitActDelete && last.blocked != c.blocked {
			t.Errorf("%s: delete blocked %v, want %v", c.name, last.blocked, c.blocked)
		}
	}
}
