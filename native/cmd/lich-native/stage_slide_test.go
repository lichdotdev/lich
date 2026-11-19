package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// slideFrames is the stage row with session s's terminal on it and the dock
// collapsed on Review, the window's right gutter left free beside it.
func slideFrames(t *testing.T) (*window, *frameRig, *termView) {
	t.Helper()
	u, r := peekDockFrames(t)
	u.palette, u.worktree = new(commandPalette), new(worktreeDialog)
	u.git = newGitPanel(u.ctx, u.client, &u.root, r.invalidate, func(ui.Toast) {}, "monospace", 12)
	rend, err := gioterm.NewRenderer(gioterm.RenderRows, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := gioterm.New(80, 24, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	tv := &termView{id: "s", vt: vt, r: rend}
	tv.seeded.Store(true)
	u.terms.byID["s"], u.terms.client, u.terms.unzoomed = tv, u.client, unit.Metric{PxPerDp: 1, PxPerSp: 1}
	v := stageView{hasActive: true, active: sessionView{Session: api.Session{ID: "s", Kind: "shell"}}}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			row := gtx
			row.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-gtx.Dp(gutter), gtx.Constraints.Max.Y))
			if err := u.stageRow(row, v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r, tv
}

// slide steps frameStep at a time from a pointer move to p until s lands,
// and reports where edge has the stage on each frame s moved, and the
// terminal's columns before it and on every frame.
func slide(t *testing.T, r *frameRig, tv *termView, p f32.Point, s *dockSlide, edge func() int) (edges, cols []int) {
	t.Helper()
	c, _ := tv.vt.Size()
	cols = append(cols, c)
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p})
	for range 100 {
		r.frame()
		c, _ := tv.vt.Size()
		cols = append(cols, c)
		if s.landed {
			return edges, cols
		}
		if s.moving() {
			edges = append(edges, edge())
		}
		r.now = r.now.Add(frameStep)
	}
	t.Fatal("the slide never landed")
	return nil, nil
}

// reflows counts the frames the terminal's grid changed on, and says
// whether the last frame, the one the slide landed in, is one.
func reflows(cols []int) (n int, last bool) {
	for i := 1; i < len(cols); i++ {
		if cols[i] != cols[i-1] {
			n, last = n+1, i == len(cols)-1
		}
	}
	return n, last
}

// onCurve fails t for each frame of a slide whose edge is not at(share),
// the share of the panel out, a frameStep a frame on the slide's curve.
func onCurve(t *testing.T, way string, edges []int, out bool, at func(share float32) int) {
	t.Helper()
	if len(edges) < 5 {
		t.Fatalf("%s: the stage's edge moved on %d frames of a 180ms slide, want one a frame", way, len(edges))
	}
	for i, x := range edges {
		share := sidebarMorphCurve(float32(i*int(frameStep)) / float32(sidebarMorphDuration))
		if !out {
			share = 1 - share
		}
		if want := at(share); x != want {
			t.Errorf("%s, frame %d of the slide: the stage's edge at %d, want %d where the curve has it", way, i, x, want)
		}
	}
}

// heldThenOnce fails t unless the terminal kept its grid through the slide
// and changed it once, in the frame the slide landed.
func heldThenOnce(t *testing.T, way string, cols []int) {
	t.Helper()
	if n, last := reflows(cols); n != 1 || !last {
		t.Errorf("%s: the terminal's columns went %v, want them held through the slide and changed once, as it lands", way, cols)
	}
}

// settledCols lets the rig go still past refitSettle and answers the
// terminal's columns.
func settledCols(t *testing.T, r *frameRig, tv *termView) int {
	t.Helper()
	r.settle(t)
	r.now = r.now.Add(refitSettle)
	r.settle(t)
	c, _ := tv.vt.Size()
	return c
}

func TestDockPeekSlidesTheStageEdgeAndReflowsTheTerminalOnceItLands(t *testing.T) {
	u, r, tv := slideFrames(t)
	wide := settledCols(t, r, tv)
	// The rig draws a pixel a dp.
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	g, w := gtx.Dp(gutter), u.dock.width(gtx)
	row := rigWindow.X - g
	end := func() int { return u.dock.islands[0].x1 }
	at := func(share float32) int { return row - int(share*float32(w+g)) }
	edges, cols := slide(t, r, tv, edgePoint, &u.dock.slide, end)
	onCurve(t, "out", edges, true, at)
	heldThenOnce(t, "out", cols)
	if narrow, _ := tv.vt.Size(); narrow >= wide {
		t.Fatalf("out: %d columns beside the dock, want fewer than the %d without it", narrow, wide)
	}
	edges, cols = slide(t, r, tv, f32.Pt(100, 300), &u.dock.slide, end)
	onCurve(t, "back", edges, false, at)
	heldThenOnce(t, "back", cols)
	if c, _ := tv.vt.Size(); c != wide {
		t.Errorf("back: %d columns once the dock went in, want the %d it had before", c, wide)
	}
}

