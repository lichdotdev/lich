package main

import (
	"errors"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A layout on the island: its tree of panes, each a session's terminal under
// a header naming it, with draggable gaps between them.

// The smallest pane a split leaves. 480dp is 51 columns at the default 13sp,
// where Claude Code's and Codex's dialogs still wrap cleanly (measured
// 2026-10-17); a larger terminal font holds fewer.
var (
	paneMinWidth  = unit.Dp(480)
	paneMinHeight = unit.Dp(180)
	paneHeader    = ui.Space(8)
	paneRadius    = ui.RadiusLG
	// paneDim is how far an unfocused pane's terminal fades toward the
	// island, so the one that takes the keys reads first.
	paneDim = float32(0.35)
)

var (
	icPaneZoom    = icons.Lucide("maximize-2")
	icPaneUnzoom  = icons.Lucide("minimize-2")
	icPaneUnsplit = icons.Lucide("x")
)

// paneUI is one pane's widgets, kept across frames by session id.
type paneUI struct {
	zoom, unsplit, chip, delegates    widget.Clickable
	zoomTip, unsplitTip, delegatesTip ui.Tooltip
}

// paneDivider is the gap between a split's two sides, dragged to move the
// ratio; ratio holds the drag's value until it is stored on release.
type paneDivider struct {
	dragging   bool
	press      float32
	startRatio float64
	ratio      float64
}

// layoutStage is the window's state for the layout on screen.
type layoutStage struct {
	panes    map[string]*paneUI
	dividers map[string]*paneDivider
	// metric is the island's, which the smallest pane is measured in.
	metric  unit.Metric
	move    moveConfirm
	pointer dragPointer
	// focus, zoom and unsplit are what this frame's clicks asked of a pane;
	// drop is a pane dragged by its header onto a drop zone.
	focus, unsplit string
	zoom           bool
	drop           ui.SortDrop
	// switchTab is a strip tab something carried rested on, to bring on
	// screen.
	switchTab string
	// delegates is the session whose delegates chip was clicked.
	delegates string
	// sort drags the panes by their headers, over the sidebar's carry.
	sort ui.Sortable
}

func (s *layoutStage) pane(id string) *paneUI {
	if s.panes == nil {
		s.panes = map[string]*paneUI{}
	}
	p, ok := s.panes[id]
	if !ok {
		p = new(paneUI)
		s.panes[id] = p
	}
	return p
}

func (s *layoutStage) divider(key string) *paneDivider {
	if s.dividers == nil {
		s.dividers = map[string]*paneDivider{}
	}
	d, ok := s.dividers[key]
	if !ok {
		d = new(paneDivider)
		s.dividers[key] = d
	}
	return d
}

// layoutIsland draws the layout on screen over the island's area.
func (u *window) layoutIsland(gtx C, v stageView) error {
	st := &u.layoutStage
	st.metric = gtx.Metric
	st.sort.Carry = &u.sidebar.carry
	// The drop comes from readDragPointer, which knows the zone under the
	// pointer; the Sortable only carries the pane.
	st.sort.Update(gtx)
	lt := v.layout
	ids := lt.sessions()
	root := lt.Root
	if lt.Zoomed {
		root = &paneNode{Session: lt.Focus}
	}
	u.terms.onScreen(onScreenOf(root)...)
	in := image.Rectangle{Max: gtx.Constraints.Max}.Inset(gtx.Dp(gutter))
	return u.paneTree(gtx, v, root, "", in, len(ids) > 1)
}

func onScreenOf(n *paneNode) []string {
	t := layoutTab{Root: n}
	return t.sessions()
}

// paneTree draws node n into r. path names the node for its divider: "" the
// root, then one digit per step, 0 first and 1 second.
func (u *window) paneTree(gtx C, v stageView, n *paneNode, path string, r image.Rectangle, several bool) error {
	if n.leaf() {
		return u.paneAt(gtx, v, n.Session, r, several)
	}
	first, second, gap := u.splitRects(gtx, v, n, path, r)
	if err := u.paneTree(gtx, v, n.First, path+"0", first, several); err != nil {
		return err
	}
	if err := u.paneTree(gtx, v, n.Second, path+"1", second, several); err != nil {
		return err
	}
	u.dividerAt(gtx, v.layout.ID, n, path, r, gap)
	return nil
}

// splitRects divides r between n's sides at its ratio, the one being dragged
// while a drag is on, keeping each side its smallest size while r has room.
func (u *window) splitRects(gtx C, v stageView, n *paneNode, path string, r image.Rectangle) (first, second, gap image.Rectangle) {
	ratio := n.Ratio
	if d := u.layoutStage.divider(v.layout.ID + "/" + path); d.dragging {
		ratio = d.ratio
	}
	g := gtx.Dp(gutter)
	fw, fh := minSize(n.First, gtx.Dp(paneMinWidth), gtx.Dp(paneMinHeight), g)
	sw, sh := minSize(n.Second, gtx.Dp(paneMinWidth), gtx.Dp(paneMinHeight), g)
	span, lo, hi := r.Dy()-g, fh, sh
	if n.Axis == splitRow {
		span, lo, hi = r.Dx()-g, fw, sw
	}
	at := int(ratio * float64(span))
	if lo+hi <= span {
		at = min(max(at, lo), span-hi)
	}
	if n.Axis == splitRow {
		x := r.Min.X + at
		return image.Rect(r.Min.X, r.Min.Y, x, r.Max.Y), image.Rect(x+g, r.Min.Y, r.Max.X, r.Max.Y), image.Rect(x, r.Min.Y, x+g, r.Max.Y)
	}
	y := r.Min.Y + at
	return image.Rect(r.Min.X, r.Min.Y, r.Max.X, y), image.Rect(r.Min.X, y+g, r.Max.X, r.Max.Y), image.Rect(r.Min.X, y, r.Max.X, y+g)
}

// dividerAt makes the gap between n's sides draggable, storing the ratio it
// was let go at.
func (u *window) dividerAt(gtx C, layoutID string, n *paneNode, path string, r, gap image.Rectangle) {
	d := u.layoutStage.divider(layoutID + "/" + path)
	span := float32(r.Dy() - gap.Dy())
	if n.Axis == splitRow {
		span = float32(r.Dx() - gap.Dx())
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: d, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok || span <= 0 {
			continue
		}
		at := e.Position.Y
		if n.Axis == splitRow {
			at = e.Position.X
		}
		switch e.Kind {
		case pointer.Press:
			d.dragging, d.press, d.startRatio, d.ratio = true, at, n.Ratio, n.Ratio
		case pointer.Drag:
			if d.dragging {
				d.ratio = clampRatio(d.startRatio + float64((at-d.press)/span))
			}
		case pointer.Release:
			if d.dragging {
				d.dragging = false
				u.model.setRatio(layoutID, pathSteps(path), d.ratio)
			}
		case pointer.Cancel:
			d.dragging = false
		}
	}
	defer clip.Rect(gap).Push(gtx.Ops).Pop()
	cursor := pointer.CursorRowResize
	if n.Axis == splitRow {
		cursor = pointer.CursorColResize
	}
	cursor.Add(gtx.Ops)
	event.Op(gtx.Ops, d)
}

func pathSteps(path string) []bool {
	steps := make([]bool, len(path))
	for i := range path {
		steps[i] = path[i] == '1'
	}
	return steps
}

// paneAt draws session id's pane in r: its header, its terminal, and with
// more than one pane the focus ring or the fade.
func (u *window) paneAt(gtx C, v stageView, id string, r image.Rectangle, several bool) error {
	sv, ok := v.panes[id]
	if !ok {
		return nil
	}
	defer op.Offset(r.Min).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(r.Size())
	size := r.Size()
	rr := gtx.Dp(paneRadius)
	focused := id == v.layout.Focus
	bg := u.terms.backgroundOf(id)
	if bg.A == 0 {
		bg = u.theme.Background
	}
	shape := clip.UniformRRect(image.Rectangle{Max: size}, rr)
	paint.FillShape(gtx.Ops, bg, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	on := u.theme.On(bg)
	head := gtx.Dp(paneHeader)
	header := gtx
	header.Constraints = layout.Exact(image.Pt(size.X, head))
	u.layoutStage.sort.Item(header, id, func(gtx C) D {
		u.paneHeader(gtx, on, v, sv, focused)
		return D{Size: gtx.Constraints.Max}
	}, nil)
	paint.FillShape(gtx.Ops, on.Over(on.Border), clip.Rect{Min: image.Pt(0, head-gtx.Dp(1)), Max: image.Pt(size.X, head)}.Op())
	body := gtx
	body.Constraints = layout.Exact(image.Pt(size.X, max(size.Y-head, 0)))
	off := op.Offset(image.Pt(0, head)).Push(gtx.Ops)
	err := u.content(body, sv, focused && u.terminalTakesKeys(gtx))
	if focused {
		u.paneDrop(body, on, sv, r.Min.Add(image.Pt(0, head)))
	}
	if several && !focused {
		paint.FillShape(gtx.Ops, ui.Alpha(u.theme.Background, paneDim), clip.Rect{Max: body.Constraints.Max}.Op())
	}
	off.Pop()
	u.paneFocusTarget(gtx, id)
	area.Pop()
	u.paneDropZones(gtx, id, image.Rectangle{Max: size}, r.Add(u.islandAt), false)
	border, width := on.Over(on.Border), float32(gtx.Dp(1))
	if several && focused {
		border, width = u.theme.Ring, float32(gtx.Dp(1.5))
	}
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: shape.Path(gtx.Ops), Width: width}.Op())
	return err
}

