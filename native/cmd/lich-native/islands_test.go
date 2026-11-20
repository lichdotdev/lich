package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/ui"
)

// inkNear reports whether a and b differ by at most 3 in every channel, the
// room antialiasing and the GPU's rounding leave.
func inkNear(a color.RGBA, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return x+3 >= y && y+3 >= x }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

// tabsFrames is the top bar on the canvas with a pinned tab and four more,
// the second of them active.
func tabsFrames(t *testing.T) (*topbar, *frameRig) {
	t.Helper()
	r := newFrameRig(image.Pt(1400, 120), nil)
	bar := newTopbar(&ui.Root{})
	f := topbarFrame{brandWidth: 200, stripX: 240, active: "b", tabs: []projectTab{
		{ID: "p", Name: "p", pinned: true}, {ID: "a", Name: "a"}, {ID: "b", Name: "b"}, {ID: "c", Name: "c"}, {ID: "d", Name: "d"},
	}}
	r.draw = func(gtx C, th *ui.Theme) {
		th.Surface = th.Canvas
		paint.Fill(gtx.Ops, th.Canvas)
		bar.layout(gtx, th, f)
	}
	return bar, r
}

func laidOf(t *testing.T, bar *topbar, id string) image.Rectangle {
	t.Helper()
	for _, l := range bar.laid {
		if l.id == id {
			return l.rect
		}
	}
	t.Fatalf("tab %s not laid", id)
	return image.Rectangle{}
}