func TestSidebarPeekSlidesTheStageEdgeAndReflowsTheTerminalOnceItLands(t *testing.T) {
	u, r, tv := slideFrames(t)
	m := modelOn("a")
	v := stageView{hasActive: true, active: sessionView{Session: api.Session{ID: "s", Kind: "shell"}}}
	s := u.sidebar
	s.open = false
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	rail, panel := s.boxWidth(gtx, false), s.boxWidth(gtx, true)
	// The stage beside the sidebar, as body lays it out.
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			s.layout(gtx, th, m, "a1")
			s.peekLayout(gtx, th, m, "a1", image.Point{})
			x := s.stageReserve(gtx)
			defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
			row := gtx
			row.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-x, gtx.Constraints.Max.Y))
			if err := u.stageRow(row, v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	wide := settledCols(t, r, tv)
	left := func() int { return s.stageReserve(gtx) }
	edges, cols := slide(t, r, tv, railTree, &s.peek.slide, left)
	// The panel slides out from under the rail, which the stage never
	// gives back.
	at := func(share float32) int { return max(int(share*float32(panel)), rail) }
	onCurve(t, "out", edges, true, at)
	heldThenOnce(t, "out", cols)
	if narrow, _ := tv.vt.Size(); narrow >= wide {
		t.Fatalf("out: %d columns beside the sidebar, want fewer than the %d beside the rail", narrow, wide)
	}
	edges, cols = slide(t, r, tv, f32.Pt(700, 300), &s.peek.slide, left)
	onCurve(t, "back", edges, false, at)
	heldThenOnce(t, "back", cols)
	if c, _ := tv.vt.Size(); c != wide {
		t.Errorf("back: %d columns once the sidebar went in, want the %d it had before", c, wide)
	}
}

// labelX is where the first thing drawn with label starts.
func labelX(t *testing.T, r *frameRig, label string) int {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			return n.Desc.Bounds.Min.X
		}
	}
	t.Fatalf("nothing drawn as %q:\n%s", label, r.drawn())
	return 0
}

func TestDockPeekCarriesTheChatWithTheStageEdgeOnEveryFrame(t *testing.T) {
	u, r := peekDockFrames(t)
	u.palette, u.worktree = new(commandPalette), new(worktreeDialog)
	u.git = newGitPanel(u.ctx, u.client, &u.root, r.invalidate, func(ui.Toast) {}, "monospace", 12)
	u.terms.client = u.client
	u.model = chatModel(api.ViewChat)
	u.model.invalidate = r.invalidate
	u.model.chat.kinds = []string{"claude"}
	sv, _ := u.model.sessionViewOf("s1")
	v := stageView{hasActive: true, active: sv}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			row := gtx
			row.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-gtx.Dp(gutter), gtx.Constraints.Max.Y))
			if err := u.stageRow(row, v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.settle(t)
	tui := i18n.T("chat.switch.tui")
	// The switch sits at the strip's right end, inset from the island's.
	inset := rigWindow.X - gtx1(gutter) - labelX(t, r, tui)
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: edgePoint})
	frames := 0
	for range 100 {
		r.frame()
		if u.dock.slide.landed {
			break
		}
		if u.dock.slide.moving() {
			frames++
			if end, x := u.dock.islands[0].x1, labelX(t, r, tui); x != end-inset {
				t.Errorf("frame %d of the slide: the chat's switch starts at %d, want %d, %d inside the stage's edge at %d", frames-1, x, end-inset, inset, end)
			}
		}
		r.now = r.now.Add(frameStep)
	}
	if frames < 5 {
		t.Fatalf("the dock slid over %d frames, want one a frame of 180ms", frames)
	}
}

// gtx1 is d in the rig's pixels, a pixel a dp.
func gtx1(d unit.Dp) int { return layout.Context{Metric: unit.Metric{PxPerDp: 1}}.Dp(d) }
