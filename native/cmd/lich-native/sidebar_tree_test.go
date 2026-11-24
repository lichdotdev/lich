package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func treeSession(id, origin, path string) sessionView {
	return sessionView{ID: id, OriginSessionID: origin, path: path}
}

func rowShape(rows []sessionRow) []string {
	var out []string
	for _, r := range rows {
		mark := ""
		if r.tail {
			mark = "."
		}
		out = append(out, string(rune('0'+r.depth))+":"+r.key+mark)
	}
	return out
}

func TestSessionRowsNestEachGeneration(t *testing.T) {
	sessions := []sessionView{
		treeSession("mother", "", "/repo"),
		treeSession("solo", "", "/repo/wt/solo"),
		treeSession("child", "mother", "/repo"),
		treeSession("grandchild", "child", "/repo"),
		treeSession("second", "mother", "/repo"),
	}
	rows := sessionRows(sessions, func(string) bool { return false }, nil)
	got := rowShape(rows)
	want := []string{"0:mother", "1:child", "2:grandchild.", "1:second.", "0:solo."}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if !slices.Equal(rows[2].lanes, []bool{true}) {
		t.Errorf("grandchild lanes = %v: the mother's line runs on to second", rows[2].lanes)
	}
	if rows[0].kids != 3 || rows[1].kids != 1 {
		t.Errorf("kids = %d and %d, want 3 under the mother and 1 under child", rows[0].kids, rows[1].kids)
	}
}

func TestSessionRowsFoldAMotherAtAnyDepth(t *testing.T) {
	sessions := []sessionView{treeSession("mother", "", "/repo"), treeSession("child", "mother", "/repo"), treeSession("grandchild", "child", "/repo")}
	got := rowShape(sessionRows(sessions, func(id string) bool { return id == "child" }, nil))
	if want := []string{"0:mother.", "1:child."}; !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestSessionRowsHangAChildInAnotherCheckoutFromItsMother(t *testing.T) {
	sessions := []sessionView{
		treeSession("mother", "", "/repo/wt/a"),
		treeSession("tests", "mother", "/repo/wt/a"),
		treeSession("flake", "mother", "/repo/wt/b"),
	}
	rows := sessionRows(sessions, func(string) bool { return false }, nil)
	want := []string{"0:mother.", "1:tests", "1:flake."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v: no checkout row between a mother and a child", got, want)
	}
}

func TestSessionRowsFoldAMotherWithTheMarkOfHerChildren(t *testing.T) {
	waiting := treeSession("asks", "mother", "/repo")
	waiting.status = statusWaiting
	sessions := []sessionView{treeSession("mother", "", "/repo"), treeSession("done", "mother", "/repo"), waiting}
	rows := sessionRows(sessions, func(id string) bool { return id == "mother" }, nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want the mother alone", rowShape(rows))
	}
	if r := rows[0]; !r.folded || r.kids != 2 || r.mark != markWait || r.stub {
		t.Errorf("mother = %+v, want folded over 2 with the waiting mark and no stub", r)
	}
}

func TestSessionRowsMakeAnOrphanARoot(t *testing.T) {
	sessions := []sessionView{treeSession("child", "closed-mother", "/repo")}
	if got := rowShape(sessionRows(sessions, func(string) bool { return false }, nil)); !slices.Equal(got, []string{"0:child."}) {
		t.Errorf("rows = %v, want the orphan as a root", got)
	}
}

func TestSessionBlocksPutAChildWithItsMother(t *testing.T) {
	sessions := []backend.Session{
		{ID: "mother", Path: "/repo/wt/a"},
		{ID: "flake", Path: "/repo/wt/b", OriginSessionID: "mother"},
		{ID: "filed", Path: "/repo/wt/b", OriginSessionID: "mother", Folder: "Later"},
	}
	blocks := sessionBlocks(sessions)
	if len(blocks) != 2 {
		t.Fatalf("blocks = %+v, want the loose list and the folder", blocks)
	}
	if b := blocks[0]; b.key != rootBlockKey || len(b.sessions) != 2 {
		t.Errorf("first block = %+v, want the loose list with the mother and flake", b)
	}
	if b := blocks[1]; b.folder != "Later" {
		t.Errorf("second block = %+v, want the folder a child filed itself in", b)
	}
}

func TestRootsOfSurvivesACycle(t *testing.T) {
	roots := rootsOf([]backend.Session{{ID: "a", OriginSessionID: "b"}, {ID: "b", OriginSessionID: "a"}})
	if roots["a"] == "" || roots["b"] == "" {
		t.Errorf("roots = %v, want an answer for both", roots)
	}
}

func TestFilterKeepsTheMotherOfAMatchedChild(t *testing.T) {
	sessions := []backend.Session{
		{ID: "mother", Label: "Sidebar pass"},
		{ID: "other", Label: "Docs"},
		{ID: "child", Label: "Row flake", OriginSessionID: "mother"},
	}
	kept, matched := filterSessions(sessions, "flake", "/repo", "", nil, func(string) sessionPhase { return "" })
	var ids []string
	for _, s := range kept {
		ids = append(ids, s.ID)
	}
	if !matched || !slices.Equal(ids, []string{"mother", "child"}) {
		t.Errorf("kept = %v (matched %v), want the child and the mother it hangs from", ids, matched)
	}
}

func TestAFoldersRootsSharingACheckoutGatherUnderItsLabel(t *testing.T) {
	b := blockView{key: "__folder__:Later"}
	sessions := []sessionView{
		treeSession("a", "", "/repo/wt/a"),
		treeSession("b", "", "/repo/wt/b"),
		treeSession("a-child", "a", "/repo/wt/c"),
		treeSession("a2", "", "/repo/wt/a"),
	}
	want := []string{"0:__checkout__:/repo/wt/a", "0:a", "1:a-child.", "0:a2", "0:b."}
	if got := rowShape(blockRows(b, sessions)); !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestACheckoutWithOneRootHasNoLabel(t *testing.T) {
	sessions := []sessionView{
		treeSession("mother", "", "/repo/wt/a"),
		treeSession("shell", "mother", "/repo/wt/a"),
		treeSession("other", "", "/repo/wt/b"),
	}
	want := []string{"0:mother", "1:shell.", "0:other."}
	if got := rowShape(sessionRows(sessions, func(string) bool { return false }, nil)); !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v: children do not count toward a label", got, want)
	}
}

func TestTheLabelOrTheMotherSaysTheBranch(t *testing.T) {
	sessions := []sessionView{
		treeSession("polish", "", "/repo/wt/a"),
		treeSession("shell", "polish", "/repo/wt/a"),
		treeSession("sweep", "polish", "/repo/wt/s"),
		treeSession("docs", "", "/repo"),
		treeSession("spacing", "", "/repo/wt/a"),
	}
	said := map[string]bool{}
	for _, r := range sessionRows(sessions, func(string) bool { return false }, nil) {
		if r.kind == rowCard {
			said[r.key] = r.branchSaid
		}
	}
	want := map[string]bool{"polish": true, "shell": true, "sweep": false, "spacing": true, "docs": false}
	for id, w := range want {
		if said[id] != w {
			t.Errorf("%s branchSaid = %v, want %v", id, said[id], w)
		}
	}
}