// paneDrop takes a file dragged over the focused pane's terminal, at at in
// the island, which is where the active session's prompt is.
func (u *window) paneDrop(gtx C, th *ui.Theme, sv sessionView, at image.Point) {
	if !u.terms.has(sv.ID) {
		return
	}
	corner := u.islandAt.Add(at)
	u.drop.setRect(image.Rectangle{Min: corner, Max: corner.Add(gtx.Constraints.Max)})
	if u.drop.hinting() || *flagDropHint {
		dropHint(gtx, th, sv.Label, sv.sandbox.Confined)
	}
}

// paneFocusTarget hears a press anywhere on the pane, passing it on to the
// terminal under it, and focuses the pane.
func (u *window) paneFocusTarget(gtx C, id string) {
	p := u.layoutStage.pane(id)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: p, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
			u.layoutStage.focus = id
		}
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, p)
}

// paneHeader is the pane's top row: its ring, its project when the layout
// spans several, its name and checkout, and its zoom and unsplit buttons, or
// while zoomed, the chip that tells what the zoom hides.
func (u *window) paneHeader(gtx C, th *ui.Theme, v stageView, sv sessionView, focused bool) {
	p := u.layoutStage.pane(sv.ID)
	if p.zoom.Clicked(gtx) || p.chip.Clicked(gtx) {
		u.layoutStage.focus, u.layoutStage.zoom = sv.ID, true
	}
	if p.unsplit.Clicked(gtx) {
		u.layoutStage.unsplit = sv.ID
	}
	if p.delegates.Clicked(gtx) {
		u.layoutStage.delegates = sv.ID
	}
	fg, weight := th.MutedForeground, font.Normal
	if focused {
		fg, weight = th.Foreground, font.Medium
	}
	layout.Inset{Left: ui.Space(1), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.sidebar.statusGlyph(gtx, th, &sv) }),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D {
				if !v.mixed {
					return D{}
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(func(gtx C) D { return projectTag(gtx, th, v.projectNames[sv.projectID], fg) }), ui.Gap(1.5))
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextXS, sv.Label).In(fg).Weight(weight).Layout),
					ui.Gap(2),
					layout.Flexed(1, th.Text(ui.TextXS, worktreeLabel(sv, v.projectPaths[sv.projectID])).Muted().Mono().Layout),
				)
			}),
			layout.Rigid(func(gtx C) D { return u.delegatesChip(gtx, th, v, sv, p) }),
			layout.Rigid(func(gtx C) D { return u.paneButtons(gtx, th, v, sv, p) }),
		)
	})
}

