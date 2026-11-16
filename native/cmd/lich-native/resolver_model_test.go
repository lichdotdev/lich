package main

import (
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

// conflictFixture: ours added "start" on top and rewrote line 3; theirs
// rewrote line 3 too and added "end".
func conflictFixture() api.GitConflict {
	return api.GitConflict{
		Base:   "a\nb\nc\nd\n",
		Ours:   "start\na\nb\nOURS\nd\n",
		Theirs: "a\nb\nTHEIRS\nd\nend\n",
		Chunks: []api.GitConflictChunk{
			{Kind: api.GitChunkOurs, Base: api.GitLines{}, Ours: api.GitLines{To: 1}, Theirs: api.GitLines{}},
			{Kind: api.GitChunkConflict, Base: api.GitLines{From: 2, To: 3}, Ours: api.GitLines{From: 3, To: 4}, Theirs: api.GitLines{From: 2, To: 3}},
			{Kind: api.GitChunkTheirs, Base: api.GitLines{From: 4, To: 4}, Ours: api.GitLines{From: 5, To: 5}, Theirs: api.GitLines{From: 4, To: 5}},
		},
	}
}

// The result opens as the base, nothing taken: a change only one side made
// waits to be applied like a conflict does (it used to be taken on open).
func TestResolveModelStartsFromTheBase(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	if want := "a\nb\nc\nd\n"; doc != want {
		t.Fatalf("doc = %q, want the base %q", doc, want)
	}
	if got := doc[m.chunks[1].from:m.chunks[1].to]; got != "c\n" {
		t.Errorf("conflict holds %q, want the base's c", got)
	}
	if conflicts, changes := m.unresolved(doc); conflicts != 1 || changes != 2 {
		t.Errorf("unresolved = %d conflicts, %d changes; want 1 and 2", conflicts, changes)
	}
}

func TestResolveModelOneSidedChange(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	if m.state(0, doc) != chunkPending {
		t.Fatal("a change only ours made is taken before the user takes it")
	}
	c := m.take(0, true, doc)
	taken := doc[:c.From] + c.Insert + doc[c.To:]
	if !strings.HasPrefix(taken, "start\na\n") {
		t.Errorf("after taking ours' line the result starts %q", taken)
	}
	m.chunks[2].theirsOff = true
	if m.state(2, doc) != chunkDismissed {
		t.Error("a change only theirs made is not left out by its own dismiss")
	}
	m.chunks[0].theirsOff = true
	if m.state(0, doc) != chunkPending {
		t.Error("the side that did not make a change dismissed it")
	}
}

func TestTakeChangesLeavesConflictsAlone(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	r := &conflictResolver{model: m}
	got := r.takeChanges(doc)
	if len(got) != 2 || got[0].Insert != "start\n" || got[1].Insert != "end\n" {
		t.Errorf("takeChanges = %+v, want ours' start and theirs' end", got)
	}
	if all := r.takeAll(true, doc); len(all) != 1 || all[0].Insert != "OURS\n" {
		t.Errorf("takeAll = %+v, want the conflict alone", all)
	}
}

func TestWordSpan(t *testing.T) {
	t.Parallel()
	fill := color.NRGBA{A: 255}
	cases := []struct {
		line     string
		others   []string
		from, to int
	}{
		{`TITLE = "a"` + "\n", []string{`TITLE = "post"` + "\n"}, 9, 10},
		{`TITLE = "a"`, []string{`TITLE = "post"`, `TITLE = "untitled"`}, 9, 10},
		{`x = 10`, []string{`x = 50`, `y = 10`}, 0, 5}, // from the first difference of any to the last
		{`same`, []string{`same`}, 0, 0},
		{`abc`, []string{`xyz`}, 0, 0}, // the whole line differs
		{`é = 1`, []string{`é = 2`}, 4, 5},
	}
	for _, c := range cases {
		got := wordSpan(c.line, c.others, fill)
		if c.from == c.to {
			if got != nil {
				t.Errorf("wordSpan(%q) = %+v, want none", c.line, got)
			}
			continue
		}
		if len(got) != 1 || got[0].From != c.from || got[0].To != c.to {
			t.Errorf("wordSpan(%q) = %+v, want %d..%d", c.line, got, c.from, c.to)
		}
	}
}

func TestResolveModelTakeStacksSides(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	apply := func(c godemirror.ChangeSpec) {
		doc = doc[:c.From] + c.Insert + doc[c.To:]
		m.chunks[1].to = c.From + len(c.Insert)
	}
	apply(m.take(1, true, doc))
	if got := doc[m.chunks[1].from:m.chunks[1].to]; got != "OURS\n" || !m.resolved(1, doc) || !m.applied(1, true, doc) {
		t.Fatalf("after taking ours the chunk is %q", got)
	}
	apply(m.take(1, false, doc))
	if got := doc[m.chunks[1].from:m.chunks[1].to]; got != "OURS\nTHEIRS\n" || !m.applied(1, false, doc) {
		t.Errorf("after taking theirs too the chunk is %q, want both stacked", got)
	}
}

func TestResolveModelBothDismissedKeepsTheBase(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	m.chunks[1].oursOff = true
	if m.resolved(1, doc) {
		t.Fatal("one dismissed side resolved the conflict")
	}
	m.chunks[1].theirsOff = true
	if !m.resolved(1, doc) {
		t.Error("both sides dismissed should keep the base and resolve it")
	}
}

func TestResolveModelKeepsAMissingFinalNewline(t *testing.T) {
	t.Parallel()
	c := api.GitConflict{Base: "a", Ours: "b", Theirs: "c",
		Chunks: []api.GitConflictChunk{{Kind: api.GitChunkConflict, Base: api.GitLines{To: 1}, Ours: api.GitLines{To: 1}, Theirs: api.GitLines{To: 1}}}}
	if _, doc := newResolveModel(c); doc != "a" {
		t.Errorf("doc = %q, want %q", doc, "a")
	}
}

func TestMapLine(t *testing.T) {
	t.Parallel()
	src := []api.GitLines{{From: 2, To: 4}, {From: 10, To: 10}}
	dst := []api.GitLines{{From: 2, To: 3}, {From: 9, To: 12}}
	cases := map[int]int{0: 0, 1: 1, 2: 2, 3: 2, 4: 3, 9: 8, 10: 12, 15: 17}
	for line, want := range cases {
		if got := mapLine(line, src, dst); got != want {
			t.Errorf("mapLine(%d) = %d, want %d", line, got, want)
		}
	}
}

// TestResolveModelFollowsEdits runs the chunks through a real editor's
// changes: text typed inside a conflict is the conflict's, and resolves it.
func TestResolveModelFollowsEdits(t *testing.T) {
	m, doc := newResolveModel(conflictFixture())
	ed, err := codeview.NewTextEditor(fonts.SystemMono, 13)
	if err != nil {
		t.Skipf("no editor font: %v", err)
	}
	if err := ed.SetState(godemirror.EditorStateConfig{Doc: doc}); err != nil {
		t.Fatal(err)
	}
	var changes []godemirror.ChangeSet
	ed.OnUpdate = func(u godemirror.Update) { changes = append(changes, u.Changes) }
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300))}
		if _, err := ed.Layout(gtx); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	c := m.chunks[1]
	before := []resolveChunk{m.chunks[0], m.chunks[2]}
	if err := ed.Dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{{From: c.from, To: c.from, Insert: "x"}}}); err != nil {
		t.Fatal(err)
	}
	frame()
	if len(changes) != 1 {
		t.Fatalf("got %d updates, want 1", len(changes))
	}
	m.remap(changes[0])
	doc = ed.State().Doc()
	if got := doc[m.chunks[1].from:m.chunks[1].to]; got != "xc\n" || !m.resolved(1, doc) {
		t.Errorf("conflict after typing at its start = %q, want xc and resolved", got)
	}
	if m.chunks[0].from != before[0].from || m.chunks[2].from != before[1].from+1 {
		t.Errorf("the other chunks did not follow: %+v", m.chunks)
	}
	if !reflect.DeepEqual(m.chunks[0].GitConflictChunk, before[0].GitConflictChunk) {
		t.Error("remap changed a chunk's lines")
	}
}

