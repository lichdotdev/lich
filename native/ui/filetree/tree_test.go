package filetree

import (
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

	"github.com/lichdotdev/lich/native/ui"
)

// shape spells a tree as "name/ (children)" for dirs and "name" for files.

func shape(nodes []*Node) string {
	var parts []string
	for _, n := range nodes {
		if n.Dir {
			parts = append(parts, n.Name+"/("+shape(n.Children)+")")
			continue
		}
		parts = append(parts, n.Name)
	}
	return strings.Join(parts, " ")
}

func TestBuildTreePutsFoldersFirstAndSortsWithoutCase(t *testing.T) {
	got := shape(Build([]string{"b.go", "Zeta/x.go", "alpha/y.go", "alpha/z.go", "A.md"}))
	want := "alpha/(y.go z.go) Zeta/(x.go) A.md b.go"
	if got != want {
		t.Errorf("tree = %q, want %q", got, want)
	}
}

func TestBuildTreeCollapsesAFolderChainKeepingTheDeepestPath(t *testing.T) {
	tree := Build([]string{"src/main/java/App.java", "src/main/java/Util.java", "README.md"})
	if got := shape(tree); got != "src/main/java/(App.java Util.java) README.md" {
		t.Fatalf("tree = %q", got)
	}
	if tree[0].Path != "src/main/java" {
		t.Errorf("collapsed row's path = %q, want the deepest folder's", tree[0].Path)
	}
}

func TestBuildTreeStopsCollapsingWhereAFolderBranches(t *testing.T) {
	got := shape(Build([]string{"a/b/one.go", "a/b/c/two.go"}))
	if got != "a/b/(c/(two.go) one.go)" {
		t.Errorf("tree = %q", got)
	}
}

func TestWithDirStatsSumsEveryFolderAboveAChange(t *testing.T) {
	tree := Build([]string{"a/b/x.go", "a/c/y.go", "a/c/z.go", "top.go"})
	got := WithDirStats(tree, map[string]Delta{"a/b/x.go": {1, 2}, "a/c/y.go": {3, 0}})
	want := map[string]Delta{"a/b/x.go": {1, 2}, "a/c/y.go": {3, 0}, "a/b": {1, 2}, "a/c": {3, 0}, "a": {4, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stats = %v, want %v", got, want)
	}
}

func TestVisibleRowsShowAFoldersChildrenOnlyWhileOpen(t *testing.T) {
	tree := Build([]string{"a/x.go", "a/y.go", "b/z.go"})
	names := func(rows []Row) (out []string) {
		for _, r := range rows {
			out = append(out, strings.Repeat(" ", r.Depth)+r.Node.Name)
		}
		return out
	}
	closed := VisibleRows(tree, func(string) bool { return false })
	if got := names(closed); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("closed rows = %q", got)
	}
	open := VisibleRows(tree, func(p string) bool { return p == "a" })
	if got := names(open); !reflect.DeepEqual(got, []string{"a", " x.go", " y.go", "b"}) {
		t.Errorf("rows with a open = %q", got)
	}
}

func TestTreeScrollsSidewaysByItsWidestRow(t *testing.T) {
	var rows Rows
	rows.List.Axis = layout.Vertical
	rows.Tree = Build([]string{"src/" + strings.Repeat("a", 120) + ".go", "b.go"})
	rows.Stats = map[string]Delta{"b.go": {Added: 3}}
	visible := VisibleRows(rows.Tree, func(string) bool { return true })
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(200, 300))}
	for range 2 {
		ops.Reset()
		rows.Layout(gtx, th, visible, "")
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
	var rows Rows
	rows.List.Axis = layout.Vertical
	rows.Tree = Build(paths)
	visible := VisibleRows(rows.Tree, func(string) bool { return true })
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(200, 300))}
	for range 3 {
		ops.Reset()
		rows.offsetX = 1 << 20
		rows.Layout(gtx, th, visible, "")
	}
	bar := gtx.Dp(th.ScrollArea(&rows.List).ScrollbarStyle.Width())
	if want := rows.widest - (200 - bar); rows.offsetX != want {
		t.Errorf("scrolled to the end at %dpx, want %dpx: the last %dpx of the widest row hide under the vertical bar", rows.offsetX, want, bar)
	}
}

func BenchmarkBuildTree(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 50_000} {
		for _, shape := range []string{"wide", "distributed", "deep"} {
			b.Run(fmt.Sprintf("%d/%s", size, shape), func(b *testing.B) {
				paths := benchmarkTreePaths(size, shape)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					Build(paths)
				}
			})
		}
	}
}

func benchmarkTreePaths(size int, shape string) []string {
	paths := make([]string, size)
	for i := range size {
		switch shape {
		case "wide":
			paths[i] = fmt.Sprintf("file-%05d.go", i)
		case "deep":
			paths[i] = fmt.Sprintf("root/%s/file-%05d.go", strings.Repeat("level/", 12), i)
		default:
			paths[i] = fmt.Sprintf("group-%03d/sub-%03d/file-%05d.go", i%100, i%1_000, i)
		}
	}
	return paths
}