// paneButtons is the header's right end: zoom and unsplit, or the zoom chip.
func (u *window) paneButtons(gtx C, th *ui.Theme, v stageView, sv sessionView, p *paneUI) D {
	if v.layout.Zoomed {
		return p.chip.Layout(gtx, func(gtx C) D { return zoomChip(gtx, th, v) })
	}
	zoom := th.Tooltip(&u.root, &p.zoomTip, i18n.T("layouts.pane.zoom"))
	unsplit := th.Tooltip(&u.root, &p.unsplitTip, i18n.T("layouts.pane.unsplit"))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return zoom.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, &p.zoom, icPaneZoom, false) })
		}),
		ui.Gap(1.5),
		layout.Rigid(func(gtx C) D {
			return unsplit.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, &p.unsplit, icPaneUnsplit, false) })
		}),
	)
}

// zoomChip names what the zoom hides: how many panes, and how many of them
// wait on the user.
func zoomChip(gtx C, th *ui.Theme, v stageView) D {
	hidden, waiting := 0, 0
	for _, id := range v.layout.sessions() {
		if id == v.layout.Focus {
			continue
		}
		hidden++
		if v.panes[id].status == statusWaiting {
			waiting++
		}
	}
	text := i18n.T("layouts.pane.hidden", i18n.Args{"count": hidden})
	if waiting > 0 {
		text = i18n.T("layouts.pane.hiddenWaiting", i18n.Args{"count": hidden, "waiting": waiting})
	}
	return pill(gtx, th, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icPaneUnzoom.Layout(gtx, 12, th.MutedForeground) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, text).Muted().Layout),
		)
	}, ui.Space(2), ui.Space(2))
}