// TestResolverFirstFrame lays the resolver out in the frames after Resolve:
// the gutters place their buttons from the panes' geometry, so a conflict
// below the first line must find the pane laid out with its own text, not
// the empty one it was built with.
func TestResolverFirstFrame(t *testing.T) {
	c := conflictFixture()
	c.Base, c.Ours, c.Theirs = "x\ny\nz\n"+c.Base, "x\ny\nz\n"+c.Ours, "x\ny\nz\n"+c.Theirs
	for i := range c.Chunks {
		ch := &c.Chunks[i]
		for _, r := range []*api.GitLines{&ch.Base, &ch.Ours, &ch.Theirs} {
			r.From, r.To = r.From+3, r.To+3
		}
	}
	r, err := newConflictResolver("f.txt", c, fonts.SystemMono, 13)
	if err != nil {
		t.Skipf("no editor font: %v", err)
	}
	p, err := newReviewPanel(nil, &ui.Root{}, nil, fonts.SystemMono, 13, func() {}, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	p.git = backend.DiffStats{Branch: "master", Merging: "side", Conflicted: []string{"f.txt"}}
	var router input.Router
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	for range 3 {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1200, 600)),
			Source: router.Source()}
		r.layout(gtx, th, p)
	}
	if lines := r.result.State().Decorations().Lines; len(lines) == 0 {
		t.Error("the result shows its chunks untinted until the first edit")
	}
}

