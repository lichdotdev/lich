package main

import (
	"reflect"
	"strings"
	"testing"
)

// paneShape spells a tree as row(a,column(b,c)), the ratio left out.
func paneShape(n *paneNode) string {
	if n == nil {
		return "<nil>"
	}
	if n.leaf() {
		return n.Session
	}
	return string(n.Axis) + "(" + paneShape(n.First) + "," + paneShape(n.Second) + ")"
}

func TestSplitPutsTheSessionAgainstTheEdge(t *testing.T) {
	cases := []struct {
		edge paneEdge
		want string
	}{
		{edgeRight, "row(a,b)"},
		{edgeLeft, "row(b,a)"},
		{edgeBottom, "column(a,b)"},
		{edgeTop, "column(b,a)"},
		{edgeCenter, "b"},
	}
	for _, c := range cases {
		tab := layoutTab{Root: &paneNode{Session: "a"}}
		if !tab.split("a", "b", c.edge) {
			t.Fatalf("edge %d: split reported a missing target", c.edge)
		}
		if got := paneShape(tab.Root); got != c.want {
			t.Errorf("edge %d: got %s, want %s", c.edge, got, c.want)
		}
	}
}

func TestSplitNestsInsideTheTargetPaneOnly(t *testing.T) {
	var ls layoutTabs
	id := ls.open("orchestration", "orch", "w1", edgeRight)
	tab := ls.byID(id)
	tab.split("w1", "w2", edgeBottom)
	if got, want := paneShape(tab.Root), "row(orch,column(w1,w2))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if got := tab.sessions(); !reflect.DeepEqual(got, []string{"orch", "w1", "w2"}) {
		t.Fatalf("reading order %v", got)
	}
	if tab.split("missing", "w3", edgeRight) {
		t.Fatal("split into a session the tab does not hold succeeded")
	}
}

func TestRemoveCollapsesTheParentAndMovesFocus(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.open("x", "a", "b", edgeRight))
	tab.split("b", "c", edgeBottom)
	tab.Focus = "b"
	if !tab.remove("b") {
		t.Fatal("tab reported empty with two panes left")
	}
	if got, want := paneShape(tab.Root), "row(a,c)"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if tab.Focus != "a" {
		t.Fatalf("focus %q, want the first pane left", tab.Focus)
	}
}

func TestTakeDissolvesALayoutLeftWithOnePane(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.open("x", "a", "b", edgeRight))
	tab.split("b", "c", edgeBottom)
	if left, ok := ls.take("c"); !ok || left != "" || len(ls.tabs) != 1 {
		t.Fatalf("two panes left: left %q ok %v tabs %+v, want the layout kept", left, ok, ls.tabs)
	}
	if left, ok := ls.take("a"); !ok || left != "b" || len(ls.tabs) != 0 {
		t.Fatalf("one pane left: left %q ok %v tabs %+v, want the layout gone and b alone", left, ok, ls.tabs)
	}
	if _, ok := ls.take("b"); ok {
		t.Fatal("taking a session no layout holds reported a change")
	}
}

func TestPruneDropsSessionsThatAreGone(t *testing.T) {
	var ls layoutTabs
	ls.open("x", "a", "b", edgeRight)
	ls.open("y", "c", "d", edgeRight)
	ls.byID(ls.tabs[1].ID).split("d", "e", edgeBottom)
	live := map[string]bool{"a": true, "c": true, "e": true}
	if !ls.prune(func(id string) bool { return live[id] }) {
		t.Fatal("prune reported no change")
	}
	if len(ls.tabs) != 1 || paneShape(ls.tabs[0].Root) != "row(c,e)" {
		t.Fatalf("got %+v, want x gone with one pane left and y holding c and e", ls.tabs)
	}
}

func TestOfFindsTheLayoutHoldingASession(t *testing.T) {
	var ls layoutTabs
	ls.open("x", "a", "b", edgeRight)
	ls.open("y", "c", "d", edgeRight)
	if ls.of("d") != 1 || ls.of("a") != 0 || ls.of("z") != -1 {
		t.Fatalf("of: d=%d a=%d z=%d", ls.of("d"), ls.of("a"), ls.of("z"))
	}
}

func TestMinSizeAddsAlongTheAxisAndMaxesAcross(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.open("x", "a", "b", edgeRight))
	tab.split("b", "c", edgeBottom)
	w, h := minSize(tab.Root, 100, 50, 6)
	if w != 206 || h != 106 {
		t.Fatalf("got %dx%d, want 206x106", w, h)
	}
}

func TestLayoutsRoundTrip(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.open("orchestration", "a", "b", edgeRight))
	tab.split("b", "c", edgeBottom)
	tab.Zoomed = true
	raw, err := ls.marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseLayouts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.tabs, ls.tabs) {
		t.Fatalf("got %+v, want %+v", got.tabs, ls.tabs)
	}
	if id := got.open("next", "d", "e", edgeRight); id == tab.ID {
		t.Fatalf("a restored set handed out a stored id again: %s", id)
	}
}