// projectTag is the project's name in an outlined pill.
func projectTag(gtx C, th *ui.Theme, name string, fg color.NRGBA) D {
	return pill(gtx, th, th.Text(ui.TextXS, name).In(fg).Layout, ui.Space(2), ui.Space(2))
}

// worktreeLabel names the worktree a pane's session runs in, "" for the
// project's own checkout: the project already names that one.
func worktreeLabel(sv sessionView, projectPath string) string {
	if checkoutPath(sv.path, projectPath) == "" {
		return ""
	}
	return checkoutLabel(sv.path, projectPath, sv.projectID)
}

// applyLayoutStage carries out what the frame's clicks asked of the panes.
func (u *window) applyLayoutStage() {
	st := &u.layoutStage
	focus, zoom, unsplit, drop := st.focus, st.zoom, st.unsplit, st.drop
	st.focus, st.zoom, st.unsplit, st.drop = "", false, "", ui.SortDrop{}
	if id := st.switchTab; id != "" {
		st.switchTab = ""
		u.settings.hide()
		u.pulls.hide()
		u.showStripTab(id)
		if carried := u.sidebar.carry.Carrying(); carried != "" {
			u.model.showOtherThan(u.ctx, carried)
		}
	}
	if drop.ID != "" {
		u.dropOnPane(drop.ID, drop.Over)
	}
	if id := st.delegates; id != "" {
		st.delegates = ""
		u.layDelegatesOut(id)
	}
	if focus != "" {
		if active, ok := u.model.activeSession(); !ok || active.ID != focus {
			u.model.setActive(u.ctx, focus)
		}
	}
	if zoom {
		u.model.toggleZoom()
	}
	if unsplit != "" {
		u.model.unsplit(u.ctx, unsplit)
	}
	if focus != "" || zoom || unsplit != "" {
		u.model.invalidate()
	}
}

// content shows session s's terminal over gtx's area, keys typing into it
// when it takes them.
func (u *window) content(gtx C, s sessionView, keys bool) error {
	resume, ok := "", u.terms.has(s.ID)
	if !ok {
		resume, ok = u.gate.release(u.ctx, s)
	}
	if !ok {
		return nil
	}
	t, err := u.terms.show(u.ctx, gtx, s, resume)
	if err != nil {
		return err
	}
	err = u.terms.layout(gtx, t, keys)
	// A closed session frees its terminal off the frame goroutine; one shown
	// since is replaced on the next frame, which the window must survive.
	if errors.Is(err, gioterm.ErrClosed) {
		gtx.Execute(op.InvalidateCmd{})
		return nil
	}
	return err
}
