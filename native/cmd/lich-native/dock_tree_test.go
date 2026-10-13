package main

import (
	"context"
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/ui"
)

// shape spells a tree as "name/ (children)" for dirs and "name" for files.
func shape(nodes []*treeNode) string {
	var parts []string
	for _, n := range nodes {
		if n.dir {
			parts = append(parts, n.name+"/("+shape(n.children)+")")
			continue
		}
		parts = append(parts, n.name)
	}
	return strings.Join(parts, " ")
}

func TestBuildTreePutsFoldersFirstAndSortsWithoutCase(t *testing.T) {
	got := shape(buildTree([]string{"b.go", "Zeta/x.go", "alpha/y.go", "alpha/z.go", "A.md"}))
	want := "alpha/(y.go z.go) Zeta/(x.go) A.md b.go"
	if got != want {
		t.Errorf("tree = %q, want %q", got, want)
	}
}

func TestBuildTreeCollapsesAFolderChainKeepingTheDeepestPath(t *testing.T) {
	tree := buildTree([]string{"src/main/java/App.java", "src/main/java/Util.java", "README.md"})
	if got := shape(tree); got != "src/main/java/(App.java Util.java) README.md" {
		t.Fatalf("tree = %q", got)
	}
	if tree[0].path != "src/main/java" {
		t.Errorf("collapsed row's path = %q, want the deepest folder's", tree[0].path)
	}
}

func TestBuildTreeStopsCollapsingWhereAFolderBranches(t *testing.T) {
	got := shape(buildTree([]string{"a/b/one.go", "a/b/c/two.go"}))
	if got != "a/b/(c/(two.go) one.go)" {
		t.Errorf("tree = %q", got)
	}
}

func TestFilterPathsMatchesEveryTokenAnywhereInThePath(t *testing.T) {
	paths := []string{"frontend/src/App.tsx", "native/cmd/main.go", "frontend/src/main.tsx"}
	got := filterPaths(paths, "front MAIN")
	if !reflect.DeepEqual(got, []string{"frontend/src/main.tsx"}) {
		t.Errorf("filter = %v", got)
	}
	if got := filterPaths(paths, "  "); len(got) != len(paths) {
		t.Errorf("a blank filter kept %d of %d", len(got), len(paths))
	}
}

func TestWithDirStatsSumsEveryFolderAboveAChange(t *testing.T) {
	tree := buildTree([]string{"a/b/x.go", "a/c/y.go", "a/c/z.go", "top.go"})
	got := withDirStats(tree, map[string]lineDelta{"a/b/x.go": {1, 2}, "a/c/y.go": {3, 0}})
	want := map[string]lineDelta{"a/b/x.go": {1, 2}, "a/c/y.go": {3, 0}, "a/b": {1, 2}, "a/c": {3, 0}, "a": {4, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stats = %v, want %v", got, want)
	}
}

func TestVisibleRowsShowAFoldersChildrenOnlyWhileOpen(t *testing.T) {
	tree := buildTree([]string{"a/x.go", "a/y.go", "b/z.go"})
	names := func(rows []treeRow) (out []string) {
		for _, r := range rows {
			out = append(out, strings.Repeat(" ", r.depth)+r.node.name)
		}
		return out
	}
	closed := visibleRows(tree, func(string) bool { return false })
	if got := names(closed); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("closed rows = %q", got)
	}
	open := visibleRows(tree, func(p string) bool { return p == "a" })
	if got := names(open); !reflect.DeepEqual(got, []string{"a", " x.go", " y.go", "b"}) {
		t.Errorf("rows with a open = %q", got)
	}
}

func TestTreeFootnoteNamesTheHiddenBeforeTheCut(t *testing.T) {
	if got := treeFootnote(false, nil); got != "" {
		t.Errorf("a whole listing owes %q", got)
	}
	got := treeFootnote(true, []string{"build", "node_modules"})
	want := "Hidden: build, node_modules. This folder has more files than the tree can list."
	if got != want {
		t.Errorf("footnote = %q, want %q", got, want)
	}
}

func TestRemoteKeepsTheLastAnswerUntilTheNextLandsButDropsItAcrossAReset(t *testing.T) {
	var r remote[string]
	answered := make(chan struct{})
	r.invalidate = func() { answered <- struct{}{} }
	r.want("k1", "repo", func(context.Context) (string, error) { return "one", nil })
	<-answered
	block := make(chan struct{})
	r.want("k2", "repo", func(context.Context) (string, error) { <-block; return "two", nil })
	if data, loading, _ := r.get(); data != "one" || loading {
		t.Errorf("during a refetch = %q loading=%v, want the last answer", data, loading)
	}
	r.want("k3", "other", func(context.Context) (string, error) { <-block; return "three", nil })
	if data, loading, _ := r.get(); data != "" || !loading {
		t.Errorf("after a reset = %q loading=%v, want nothing and loading", data, loading)
	}
	close(block)
	<-answered
	if data, _, _ := r.get(); data != "three" {
		t.Errorf("answer = %q, want the latest key's", data)
	}
}

func TestTreeScrollsSidewaysByItsWidestRow(t *testing.T) {
	var rows treeRows
	rows.list.Axis = layout.Vertical
	rows.tree = buildTree([]string{"src/" + strings.Repeat("a", 120) + ".go", "b.go"})
	rows.stats = map[string]lineDelta{"b.go": {added: 3}}
	visible := visibleRows(rows.tree, func(string) bool { return true })
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(200, 300))}
	for range 2 {
		ops.Reset()
		rows.layout(gtx, th, visible, "")
	}
	if rows.widest <= 200 || rows.widest > 2000 {
		t.Fatalf("widest row = %dpx, want the long name's own width, past the 200px column", rows.widest)
	}
}

func TestTreeRowsStopShortOfTheVerticalBar(t *testing.T) {
	paths := []string{"src/" + strings.Repeat("a", 120) + ".go"}
	for i := range 60 {
		paths = append(paths, fmt.Sprintf("f%02d.go", i))
	}
	var rows treeRows
	rows.list.Axis = layout.Vertical
	rows.tree = buildTree(paths)
	visible := visibleRows(rows.tree, func(string) bool { return true })
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(200, 300))}
	for range 3 {
		ops.Reset()
		rows.offsetX = 1 << 20
		rows.layout(gtx, th, visible, "")
	}
	bar := gtx.Dp(th.ScrollArea(&rows.list).ScrollbarStyle.Width())
	if want := rows.widest - (200 - bar); rows.offsetX != want {
		t.Errorf("scrolled to the end at %dpx, want %dpx: the last %dpx of the widest row hide under the vertical bar", rows.offsetX, want, bar)
	}
}