func TestParseLayoutsRepairsWhatItCanAndRefusesBrokenTrees(t *testing.T) {
	got, err := parseLayouts(`[
		{"id":"layout-1","name":"x","root":{"axis":"row","ratio":2,"first":{"session":"a"},"second":{"session":"b"}},"focus":"gone"},
		{"id":"layout-7","name":"y","root":{"session":"a"}}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.tabs) != 1 {
		t.Fatalf("the layout whose only pane was a duplicate stayed: %+v", got.tabs)
	}
	tab := got.tabs[0]
	if tab.Root.Ratio != ratioMax || tab.Focus != "a" || got.seq != 7 {
		t.Fatalf("ratio %v focus %q seq %d", tab.Root.Ratio, tab.Focus, got.seq)
	}
	for _, raw := range []string{
		`{`,
		`[{"id":"layout-1","root":{"axis":"diagonal","first":{"session":"a"},"second":{"session":"b"}}}]`,
		`[{"id":"layout-1","root":{"axis":"row","first":{"session":"a"}}}]`,
	} {
		if _, err := parseLayouts(raw); err == nil || !strings.Contains(err.Error(), layoutsKey) {
			t.Errorf("%s: err %v, want one naming %s", raw, err, layoutsKey)
		}
	}
	if empty, err := parseLayouts(""); err != nil || len(empty.tabs) != 0 {
		t.Fatalf("unset setting: %+v, %v", empty, err)
	}
}

func TestMoveTakesThePaneOutAndSplitsTheTarget(t *testing.T) {
	cases := []struct {
		name, source, target string
		edge                 paneEdge
		want                 string
	}{
		{"side by side to stacked", "b", "a", edgeBottom, "row(column(a,b),c)"},
		{"one beside two stacked", "b", "c", edgeBottom, "row(a,column(c,b))"},
		{"swap", "a", "c", edgeCenter, "row(row(c,b),a)"},
	}
	for _, c := range cases {
		tab := layoutTab{}
		tab.Root = &paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Session: "a"}, Second: &paneNode{Session: "b"}}, Second: &paneNode{Session: "c"}}
		if !tab.move(c.source, c.target, c.edge) {
			t.Fatalf("%s: move refused", c.name)
		}
		if got := paneShape(tab.Root); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	tab := layoutTab{Root: &paneNode{Session: "a"}}
	tab.split("a", "b", edgeRight)
	if tab.move("a", "a", edgeLeft) || tab.move("a", "z", edgeLeft) {
		t.Fatal("moved onto itself or onto a pane the tab does not hold")
	}
}

func TestRotateTurnsTheFocusedSplit(t *testing.T) {
	tab := layoutTab{Root: &paneNode{Session: "a"}}
	if tab.rotate("a") {
		t.Fatal("rotated a pane alone in its tab")
	}
	tab.split("a", "b", edgeRight)
	tab.split("b", "c", edgeBottom)
	tab.rotate("c")
	if got, want := paneShape(tab.Root), "row(a,row(b,c))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	tab.rotate("a")
	if got, want := paneShape(tab.Root), "column(a,row(b,c))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestPaneHeaderNamesOnlyAWorktree(t *testing.T) {
	root := sessionView{projectID: "e48f04f46a4a", path: "/src/lich"}
	worktree := sessionView{projectID: "e48f04f46a4a", path: "/home/u/.local/share/lich/worktrees/e48f04f46a4a/imfl"}
	if got := worktreeLabel(root, "/src/lich"); got != "" {
		t.Errorf("a session in the project's own checkout is labelled %q, want none", got)
	}
	if got := worktreeLabel(worktree, "/src/lich"); got != "imfl" {
		t.Errorf("worktree label = %q, want imfl", got)
	}
}

func TestOpenBesideStacksTheDelegatesEvenlyAtTheRight(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.openBeside("orch", "orch", []string{"w1", "w2", "w3"}))
	if got, want := paneShape(tab.Root), "row(orch,column(w1,column(w2,w3)))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	column := tab.Root.Second
	if column.Ratio != 1.0/3 || column.Second.Ratio != 0.5 {
		t.Errorf("column ratios %v and %v, want each pane a third", column.Ratio, column.Second.Ratio)
	}
	if tab.Focus != "orch" {
		t.Errorf("focus %q, want the orchestrator", tab.Focus)
	}
}

func TestStackUnderSharesThePaneItJoins(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.openBeside("orch", "orch", []string{"w1"}))
	if !tab.stackUnder("w1", []string{"w2", "w3"}) {
		t.Fatal("stackUnder refused a pane the tab holds")
	}
	if got, want := paneShape(tab.Root), "row(orch,column(w1,column(w2,w3)))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if tab.stackUnder("missing", []string{"w4"}) {
		t.Fatal("stackUnder into a session the tab does not hold succeeded")
	}
}