// Every tab is a pill on the canvas, a gutter clear of the islands: the
// active one in Accent, a hovered one at the foreground's 4%, and a hairline
// only where the pinned tabs end.
func TestTabsArePillsOnTheCanvas(t *testing.T) {
	bar, r := tabsFrames(t)
	r.settle(t)
	c := laidOf(t, bar, "c")
	r.hover(f32.Pt(float32(c.Min.X+c.Dx()/2), float32(c.Min.Y+4)))
	r.settle(t)
	img := renderRig(t, r)
	th := r.theme
	top, pill := gtx1(tabTop), gtx1(tabHeight)
	p, a, b, d := laidOf(t, bar, "p"), laidOf(t, bar, "a"), laidOf(t, bar, "b"), laidOf(t, bar, "d")
	if b.Min.Y != top || p.Max.X != a.Min.X {
		t.Fatalf("tabs laid at %v, %v and %v: want them from y=%d, edge to edge", p, a, b, top)
	}
	mid, cmid := b.Min.X+b.Dx()/2, c.Min.X+c.Dx()/2
	hover := th.Over(ui.Alpha(th.Foreground, tabHoverFill))
	for _, c := range []struct {
		name string
		x, y int
		want color.NRGBA
	}{
		{"the active pill", mid, top + 3, th.Accent},
		{"the active pill's foot", mid, top + pill - 2, th.Accent},
		{"the gutter under the active pill", mid, top + pill + 2, th.Canvas},
		{"the active pill's rounded corner", b.Min.X, top, th.Canvas},
		{"a tab at rest", a.Min.X + a.Dx()/2, top + 3, th.Canvas},
		{"the hovered pill", cmid, top + 3, hover},
		{"the gutter under the hovered pill", cmid, top + pill + 2, th.Canvas},
		{"between two tabs at rest", d.Min.X, top + pill/2, th.Canvas},
		{"the pinned group's hairline", a.Min.X, top + pill/2, th.Over(th.Border)},
		{"over the hairline's top", a.Min.X, top + 4, th.Canvas},
	} {
		if got := img.RGBAAt(c.x, c.y); !inkNear(got, c.want) {
			t.Errorf("%s at (%d,%d): %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
}

// A tab takes hover and clicks on its pill only, not the gutter under it.
func TestATabTakesOnlyItsPill(t *testing.T) {
	// The spec's pill: 32px tall, 6px under the band's top, a 6px gutter
	// under it.
	const pillTop, pillFoot = 6, 38
	bar, r := tabsFrames(t)
	r.settle(t)
	c := laidOf(t, bar, "c")
	if c.Min.Y != pillTop || c.Max.Y != pillFoot {
		t.Errorf("tab c laid %v, want it from y=%d to %d", c, pillTop, pillFoot)
	}
	x, gap := c.Min.X+c.Dx()/2, pillFoot+3
	r.hover(f32.Pt(float32(x), float32(gap)))
	r.settle(t)
	if bar.tabState("c").click.Hovered() {
		t.Error("the gutter under the pill hovers the tab")
	}
	if b, ok := r.smallestClickAt(image.Pt(x, gap)); ok && b.Dx() <= c.Dx() {
		t.Errorf("the gutter under the pill takes the tab's clicks, at %v", b)
	}
	r.hover(f32.Pt(float32(x), float32(pillFoot-2)))
	r.settle(t)
	if !bar.tabState("c").click.Hovered() {
		t.Error("the pill's foot does not hover the tab")
	}
}

// A toast is a cover: a native page under it is cut where the stack is
// drawn, and whole again once it is gone.
func TestAToastCutsTheNativePageUnderIt(t *testing.T) {
	u := &window{sidebar: &sidebar{}, dock: &dock{}, toastFloor: 60}
	r := newFrameRig(rigWindow, nil)
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.layoutToasts(gtx)
	}
	// The toaster dates a dismissal by the clock, so the rig runs on it.
	r.now = time.Now()
	id := u.toaster.Show(ui.Toast{Title: "Saved", Duration: ui.ToastForever})
	r.settle(t)
	page := image.Rect(rigWindow.X/2, 0, rigWindow.X, rigWindow.Y)
	cuts := pageCuts(page, nil, u.floatingBoxes())
	title := labelBox(t, r, "Saved").Min.Sub(page.Min)
	if len(cuts) != 1 || !title.In(cuts[0]) {
		t.Fatalf("the page's cuts are %v, want one around the toast's title at %v", cuts, title)
	}
	u.toaster.Dismiss(id)
	r.settle(t)
	if boxes := u.floatingBoxes(); len(boxes) != 0 {
		t.Errorf("the page is still cut at %v once the toast is gone", boxes)
	}
}

// agentTerm is a terminal for session id on u's stage.
func agentTerm(t *testing.T, u *window, id string) *termView {
	t.Helper()
	rend, err := gioterm.NewRenderer(gioterm.RenderRows, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := gioterm.New(80, 24, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	tv := &termView{id: id, vt: vt, r: rend}
	tv.seeded.Store(true)
	u.terms.byID[id] = tv
	return tv
}

// splitFrames is the stage row on the canvas with a layout of two agents'
// terminals side by side, a's in focus, and the dock collapsed.
func splitFrames(t *testing.T) (*window, *frameRig, *stageView) {
	t.Helper()
	u, r := peekDockFrames(t)
	u.palette, u.worktree = new(commandPalette), new(worktreeDialog)
	u.git = newGitPanel(u.ctx, u.client, &u.root, r.invalidate, func(ui.Toast) {}, "monospace", 12)
	agentTerm(t, u, "a")
	agentTerm(t, u, "b")
	u.terms.client, u.terms.unzoomed = u.client, unit.Metric{PxPerDp: 1, PxPerSp: 1}
	agent := func(id string) sessionView { return sessionView{ID: id, Kind: "claude"} }
	v := &stageView{hasLayout: true, hasActive: true, active: agent("a"), in: busySession(),
		layout: layoutTab{ID: "l", Focus: "a", Root: &paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Session: "a"}, Second: &paneNode{Session: "b"}}},
		panes:  map[string]sessionView{"a": agent("a"), "b": agent("b")},
	}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		paint.Fill(gtx.Ops, th.Canvas)
		u.root.Layout(gtx, func(gtx C) D {
			u.footer.frame(gtx, v.in)
			row := gtx
			row.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-gtx.Dp(gutter), gtx.Constraints.Max.Y))
			if err := u.stageRow(row, *v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r, v
}

// paneBodies is where the last frame drew each pane's terminal.
func paneBodies(u *window) (a, b image.Rectangle) {
	u.drop.mu.Lock()
	defer u.drop.mu.Unlock()
	return u.drop.rects["a"], u.drop.rects["b"]
}

// A layout's panes are islands on the canvas: they run to the stage's
// edges, a gutter apart, and stop a gutter over the balloons, which stand
// flush on the stage's foot and sides.
func TestALayoutsPanesAreIslandsOnTheCanvas(t *testing.T) {
	u, r, _ := splitFrames(t)
	r.settle(t)
	row := rigWindow.X - gtx1(gutter)
	a, b := paneBodies(u)
	head, g := gtx1(paneHeader), gtx1(gutter)
	foot := rigWindow.Y - gtx1(layoutBalloonBand)
	if a.Min != image.Pt(0, head) || b.Max.X != row {
		t.Errorf("panes at %v and %v, want them from the stage's corner at (0,%d) to its edge at %d", a, b, head, row)
	}
	if b.Min.X-a.Max.X != g {
		t.Errorf("the panes are %d apart, want the gutter's %d", b.Min.X-a.Max.X, g)
	}
	if a.Max.Y != foot || b.Max.Y != foot {
		t.Errorf("the panes end at y=%d and %d, want %d, a gutter over the balloons", a.Max.Y, b.Max.Y, foot)
	}
	pad := gtx1(balloonPad)
	first, _ := r.smallestClickAt(image.Pt(pad+1, rigWindow.Y-pad-1))
	last, _ := r.smallestClickAt(image.Pt(row-pad-1, rigWindow.Y-pad-1))
	if first.Min.X != pad || first.Max.Y != rigWindow.Y-pad || last.Max.X != row-pad {
		t.Errorf("the balloons' end items at %v and %v, want them %d inside the stage's sides and foot", first, last, pad)
	}
	img := renderRig(t, r)
	th := r.theme
	if got := img.RGBAAt(a.Max.X+g/2, rigWindow.Y/2); !inkNear(got, th.Canvas) {
		t.Errorf("the gutter between the panes is %v, want the canvas %v: no island around them", got, th.Canvas)
	}
	if got := img.RGBAAt(rigWindow.X/2, foot+g/2); !inkNear(got, th.Canvas) {
		t.Errorf("the gutter over the balloons is %v, want the canvas %v", got, th.Canvas)
	}
}

// The balloons' band is the layout's, not the focused pane's: moving the
// focus resizes no pane's terminal.
func TestMovingTheFocusResizesNoPane(t *testing.T) {
	u, r, v := splitFrames(t)
	settledCols(t, r, u.terms.byID["a"])
	a0, b0 := paneBodies(u)
	ca, ra := u.terms.byID["a"].vt.Size()
	cb, rb := u.terms.byID["b"].vt.Size()
	if a0.Max.Y >= rigWindow.Y-gtx1(balloonItemHeight) {
		t.Fatalf("pane a ends at y=%d, inside the balloons", a0.Max.Y)
	}
	v.layout.Focus, v.active = "b", v.panes["b"]
	settledCols(t, r, u.terms.byID["b"])
	a1, b1 := paneBodies(u)
	if a1 != a0 || b1 != b0 {
		t.Errorf("the panes went from %v %v to %v %v when the focus moved", a0, b0, a1, b1)
	}
	if c, rr := u.terms.byID["a"].vt.Size(); c != ca || rr != ra {
		t.Errorf("pane a's terminal went from %dx%d to %dx%d", ca, ra, c, rr)
	}
	if c, rr := u.terms.byID["b"].vt.Size(); c != cb || rr != rb {
		t.Errorf("pane b's terminal went from %dx%d to %dx%d", cb, rb, c, rr)
	}
}
