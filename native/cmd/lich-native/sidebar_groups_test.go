package main

import (
	"reflect"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
)

// ss builds a stored session list: "id" for the project root, "id@path" for
// a worktree; filed and pinned are set by the caller.
func ss(specs ...string) []lichclient.Session {
	out := make([]lichclient.Session, 0, len(specs))
	for _, spec := range specs {
		s := lichclient.Session{ID: spec, Label: spec}
		for i := range spec {
			if spec[i] == '@' {
				s.ID, s.Label, s.Path = spec[:i], spec[:i], spec[i+1:]
			}
		}
		out = append(out, s)
	}
	return out
}

func with(sessions []lichclient.Session, id string, edit func(*lichclient.Session)) []lichclient.Session {
	out := slices.Clone(sessions)
	for i := range out {
		if out[i].ID == id {
			edit(&out[i])
		}
	}
	return out
}

func filed(sessions []lichclient.Session, folder string, ids ...string) []lichclient.Session {
	next, _ := fileSessions(sessions, ids, folder)
	return next
}

func pin(sessions []lichclient.Session, id string) []lichclient.Session {
	return with(sessions, id, func(s *lichclient.Session) { s.Pinned = true })
}

// layout is each block's key with its session ids, the shape the web tests
// assert.
func blockLayout(blocks []sessionBlock) [][]string {
	var out [][]string
	for _, b := range blocks {
		row := []string{b.key}
		for _, s := range b.sessions {
			row = append(row, s.ID)
		}
		out = append(out, row)
	}
	return out
}

func assertLayout(t *testing.T, got []sessionBlock, want ...[]string) {
	t.Helper()
	if l := blockLayout(got); !reflect.DeepEqual(l, want) {
		t.Fatalf("blocks = %v, want %v", l, want)
	}
}

func TestBlocksDrawAFolderAboveTheCheckoutItsCardsCameFrom(t *testing.T) {
	stored := filed(ss("s1", "wt1@/wt/a", "wt2@/wt/a"), "Apps", "wt1")
	assertLayout(t, sessionBlocks(stored),
		[]string{rootBlockKey, "s1"}, []string{folderKey("Apps"), "wt1"}, []string{"/wt/a", "wt2"})
}

func TestBlocksGatherAFolderFromEveryCheckout(t *testing.T) {
	stored := filed(ss("s1", "wt1@/wt/a", "wt2@/wt/b"), "Apps", "s1", "wt1", "wt2")
	assertLayout(t, sessionBlocks(stored), []string{folderKey("Apps"), "s1", "wt1", "wt2"})
}

func TestBlocksKeyAFolderApartFromAWorktreeOfTheSameName(t *testing.T) {
	stored := filed(ss("s1", "wt1@/wt/a"), "/wt/a", "s1")
	assertLayout(t, sessionBlocks(stored), []string{folderKeyPrefix + "/wt/a", "s1"}, []string{"/wt/a", "wt1"})
}

func TestBlocksDrawAPinnedFiledCardInThePinnedBlock(t *testing.T) {
	stored := pin(filed(ss("s1", "s2"), "Apps", "s1"), "s1")
	assertLayout(t, sessionBlocks(stored), []string{pinnedBlockKey, "s1"}, []string{rootBlockKey, "s2"})
}

func TestBlocksPutPinnedFirstWhereverItsCardsSit(t *testing.T) {
	stored := pin(pin(ss("a@/wt/a", "s1", "p2"), "p2"), "a")
	assertLayout(t, sessionBlocks(stored), []string{pinnedBlockKey, "a", "p2"}, []string{rootBlockKey, "s1"})
}

func TestBlocksLandWhereTheirFirstCardSits(t *testing.T) {
	stored := filed(ss("s1", "s2", "s3", "s4"), "Apps", "s3", "s4")
	assertLayout(t, sessionBlocks(stored), []string{rootBlockKey, "s1", "s2"}, []string{folderKey("Apps"), "s3", "s4"})
}

func TestBlocksStopDrawingAFolderOnceItsLastCardLeaves(t *testing.T) {
	stored := filed(filed(ss("s1", "s2"), "Apps", "s1"), "", "s1")
	assertLayout(t, sessionBlocks(stored), []string{rootBlockKey, "s1", "s2"})
}

