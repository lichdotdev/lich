package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

const sidebarProject = "/home/me/try/repo"

// sidebarModel is a model on project "p" at sidebarProject holding stored, the
// last session active, backed by a fake settings backend.
func sidebarModel(t *testing.T, stored []lichclient.Session) (*model, *sidebarBackend) {
	t.Helper()
	f, c := newSidebarBackend(t)
	m := newModel(c, func() {})
	m.projects = []lichclient.Project{{ID: "p", Path: sidebarProject, Sessions: stored}}
	m.activeProject, m.active = "p", stored[len(stored)-1].ID
	return m, f
}

func reportStatus(m *model, id, state string) {
	send(m, statusEventName, statusEvent{ID: id, State: state})
}

func blockKeys(v sidebarView) []string {
	var keys []string
	for _, b := range v.blocks {
		keys = append(keys, b.key)
	}
	return keys
}

func TestSidebarBlocksTitleEachKindAndHideHeadersForOneBlock(t *testing.T) {
	m, _ := sidebarModel(t, ss("s1"))
	if v := m.sidebarBlocks(sidebarFilter{}); v.showHeader || len(v.blocks) != 1 || v.blocks[0].title != "repo" {
		t.Fatalf("lone block: %+v", v)
	}
	stored := pin(filed(ss("s1", "w1@/x/worktrees/p/feat/a", "w2@/x/worktrees/p/feat/a"), "Apps", "w1"), "s1")
	stored = with(stored, "w1", func(s *lichclient.Session) { s.Color = "teal" })
	m, _ = sidebarModel(t, stored)
	v := m.sidebarBlocks(sidebarFilter{})
	titles := []string{}
	for _, b := range v.blocks {
		titles = append(titles, b.title)
	}
	if !v.showHeader || !slices.Equal(titles, []string{"Pinned", "Apps", "feat/a"}) {
		t.Fatalf("titles = %v (header %v)", titles, v.showHeader)
	}
	folder := v.blocks[1]
	if folder.color != "teal" || len(folder.checkouts) != 1 || folder.checkouts[0] != (launchCheckout{path: "/x/worktrees/p/feat/a", label: "feat/a"}) {
		t.Fatalf("folder block = %+v", folder)
	}
	if !slices.Equal(v.folders, []string{"Apps"}) {
		t.Fatalf("folders = %v", v.folders)
	}
}

func TestSidebarBlocksKeepTheHeaderOfALoneWorktree(t *testing.T) {
	m, _ := sidebarModel(t, ss("w1@/x/worktrees/p/feat/a"))
	if v := m.sidebarBlocks(sidebarFilter{}); !v.showHeader || len(v.blocks) != 1 || v.blocks[0].title != "feat/a" {
		t.Fatalf("lone worktree: %+v", v)
	}
}

func TestSidebarBlocksFilterKeepsTheActiveCardAndSaysWhy(t *testing.T) {
	m, _ := sidebarModel(t, ss("Relay a", "Docs", "Shell"))
	reportStatus(m, "Docs", "busy")
	v := m.sidebarBlocks(sidebarFilter{query: "webgl"})
	if v.matched || !v.showHeader || v.notice != "No sessions match “webgl”. The active session stays." {
		t.Fatalf("no-match view: %+v", v)
	}
	if len(v.blocks) != 1 || len(v.blocks[0].sessions) != 1 || v.blocks[0].sessions[0].ID != "Shell" {
		t.Fatalf("kept %+v", v.blocks)
	}
	v = m.sidebarBlocks(sidebarFilter{phases: picked(phaseRunning)})
	if v.blocks[0].sessions[0].ID != "Docs" || v.counts[phaseRunning] != 1 || v.counts[phaseIdle] != 2 || v.notice != "" {
		t.Fatalf("running view: %+v", v)
	}
}

func TestFoldIsReadStoredAndMarked(t *testing.T) {
	ctx := context.Background()
	m, f := sidebarModel(t, ss("s1", "w1@/wt/a"))
	f.settings["lich.sidebar.collapsed./wt/a@p"] = "true"
	if err := m.loadFolds(ctx); err != nil {
		t.Fatal(err)
	}
	reportStatus(m, "w1", "waiting")
	v := m.sidebarBlocks(sidebarFilter{})
	if b := v.blocks[1]; !b.collapsed || !b.folded || b.mark != markWait {
		t.Fatalf("folded block = %+v", b)
	}
	if b := v.blocks[0]; b.collapsed || b.mark != markNone {
		t.Fatalf("open block = %+v", b)
	}
	if err := m.toggleFold(ctx, rootBlockKey); err != nil {
		t.Fatal(err)
	}
	if f.get("lich.sidebar.collapsed."+rootBlockKey+"@p") != "true" || !m.sidebarBlocks(sidebarFilter{}).blocks[0].collapsed {
		t.Fatal("toggle did not fold and store the root block")
	}
}