// TestGutterButtonsKeepTheirOwnHeight: a gutter is as tall as the panes, and
// a button handed that height as its minimum centers itself in it, far from
// the chunk it stands for.
func TestGutterButtonsKeepTheirOwnHeight(t *testing.T) {
	t.Parallel()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Constraints: layout.Exact(image.Pt(56, 500))}
	var got image.Point
	offset(gtx, 40, func(gtx C) D { got = gtx.Constraints.Min; return D{} })
	if got != (image.Point{}) {
		t.Errorf("button laid out with minimum %v, want none", got)
	}
}

// TestSideTintsFollowTheResult: a conflict is waiting on both sides until
// the result settles it, then passes there too.
func TestSideTintsFollowTheResult(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	r := &conflictResolver{model: m}
	th := &ui.Theme{Palette: ui.Dark, Surface: ui.Dark.Sidebar}
	wait := ui.MixOKLab(th.Surface, th.ToneWait, resolverTint)
	pass := ui.MixOKLab(th.Surface, th.TonePass, resolverTint)
	// A change of its own is neutral until taken, now that none is on open.
	neutral := ui.MixOKLab(th.Surface, th.MutedForeground, resolverTint)
	ours := r.sideDecorations(th, true, doc)
	if ours[3].Background != wait || ours[0].Background != neutral || ours[1].Background != (color.NRGBA{}) {
		t.Fatalf("open conflict: line 3 %v, own change %v, untouched %v", ours[3].Background, ours[0].Background, ours[1].Background)
	}
	c := m.take(1, true, doc)
	doc = doc[:c.From] + c.Insert + doc[c.To:]
	m.chunks[1].to = c.From + len(c.Insert)
	if got := r.sideDecorations(th, false, doc)[2].Background; got != pass {
		t.Errorf("settled conflict on the incoming side = %v, want the passing tint", got)
	}
}