func TestFolderBlockCarriesItsNameAndNoPath(t *testing.T) {
	blocks := sessionBlocks(filed(ss("s1", "s2@/wt/a"), "Design system", "s2"))
	b := blocks[1]
	if b.kind != blockFolder || b.folder != "Design system" || b.path != "" {
		t.Fatalf("folder block = %+v", b)
	}
	if blocks[0].kind != blockCheckout || blocks[0].folder != "" {
		t.Fatalf("checkout block = %+v", blocks[0])
	}
}

func TestFileSessionsLeavesOrderAndIgnoresUnknownIDs(t *testing.T) {
	stored := ss("s1", "s2", "s3")
	next, ok := fileSessions(stored, []string{"s1", "ghost", "s3"}, "Apps")
	if !ok || folderList(next) != "Apps,,Apps" || idList(next) != "s1,s2,s3" {
		t.Fatalf("filed = %s / %s, %v", folderList(next), idList(next), ok)
	}
	if stored[0].Folder != "" {
		t.Fatal("filing mutated the input list")
	}
	if _, ok := fileSessions(stored, []string{"ghost"}, "Apps"); ok {
		t.Fatal("filing only unknown ids reported a change")
	}
}

func TestRenameFolderRewritesUngroupsAndRefusesTheEmptyName(t *testing.T) {
	stored := filed(ss("s1", "s2", "s3"), "Apps", "s1", "s3")
	next, _ := renameFolder(stored, "Apps", "Applications")
	if folderList(next) != "Applications,,Applications" {
		t.Fatalf("renamed = %s", folderList(next))
	}
	if next, _ := renameFolder(stored, "Apps", ""); folderList(next) != ",," {
		t.Fatalf("ungrouped = %s", folderList(next))
	}
	if _, ok := renameFolder(stored, "", "Everything"); ok {
		t.Fatal("renaming the empty folder swept unfiled sessions")
	}
	if _, ok := renameFolder(stored, "Infra", "Ops"); ok {
		t.Fatal("renaming a folder nothing is filed under reported a change")
	}
}

func TestFoldersOfNamesEachOnceInCardOrder(t *testing.T) {
	stored := filed(filed(ss("s1", "s2", "s3", "s4"), "Apps", "s2", "s4"), "Design system", "s3")
	if got := foldersOf(stored); !slices.Equal(got, []string{"Apps", "Design system"}) {
		t.Fatalf("folders = %v", got)
	}
	if got := foldersOf(ss("s1")); len(got) != 0 {
		t.Fatalf("folders of an unfiled project = %v", got)
	}
}

func TestCheckoutsOfNamesEachOnceInCardOrder(t *testing.T) {
	got := checkoutsOf(ss("s1@/wt/front", "s2", "s3@/wt/front", "s4@/wt/back"))
	if !slices.Equal(got, []string{"/wt/front", "", "/wt/back"}) {
		t.Fatalf("checkouts = %q", got)
	}
}

