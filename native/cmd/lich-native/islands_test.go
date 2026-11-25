package main

import (
	"image"
	"image/color"
	"slices"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
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

func dragTab(t *testing.T, bar *topbar, r *frameRig, id string, dx float32) f32.Point {
	t.Helper()
	tab := laidOf(t, bar, id)
	from := f32.Pt(float32(tab.Min.X+tab.Dx()/2), float32(tab.Min.Y+tab.Dy()/2))
	r.hover(from)
	r.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: from})
	r.frame()
	to := from.Add(f32.Pt(dx, 0))
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: to})
	r.frame()
	if got := bar.sort.Dragging(); got != id {
		t.Fatalf("dragging %q, want %q", got, id)
	}
	return to
}

// Every tab is a pill on the canvas, a gutter clear of the islands: the
// active one uses the Sidebar surface and Border, a hovered one gets the
// foreground's 4% fill, and one hairline marks where pinned tabs end.
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
	margin, separatorWidth := gtx1(tabGroupMargin), gtx1(tabSeparatorWidth)
	if b.Min.Y != top || a.Min.X-p.Max.X != 2*margin+separatorWidth {
		t.Fatalf("tabs laid at %v, %v and %v: want the pinned-group separator to have %dpx side margins", p, a, b, margin)
	}
	if b.Min.X-a.Max.X != gtx1(tabGap) {
		t.Fatalf("tabs a and b are %dpx apart, want %dpx", b.Min.X-a.Max.X, gtx1(tabGap))
	}
	mid, cmid := b.Min.X+b.Dx()/2, c.Min.X+c.Dx()/2
	separatorX := a.Min.X - margin - separatorWidth
	hover := th.Over(ui.Alpha(th.Foreground, tabHoverFill))
	activeBorder := th.On(th.Sidebar).Over(th.Border)
	for _, c := range []struct {
		name string
		x, y int
		want color.NRGBA
	}{
		{"the active pill", mid, top + pill/2, th.Sidebar},
		{"the active pill's top border", mid, top, activeBorder},
		{"the gutter under the active pill", mid, top + pill + 2, th.Canvas},
		{"the active pill's rounded corner", b.Min.X, top, th.Canvas},
		{"a tab at rest", a.Min.X + a.Dx()/2, top + 3, th.Canvas},
		{"the hovered pill", cmid, top + 3, hover},
		{"the gutter under the hovered pill", cmid, top + pill + 2, th.Canvas},
		{"between two tabs at rest", d.Min.X, top + pill/2, th.Canvas},
		{"before the pinned group's hairline", separatorX - 1, top + pill/2, th.Canvas},
		{"the pinned group's hairline", separatorX, top + pill/2, th.Over(th.Border)},
		{"after the pinned group's hairline", separatorX + 2, top + pill/2, th.Canvas},
		{"over the hairline's top", separatorX, top + 4, th.Canvas},
	} {
		if got := img.RGBAAt(c.x, c.y); !inkNear(got, c.want) {
			t.Errorf("%s at (%d,%d): %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
}

func TestDraggedActiveTabUsesAccentAndBorder(t *testing.T) {
	bar, r := tabsFrames(t)
	r.settle(t)
	center := dragTab(t, bar, r, "b", 120)
	img := renderRig(t, r)
	th := r.theme
	activeBorder := th.On(th.Accent).Over(th.Border)
	top := gtx1(tabTop)
	for _, c := range []struct {
		name string
		x, y int
		want color.NRGBA
	}{
		{"dragged active tab", int(center.X), top + gtx1(tabHeight)/2, th.Accent},
		{"dragged active tab border", int(center.X), top, activeBorder},
	} {
		if got := img.RGBAAt(c.x, c.y); !inkNear(got, c.want) {
			t.Errorf("%s at (%d,%d): %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
}

func TestPinnedGroupSeparatorStaysAtTheGroupBoundaryDuringDrag(t *testing.T) {
	bar, r := tabsFrames(t)
	r.settle(t)
	dragTab(t, bar, r, "a", 120)
	img := renderRig(t, r)
	th := r.theme
	pill := gtx1(tabHeight)
	top := gtx1(tabTop)
	p, a := laidOf(t, bar, "p"), laidOf(t, bar, "a")
	separatorX := a.Min.X - gtx1(tabGroupMargin) - gtx1(tabSeparatorWidth)
	if got, want := img.RGBAAt(separatorX, top+pill/2), th.Over(th.Border); !inkNear(got, want) {
		t.Errorf("group separator moved with tab a: pixel at (%d,%d) is %v, want %v (pinned tab %v, normal tab slot %v)", separatorX, top+pill/2, got, want, p, a)
	}
}

func TestPinnedGroupSeparatorScrollsWithTheTabs(t *testing.T) {
	r := newFrameRig(image.Pt(900, 120), nil)
	bar := newTopbar(&ui.Root{})
	f := topbarFrame{brandWidth: 200, stripX: 240, active: "b"}
	for i, id := range []string{"p", "q", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		f.tabs = append(f.tabs, projectTab{ID: id, Name: id, pinned: i < 2})
	}
	r.draw = func(gtx C, th *ui.Theme) {
		th.Surface = th.Canvas
		paint.Fill(gtx.Ops, th.Canvas)
		bar.layout(gtx, th, f)
	}
	r.settle(t)
	const scrolled = 40
	bar.list.Position.Offset, bar.list.Position.BeforeEnd = scrolled, true
	r.frame()
	r.frame()
	if got := bar.list.Position.Offset; got != scrolled {
		t.Fatalf("the strip scrolled %dpx, want %d", got, scrolled)
	}
	img := renderRig(t, r)
	x := laidOf(t, bar, "a").Min.X - gtx1(tabGroupMargin) - gtx1(tabSeparatorWidth)
	y := gtx1(tabTop) + gtx1(tabHeight)/2
	if got, want := img.RGBAAt(x, y), r.theme.Over(r.theme.Border); !inkNear(got, want) {
		t.Errorf("group separator of a scrolled strip at (%d,%d): %v, want %v", x, y, got, want)
	}
}

func TestTabMarginsRoundSeparatelyAtFractionalScale(t *testing.T) {
	gtx := C{Metric: unit.Metric{PxPerDp: 1.31, PxPerSp: 1.31}}
	tabs := []projectTab{{ID: "p", pinned: true}, {ID: "a"}, {ID: "b"}}
	left, right := tabMarginPixels(gtx, tabs, 1)
	wantLeft := gtx.Dp(tabGroupMargin + tabSeparatorWidth)
	wantRight := gtx.Dp(tabGap)
	if left != wantLeft || right != wantRight || left+right != wantLeft+wantRight {
		t.Fatalf("fractional tab margins = %d + %d px, want separately rounded %d + %d px", left, right, wantLeft, wantRight)
	}
	if left+right == gtx.Dp(tabGroupMargin+tabSeparatorWidth+tabGap) {
		t.Fatalf("test scale no longer distinguishes separate rounding from a combined conversion: margins %d + %d", left, right)
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

// A toast is a cover: a native page under it is cut where the toast is
// drawn, ring included, its rounded corners left to the page, and whole
// again once it is gone.
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
	if !slices.ContainsFunc(cuts, title.In) {
		t.Fatalf("the page's cuts are %v, want them around the toast's title at %v", cuts, title)
	}
	toasts := u.toaster.Covers()
	if len(toasts) != 1 {
		t.Fatalf("toaster covers %v, want the one toast", toasts)
	}
	box := toasts[0].Box.Sub(page.Min)
	if ring := image.Pt(box.Min.X+box.Dx()/2, box.Min.Y-1); !slices.ContainsFunc(cuts, ring.In) {
		t.Errorf("the toast's ring at %v is under the page", ring)
	}
	if corner := box.Min; slices.ContainsFunc(cuts, corner.In) {
		t.Errorf("the toast's rounded corner at %v shows the window's background", corner)
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

// headerParts is where the last frame drew each of labels, by label.
func headerParts(r *frameRig, labels ...string) map[string]image.Rectangle {
	parts := map[string]image.Rectangle{}
	for _, n := range r.nodes() {
		for _, l := range labels {
			if _, seen := parts[l]; !seen && n.Desc.Label == l {
				parts[l] = n.Desc.Bounds
			}
		}
	}
	return parts
}

// A pane's header is the lone session's: the name at the same place, the
// selector on the same line and the same size, only moved left by the pane's
// own controls, and no hairline under it.
func TestAPanesHeaderIsTheLoneSessionsHeader(t *testing.T) {
	const w, label = 800, "Cart totals a cent off"
	gui, tui := i18n.T("chat.switch.gui"), i18n.T("chat.switch.tui")
	_, lone := stripFrames(t, w, label)
	u, pane := stripFrames(t, w, label)
	sv, _ := u.model.sessionViewOf("s1")
	v := stageView{hasLayout: true, layout: layoutTab{Focus: "s1", Root: &paneNode{Session: "s1"}}, panes: map[string]sessionView{"s1": sv}}
	u.sidebar, u.footer, u.dock, u.git, u.extPanel, u.topbar = &sidebar{}, &footer{}, &dock{}, &gitPanel{}, &extensionPanel{}, &topbar{}
	u.palette, u.worktree = &commandPalette{}, &worktreeDialog{}
	pane.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		paint.Fill(gtx.Ops, th.Background)
		u.root.Layout(gtx, func(gtx C) D {
			if err := u.paneAt(gtx, v, "s1", image.Rect(0, 0, w, 300), false); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	lone.settle(t)
	pane.settle(t)
	a := headerParts(lone, label, gui, tui)
	b := headerParts(pane, label, gui, tui)
	for _, l := range []string{label, gui, tui} {
		if a[l].Empty() || b[l].Empty() {
			t.Fatalf("%q drawn at %v alone and %v in a pane", l, a[l], b[l])
		}
	}
	if a[label].Min != b[label].Min {
		t.Errorf("%q at %v alone, %v in a pane", label, a[label], b[label])
	}
	for _, l := range []string{gui, tui} {
		if a[l].Min.Y != b[l].Min.Y || b[l].Min.X >= a[l].Min.X {
			t.Errorf("%q at %v alone, %v in a pane: want the same line and size, left of where it stands alone", l, a[l], b[l])
		}
	}
	end := w - gtx1(balloonInset)
	loneTUI, _ := lone.smallestClickAt(a[tui].Min.Add(image.Pt(2, 2)))
	paneTUI, _ := pane.smallestClickAt(b[tui].Min.Add(image.Pt(2, 2)))
	if loneTUI.Size() != paneTUI.Size() {
		t.Errorf("TUI is %v alone, %v in a pane: want the same size", loneTUI, paneTUI)
	}
	if loneTUI.Max.X > end {
		t.Errorf("the lone selector ends at %d, past the header's edge at %d", loneTUI.Max.X, end)
	}
	if c, ok := pane.smallestClickAt(image.Pt(end-4, paneTUI.Min.Y+paneTUI.Dy()/2)); !ok || c.Min.X < paneTUI.Max.X {
		t.Errorf("the pane's header lacks its controls between the selector at %v and its edge", paneTUI)
	}
	img := renderRig(t, pane)
	head := gtx1(paneHeader)
	if got := img.RGBAAt(w/3, head-1); !inkNear(got, pane.theme.Background) {
		t.Errorf("the header's foot is %v, want the pane's ground %v: no hairline", got, pane.theme.Background)
	}
}

// mixedFrames is a layout of root's panes over the chat rig's stage: s1 and
// s3 drawn as chats, s2 as an agent's terminal.
func mixedFrames(t *testing.T, root *paneNode, focus string) (*window, *frameRig, *stageView) {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model = chatModel(api.ViewChat, api.ViewTerminal, api.ViewChat)
	u.model.invalidate = r.invalidate
	u.model.chat.kinds = []string{"claude"}
	u.sidebar, u.dock, u.git, u.extPanel, u.topbar = &sidebar{}, &dock{}, &gitPanel{}, &extensionPanel{}, &topbar{}
	u.palette, u.worktree = &commandPalette{}, &worktreeDialog{}
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate, client: u.client, unzoomed: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	agentTerm(t, u, "s2")
	lt := layoutTab{ID: "l", Focus: focus, Root: root}
	v := &stageView{hasLayout: true, layout: lt, panes: u.model.viewsOf(lt.sessions(), time.Now())}
	v.active, v.hasActive = v.panes[focus], true
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			if err := u.layoutPanes(gtx, *v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r, v
}

// The balloons' band stays under a layout holding an agent's terminal while
// the focus moves between a chat and that terminal: no pane is resized.
func TestMovingTheFocusFromAChatToATerminalResizesNoPane(t *testing.T) {
	root := &paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Session: "s1"}, Second: &paneNode{Session: "s2"}}
	u, r, v := mixedFrames(t, root, "s1")
	term := u.terms.byID["s2"]
	settledCols(t, r, term)
	bodies := func() (image.Rectangle, image.Rectangle) {
		u.drop.mu.Lock()
		defer u.drop.mu.Unlock()
		return u.drop.rects["s1"], u.drop.rects["s2"]
	}
	chat0, term0 := bodies()
	c0, r0 := term.vt.Size()
	if foot := r.size.Y - gtx1(layoutBalloonBand); term0.Max.Y != foot || chat0.Max.Y != foot {
		t.Fatalf("with the chat in focus the panes end at y=%d and %d, want %d, over the band", chat0.Max.Y, term0.Max.Y, foot)
	}
	v.layout.Focus, v.active = "s2", v.panes["s2"]
	settledCols(t, r, term)
	chat1, term1 := bodies()
	if chat1 != chat0 || term1 != term0 {
		t.Errorf("the panes went from %v %v to %v %v when the focus moved to the terminal", chat0, term0, chat1, term1)
	}
	if c, rows := term.vt.Size(); c != c0 || rows != r0 {
		t.Errorf("the terminal went from %dx%d to %dx%d", c0, r0, c, rows)
	}
}

// A layout of chats has no band: its panes run to the stage's foot.
func TestALayoutOfChatsHasNoBand(t *testing.T) {
	root := &paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Session: "s1"}, Second: &paneNode{Session: "s3"}}
	u, r, _ := mixedFrames(t, root, "s1")
	r.settle(t)
	u.drop.mu.Lock()
	defer u.drop.mu.Unlock()
	for _, id := range []string{"s1", "s3"} {
		if b := u.drop.rects[id]; b.Max.Y != r.size.Y {
			t.Errorf("chat %s ends at y=%d, want the stage's foot at %d", id, b.Max.Y, r.size.Y)
		}
	}
}

// In a layout of several projects a pane's header leads with the project, in
// the muted run before the name; in a layout of one it does not.
func TestAMixedLayoutsPaneNamesItsProject(t *testing.T) {
	const w, label = 800, "Cart totals a cent off"
	u, r := stripFrames(t, w, label)
	sv, _ := u.model.sessionViewOf("s1")
	v := stageView{hasLayout: true, layout: layoutTab{Focus: "s1", Root: &paneNode{Session: "s1"}}, panes: map[string]sessionView{"s1": sv}}
	u.sidebar, u.footer, u.dock, u.git, u.extPanel, u.topbar = &sidebar{}, &footer{}, &dock{}, &gitPanel{}, &extensionPanel{}, &topbar{}
	u.palette, u.worktree = &commandPalette{}, &worktreeDialog{}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			if err := u.paneAt(gtx, v, "s1", image.Rect(0, 0, w, 300), false); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.settle(t)
	if !r.shows(label) || r.shows("lich · ") {
		t.Fatalf("a layout of one project names it:\n%s", r.drawn())
	}
	v.projectNames = map[string]string{sv.projectID: "lich"}
	r.settle(t)
	if !r.shows("lich · ") {
		t.Fatalf("a layout of several projects does not name the pane's:\n%s", r.drawn())
	}
	if !r.shows(label) {
		t.Errorf("the name is gone from the header:\n%s", r.drawn())
	}
}

func TestMixedProjectsNeedsTwo(t *testing.T) {
	one := map[string]sessionView{"a": {projectID: "lich"}, "b": {projectID: "lich"}}
	two := map[string]sessionView{"a": {projectID: "lich"}, "b": {projectID: "skipo"}}
	if mixedProjects(one) || !mixedProjects(two) || mixedProjects(nil) {
		t.Errorf("mixed: one project %v, two %v, none %v", mixedProjects(one), mixedProjects(two), mixedProjects(nil))
	}
}
