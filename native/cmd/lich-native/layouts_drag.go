package main

import (
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// Rearranging a layout by dragging: a pane by its header, or a card from the
// sidebar, onto a pane's edge or middle. Both ride the sidebar's SortCarry,
// whose targets here are each pane's five drop zones.

// paneTargetPrefix starts a drop zone's carry target id: pane|<session>|<edge>.
const paneTargetPrefix = "pane|"

// dropZoneShare is how much of a pane's width or height each edge zone
// takes; the middle keeps the rest.
const dropZoneShare = 4

var dropLabel = unit.Dp(6)

func paneTargetID(session string, edge paneEdge) string {
	return paneTargetPrefix + session + "|" + strconv.Itoa(int(edge))
}

// parsePaneTarget reads a drop zone's id, false for any other target.
func parsePaneTarget(id string) (session string, edge paneEdge, ok bool) {
	rest, found := strings.CutPrefix(id, paneTargetPrefix)
	if !found {
		return "", 0, false
	}
	at := strings.LastIndexByte(rest, '|')
	if at <= 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(rest[at+1:])
	if err != nil || n < int(edgeLeft) || n > int(edgeCenter) {
		return "", 0, false
	}
	return rest[:at], paneEdge(n), true
}

// paneZones are r's drop zones by edge: a band a dropZoneShare-th deep along
// each side, the top and bottom ones between the side bands, and the middle.
func paneZones(r image.Rectangle) [edgeCenter + 1]image.Rectangle {
	dx, dy := r.Dx()/dropZoneShare, r.Dy()/dropZoneShare
	var z [edgeCenter + 1]image.Rectangle
	z[edgeLeft] = image.Rect(r.Min.X, r.Min.Y, r.Min.X+dx, r.Max.Y)
	z[edgeRight] = image.Rect(r.Max.X-dx, r.Min.Y, r.Max.X, r.Max.Y)
	z[edgeTop] = image.Rect(r.Min.X+dx, r.Min.Y, r.Max.X-dx, r.Min.Y+dy)
	z[edgeBottom] = image.Rect(r.Min.X+dx, r.Max.Y-dy, r.Max.X-dx, r.Max.Y)
	z[edgeCenter] = image.Rect(r.Min.X+dx, r.Min.Y+dy, r.Max.X-dx, r.Max.Y-dy)
	return z
}

// dropPreview is the part of r a pane dropped on edge would take.
func dropPreview(r image.Rectangle, edge paneEdge) image.Rectangle {
	cx, cy := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
	switch edge {
	case edgeLeft:
		r.Max.X = cx
	case edgeRight:
		r.Min.X = cx
	case edgeTop:
		r.Max.Y = cy
	case edgeBottom:
		r.Min.Y = cy
	}
	return r
}

// dragPointer is where the pointer is while anything is carried over the
// window, read by a handler on the whole window: an ancestor of every area,
// it hears a drag whatever the drag started on. The drop zone under it is
// worked out from where the panes were drawn, rather than from Enter events
// on each zone: Gio's transfer delivers those unreliably to targets laid out
// once a drag is already under way.
type dragPointer struct {
	tag int
	at  image.Point
	// rects are where each pane, or the session alone, is drawn this frame,
	// in window pixels, and last where they were drawn the frame before,
	// which the pointer and a split are measured against; whole is a session
	// alone, not a pane of a layout.
	rects, last map[string]image.Rectangle
	whole, was  map[string]bool
	// paneW, paneH and gap are the smallest pane and the gap between two, in
	// window pixels.
	paneW, paneH, gap int
	// hover is the drop zone under the pointer, "" for none.
	hover string
	// released is the pointer let go this frame.
	released bool
	// tab is the strip tab the pointer has rested on since tabSince while
	// carrying, which comes on screen once it has rested tabDwell.
	tab      string
	tabSince time.Time
}

// tabDwell is how long something carried rests on a tab before that tab
// comes on screen, as a browser switches to the tab a dragged file rests on:
// long enough that passing over the strip switches nothing.
const tabDwell = 600 * time.Millisecond

// readDragPointer follows the pointer and, when what is carried is let go
// over a drop zone, records the drop for applyLayoutStage. Call it before
// the frame lays anything out: it reads where the panes were drawn last.
func (u *window) readDragPointer(gtx C) {
	st := &u.layoutStage
	d := &st.pointer
	d.last, d.was = d.rects, d.whole
	d.rects, d.whole = map[string]image.Rectangle{}, map[string]bool{}
	m := st.metric
	d.paneW, d.paneH, d.gap = m.Dp(paneMinWidth), m.Dp(paneMinHeight), m.Dp(gutter)
	u.sidebar.carry.Hold(gtx)
	carried := u.sidebar.carry.Carrying()
	d.released = false
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &d.tag, Kinds: pointer.Move | pointer.Drag | pointer.Release})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		d.at = e.Position.Round()
		d.released = d.released || e.Kind == pointer.Release
		if e.Kind == pointer.Release && carried != "" {
			if z := d.zoneAt(d.at, carried); z != "" {
				st.drop = ui.SortDrop{ID: carried, Over: z}
			}
		}
	}
	d.hover = ""
	if carried != "" {
		d.hover = d.zoneAt(d.at, carried)
	}
	u.dwellOnTab(gtx, carried)
}