func TestRunSpan(t *testing.T) {
	t.Parallel()
	block := func(line int) (int, int) { return line * 20, line*20 + 20 }
	cases := []struct {
		run         api.GitLines
		count       int
		top, bottom int
	}{
		{api.GitLines{From: 2, To: 4}, 10, 40, 80},
		{api.GitLines{From: 3, To: 3}, 10, 60, 60},     // the seam before line 3
		{api.GitLines{From: 10, To: 10}, 10, 200, 200}, // past the last line
		{api.GitLines{From: 8, To: 12}, 10, 160, 200},  // clipped to the pane
		{api.GitLines{From: 0, To: 1}, 0, 0, 0},
	}
	for _, c := range cases {
		if top, bottom := runSpan(c.run, c.count, block); top != c.top || bottom != c.bottom {
			t.Errorf("runSpan(%+v, %d) = %d, %d; want %d, %d", c.run, c.count, top, bottom, c.top, c.bottom)
		}
	}
}

func TestBandOnSide(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind         string
		ours, theirs bool
	}{
		{api.GitChunkOurs, true, false},
		{api.GitChunkTheirs, false, true},
		{api.GitChunkBoth, true, true},
		{api.GitChunkConflict, true, true},
	}
	for _, c := range cases {
		if bandOnSide(c.kind, true) != c.ours || bandOnSide(c.kind, false) != c.theirs {
			t.Errorf("%s: bands to ours %v, theirs %v; want %v, %v", c.kind, bandOnSide(c.kind, true), bandOnSide(c.kind, false), c.ours, c.theirs)
		}
	}
}

// TestDismissIsUndone: ✕ steps back with the result's own undo, and comes
// back with its redo, until a new change drops that redo.
func TestDismissIsUndone(t *testing.T) {
	r, err := newConflictResolver("f.txt", conflictFixture(), fonts.SystemMono, 13)
	if err != nil {
		t.Skipf("no editor font: %v", err)
	}
	dismissed := func() bool { return r.model.chunks[1].oursOff }
	r.dismiss(1, true)
	if !dismissed() {
		t.Fatal("✕ did not leave the side out")
	}
	if !r.result.Undo() {
		t.Fatal("nothing to undo after ✕")
	}
	r.syncDismissals()
	if dismissed() {
		t.Fatal("undo did not bring the side back")
	}
	r.result.Redo()
	r.syncDismissals()
	if !dismissed() {
		t.Fatal("redo did not leave the side out again")
	}
	r.result.Undo()
	r.depth = r.result.UndoDepth()
	r.dismiss(2, false)
	r.result.Undo()
	r.result.Undo()
	r.result.Redo()
	r.syncDismissals()
	if dismissed() || !r.model.chunks[2].theirsOff {
		t.Errorf("after a new ✕ the undone one came back: ours off %v, theirs off %v", dismissed(), r.model.chunks[2].theirsOff)
	}
	if r.doc() != "a\nb\nc\nd\n" {
		t.Errorf("dismissing changed the text: %q", r.doc())
	}
}

func TestNeighborWalksThePendingChunks(t *testing.T) {
	t.Parallel()
	m, doc := newResolveModel(conflictFixture())
	// Chunks start at 0 (ours' line), 4 (the conflict) and 8 (theirs' line).
	steps := []struct {
		pos, dir, want int
		ok             bool
	}{
		{0, 1, 1, true}, {4, 1, 2, true}, {8, 1, 0, false},
		{8, -1, 1, true}, {4, -1, 0, true}, {0, -1, 0, false},
	}
	for _, s := range steps {
		if got, ok := m.neighbor(s.pos, s.dir, doc); ok != s.ok || ok && got != s.want {
			t.Errorf("neighbor(%d, %d) = %d, %v; want %d, %v", s.pos, s.dir, got, ok, s.want, s.ok)
		}
	}
	m.chunks[1].oursOff, m.chunks[1].theirsOff = true, true
	if got, ok := m.neighbor(0, 1, doc); !ok || got != 2 {
		t.Errorf("neighbor skips nothing settled: got %d, %v; want 2", got, ok)
	}
}