// A block folded beside a sibling that has since closed is drawn alone and
// header-less, so it draws open; the stored fold stays for when a header is
// back.
func TestALoneBlockDrawsOpenButKeepsItsFold(t *testing.T) {
	m, _ := sidebarModel(t, ss("s1"))
	_ = m.toggleFold(context.Background(), rootBlockKey)
	if b := m.sidebarBlocks(sidebarFilter{}).blocks[0]; !b.collapsed || b.folded {
		t.Fatalf("lone block = %+v", b)
	}
}

func TestCommitDragOrderStoresTheWholeList(t *testing.T) {
	m, f := sidebarModel(t, pin(ss("p1", "r1", "a1@/wt/a"), "p1"))
	if err := m.commitDragOrder(context.Background(), []string{"/wt/a", rootBlockKey}); err != nil {
		t.Fatal(err)
	}
	p, _ := m.project()
	if idList(p.Sessions) != "p1,a1,r1" || !slices.Equal(f.calls, []string{`store.ReorderSessions("p",["p1","a1","r1"])`}) {
		t.Fatalf("order %s, calls %v", idList(p.Sessions), f.calls)
	}
}

func TestCommitBlockOrderDropsAStaleOrderSilently(t *testing.T) {
	m, f := sidebarModel(t, filed(ss("r1", "f1", "r2", "f2"), "Apps", "f1", "f2"))
	ctx := context.Background()
	if err := m.commitBlockOrder(ctx, folderKey("Apps"), []string{"f2", "f1"}); err != nil {
		t.Fatal(err)
	}
	// f2 closed mid-drag: the order names the block short, so nothing is sent.
	if err := m.commitBlockOrder(ctx, folderKey("Apps"), []string{"f1"}); err != nil {
		t.Fatal(err)
	}
	p, _ := m.project()
	if idList(p.Sessions) != "r1,f2,r2,f1" || len(f.calls) != 1 {
		t.Fatalf("order %s, calls %v", idList(p.Sessions), f.calls)
	}
}

func TestFileSessionsChangesTheModelThenTheStore(t *testing.T) {
	m, f := sidebarModel(t, ss("s1", "s2"))
	if err := m.fileSessions(context.Background(), []string{"s1", "ghost"}, "Apps"); err != nil {
		t.Fatal(err)
	}
	p, _ := m.project()
	want := []string{`store.SetSessionFolder("s1","Apps")`, `store.SetSessionFolder("ghost","Apps")`}
	if folderList(p.Sessions) != "Apps," || !slices.Equal(f.calls, want) {
		t.Fatalf("folders %s, calls %v", folderList(p.Sessions), f.calls)
	}
}

func TestRenameFolderMovesTheFoldAndUngroupDropsIt(t *testing.T) {
	ctx := context.Background()
	m, f := sidebarModel(t, filed(ss("s1", "s2"), "Apps", "s1"))
	_ = m.toggleFold(ctx, folderKey("Apps"))
	if err := m.renameFolder(ctx, "Apps", "Ops"); err != nil {
		t.Fatal(err)
	}
	v := m.sidebarBlocks(sidebarFilter{})
	if !slices.Equal(blockKeys(v), []string{folderKey("Ops"), rootBlockKey}) || !v.blocks[0].collapsed {
		t.Fatalf("after rename: %v %+v", blockKeys(v), v.blocks[0])
	}
	if f.get("lich.sidebar.collapsed.__folder__:Ops@p") != "true" || f.get("lich.sidebar.collapsed.__folder__:Apps@p") != "" {
		t.Fatalf("stored folds %v", f.settings)
	}
	if err := m.ungroupFolder(ctx, "Ops"); err != nil {
		t.Fatal(err)
	}
	want := []string{`store.RenameFolder("p","Apps","Ops")`, `store.RenameFolder("p","Ops","")`}
	if got := m.sidebarBlocks(sidebarFilter{}); !slices.Equal(blockKeys(got), []string{rootBlockKey}) || !slices.Equal(f.calls, want) {
		t.Fatalf("after ungroup: %v, calls %v", blockKeys(got), f.calls)
	}
	if f.get("lich.sidebar.collapsed.__folder__:Ops@p") != "" {
		t.Fatal("ungroup kept the fold")
	}
}

func TestBlockViewsCarryTheCardsLiveState(t *testing.T) {
	m, _ := sidebarModel(t, ss("s1"))
	reportStatus(m, "s1", "busy")
	sv := m.sidebarBlocks(sidebarFilter{}).blocks[0].sessions[0]
	if sv.status != statusBusy || !sv.aged || sv.path != sidebarProject || sv.age > time.Minute {
		t.Fatalf("card view = %+v", sv)
	}
}