// dwellOnTab switches to the strip tab something carried has rested on for
// tabDwell, the drop then landing on that tab's panes.
func (u *window) dwellOnTab(gtx C, carried string) {
	d := &u.layoutStage.pointer
	id := ""
	if carried != "" && !d.released {
		id = u.topbar.tabAt(d.at)
	}
	if _, active := u.model.stripTabs(); id == active {
		id = ""
	}
	switch {
	case id == "":
		d.tab = ""
	case id != d.tab:
		d.tab, d.tabSince = id, gtx.Now
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(tabDwell)})
	case gtx.Now.Sub(d.tabSince) >= tabDwell:
		u.layoutStage.switchTab, d.tab = id, ""
	default:
		gtx.Execute(op.InvalidateCmd{At: d.tabSince.Add(tabDwell)})
	}
}

// drawCarried draws the pill that follows the pointer while a session is
// carried: a pane dragged by its header anywhere, a card once it is past the
// sidebar, which draws it itself while it is over the list.
func (u *window) drawCarried(gtx C) {
	carried := u.sidebar.carry.Carrying()
	d := &u.layoutStage.pointer
	if carried == "" || u.layoutStage.sort.Dragging() == "" && d.at.X < u.islandAt.X-gtx.Dp(gutter) {
		return
	}
	sv, ok := u.model.viewsOf([]string{carried}, gtx.Now)[carried]
	if !ok {
		return
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	u.sidebar.inHand(gtx, u.theme, &sv, d.at)
}

// endStrandedDrag lets go of a drag whose own list was not laid out to hear
// the pointer go up, as when the tab it started on is no longer on screen.
func (u *window) endStrandedDrag(gtx C) {
	if u.layoutStage.pointer.released && u.sidebar.carry.Carrying() != "" {
		u.sidebar.carry.Release()
		gtx.Execute(op.InvalidateCmd{})
	}
}

// fits reports whether target's pane, as last drawn, holds two panes of the
// smallest size side by side for edge left or right, stacked for top or
// bottom; the middle swaps and always fits.
func (d *dragPointer) fits(target string, edge paneEdge) bool {
	r, ok := d.last[target]
	if !ok {
		return false
	}
	switch edge {
	case edgeLeft, edgeRight:
		return r.Dx() >= 2*d.paneW+d.gap
	case edgeTop, edgeBottom:
		return r.Dy() >= 2*d.paneH+d.gap
	}
	return true
}

// zoneAt is the drop zone at p of a pane other than carried, "" for none.
// Only a pane of the layout on screen swaps with another, through the
// middle; a card from the sidebar or a session alone has none.
func (d *dragPointer) zoneAt(p image.Point, carried string) string {
	_, carriedIsPane := d.last[carried]
	carriedIsPane = carriedIsPane && !d.was[carried]
	for id, r := range d.last {
		if id == carried || !p.In(r) {
			continue
		}
		var allowed [edgeCenter + 1]bool
		for _, edge := range []paneEdge{edgeLeft, edgeRight, edgeTop, edgeBottom} {
			allowed[edge] = d.fits(id, edge)
		}
		allowed[edgeCenter] = carriedIsPane && !d.was[id]
		if edge, ok := pickZone(r, p, allowed); ok {
			return paneTargetID(id, edge)
		}
	}
	return ""
}

// pickZone is the zone of r the pointer at p is in, among those allowed: the
// middle while inside its band, else the nearest allowed edge, measured as a
// share of r's width or height, so an edge left out gives its area to the
// others.
func pickZone(r image.Rectangle, p image.Point, allowed [edgeCenter + 1]bool) (paneEdge, bool) {
	if allowed[edgeCenter] && p.In(paneZones(r)[edgeCenter]) {
		return edgeCenter, true
	}
	share := map[paneEdge]float64{
		edgeLeft:   float64(p.X-r.Min.X) / float64(r.Dx()),
		edgeRight:  float64(r.Max.X-p.X) / float64(r.Dx()),
		edgeTop:    float64(p.Y-r.Min.Y) / float64(r.Dy()),
		edgeBottom: float64(r.Max.Y-p.Y) / float64(r.Dy()),
	}
	best, found := edgeLeft, false
	for _, edge := range []paneEdge{edgeLeft, edgeRight, edgeTop, edgeBottom} {
		if allowed[edge] && (!found || share[edge] < share[best]) {
			best, found = edge, true
		}
	}
	return best, found
}

// paneDropZones records where target is drawn, local in gtx and at in the
// window, and draws where the carried pane would land while the pointer is
// over one of its zones. whole is a session alone, not a pane of a layout.
func (u *window) paneDropZones(gtx C, target string, local, at image.Rectangle, whole bool) {
	d := &u.layoutStage.pointer
	if d.rects == nil {
		return
	}
	d.rects[target], d.whole[target] = at, whole
	over, edge, ok := parsePaneTarget(d.hover)
	if !ok || over != target {
		return
	}
	u.dropHighlight(gtx, dropPreview(local, edge), dropText(edge))
}

func dropText(edge paneEdge) string {
	switch edge {
	case edgeLeft:
		return i18n.T("layouts.drop.left")
	case edgeRight:
		return i18n.T("layouts.drop.right")
	case edgeTop:
		return i18n.T("layouts.drop.top")
	case edgeBottom:
		return i18n.T("layouts.drop.bottom")
	}
	return i18n.T("layouts.drop.swap")
}

// dropHighlight rings r and names the drop in its middle. It leaves the
// terminal under it unfilled: a translucent fill blends in linear light here
// and greys the text out, an opaque one hides it.
func (u *window) dropHighlight(gtx C, r image.Rectangle, text string) {
	th := u.theme
	rr := gtx.Dp(paneRadius)
	shape := clip.UniformRRect(r, rr)
	paint.FillShape(gtx.Ops, th.Ring, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
	m := op.Record(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Constraints{Max: r.Size()}
	on := th.On(th.Popover)
	d := layout.Background{}.Layout(lgtx,
		func(gtx C) D { return ui.Fill(gtx, th.Popover, ui.RadiusMD) },
		func(gtx C) D {
			return layout.UniformInset(dropLabel).Layout(gtx, on.Text(ui.TextXS, text).Weight(font.Medium).Layout)
		},
	)
	call := m.Stop()
	defer op.Offset(r.Min.Add(r.Size().Sub(d.Size).Div(2))).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// dropOnPane carries out a drop of session on a pane's zone, which
// zoneAt offered only where it fits: a pane of the same layout moves or
// swaps, a session in another layout is asked about, any other splits the
// pane.
func (u *window) dropOnPane(session, over string) {
	target, edge, ok := parsePaneTarget(over)
	if !ok || target == session {
		return
	}
	layoutID := u.model.layoutOf(target)
	from := u.model.layoutOf(session)
	if layoutID != "" && from == layoutID {
		u.model.movePane(u.ctx, session, target, edge)
		u.model.invalidate()
		return
	}
	u.besidePicked(besideRequest{target: target, edge: edge}, session)
}