func TestCollapsedMark(t *testing.T) {
	waiting := pendingStatus{ID: "s2", Status: statusWaiting}
	done := pendingStatus{ID: "s1", Status: statusDone}
	cases := []struct {
		name    string
		pending []pendingStatus
		want    string
	}{
		{"nothing of the block queued", []pendingStatus{{ID: "other", Status: statusWaiting}}, markNone},
		{"an empty queue", nil, markNone},
		{"an unread turn", []pendingStatus{done}, markDone},
		{"waiting after done", []pendingStatus{done, waiting}, markWait},
		{"waiting before done", []pendingStatus{waiting, done}, markWait},
	}
	for _, tc := range cases {
		if got := collapsedMark(tc.pending, []string{"s1", "s2"}); got != tc.want {
			t.Errorf("%s: mark = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSharedColor(t *testing.T) {
	stored := with(with(ss("a", "b"), "a", func(s *lichclient.Session) { s.Color = "teal" }), "b", func(s *lichclient.Session) { s.Color = "teal" })
	if got := sharedColor(stored); got != "teal" {
		t.Fatalf("shared = %q, want teal", got)
	}
	if got := sharedColor(with(stored, "b", func(s *lichclient.Session) { s.Color = "red" })); got != "" {
		t.Fatalf("mixed colors shared %q", got)
	}
	if got := sharedColor(with(stored, "a", func(s *lichclient.Session) { s.Color = "mauve" })); got != "" {
		t.Fatalf("a color off the palette shared %q", got)
	}
}

func TestReorderSubset(t *testing.T) {
	stored := pin(pin(ss("a", "p1", "b", "c", "p2"), "p1"), "p2")
	unpinned := func(s lichclient.Session) bool { return !s.Pinned }
	cases := []struct {
		name   string
		ids    []string
		member func(lichclient.Session) bool
		want   []string
	}{
		{"the subset moves, the rest stays", []string{"c", "a", "b"}, unpinned, []string{"c", "p1", "a", "b", "p2"}},
		{"the pinned cards swap in place", []string{"p2", "p1"}, func(s lichclient.Session) bool { return s.Pinned }, []string{"a", "p2", "b", "c", "p1"}},
		// A repeat, not a gap: applyOrder rejects it, dropping a stale drag.
		{"short ids repeat", []string{"c"}, unpinned, []string{"c", "p1", "b", "c", "p2"}},
		{"no member", nil, func(lichclient.Session) bool { return false }, []string{"a", "p1", "b", "c", "p2"}},
	}
	for _, tc := range cases {
		if got := reorderSubset(stored, tc.ids, tc.member); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestOrderBlocksMovesWholeBlocks(t *testing.T) {
	blocks := sessionBlocks(ss("r1", "r2", "a1@/wt/a", "b1@/wt/b", "b2@/wt/b"))
	if got := orderBlocks(blocks, []string{"/wt/b", rootBlockKey, "/wt/a"}); !slices.Equal(got, []string{"b1", "b2", "r1", "r2", "a1"}) {
		t.Fatalf("order = %v", got)
	}
	if got := orderBlocks(blocks, []string{""}); len(got) != 0 {
		t.Fatalf("the root keyed by \"\" gave %v", got)
	}
	if got := orderBlocks(blocks, []string{"/wt/gone"}); len(got) != 0 {
		t.Fatalf("a gone key gave %v", got)
	}
}

func TestDragOrderLeavesThePinnedCardsSlots(t *testing.T) {
	stored := pin(ss("p1", "r1", "a1@/wt/a"), "p1")
	got := dragOrder(stored, sessionBlocks(stored), []string{"/wt/a", rootBlockKey})
	if !slices.Equal(got, []string{"p1", "a1", "r1"}) {
		t.Fatalf("drag order = %v", got)
	}
}

func TestDragOrderMovesAFolderPastACheckout(t *testing.T) {
	stored := filed(ss("a1@/wt/a", "r1", "r2"), "Apps", "r1", "r2")
	got := dragOrder(stored, sessionBlocks(stored), []string{folderKey("Apps"), "/wt/a"})
	if !slices.Equal(got, []string{"r1", "r2", "a1"}) {
		t.Fatalf("drag order = %v", got)
	}
}

func TestBlockOrderFillsOnlyTheBlocksOwnSlots(t *testing.T) {
	stored := filed(ss("r1", "f1", "r2", "f2"), "Apps", "f1", "f2")
	folder := sessionBlocks(stored)[1]
	got := blockOrder(stored, folder, []string{"f2", "f1"})
	if !slices.Equal(got, []string{"r1", "f2", "r2", "f1"}) {
		t.Fatalf("block order = %v", got)
	}
}

func TestApplyOrderRejectsAnythingButTheExactSet(t *testing.T) {
	stored := ss("a", "b", "c")
	if next, ok := applyOrder(stored, []string{"c", "a", "b"}); !ok || idList(next) != "c,a,b" {
		t.Fatalf("applied = %s, %v", idList(next), ok)
	}
	for _, bad := range [][]string{{"a", "b"}, {"a", "b", "b"}, {"a", "b", "x"}, {"a", "b", "c", "d"}} {
		if _, ok := applyOrder(stored, bad); ok {
			t.Errorf("applyOrder accepted %v", bad)
		}
	}
}

func idList(sessions []lichclient.Session) string {
	return joinBy(sessions, func(s lichclient.Session) string { return s.ID })
}

func folderList(sessions []lichclient.Session) string {
	return joinBy(sessions, func(s lichclient.Session) string { return s.Folder })
}

func joinBy(sessions []lichclient.Session, of func(lichclient.Session) string) string {
	out := ""
	for i, s := range sessions {
		if i > 0 {
			out += ","
		}
		out += of(s)
	}
	return out
}
