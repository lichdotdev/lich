package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"gioui.org/f32"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// notePick is the color of the pick, its outlines and its pins: one hue of its
// own, so a note is never taken for the agent's cursor, which wears the
// card's color or the theme's primary.
var notePick = cardColors["blue"]

// The tag and the halos sit on the website, not on lich, so they keep one
// look on every page whatever lich's theme: a dark tag, and white under each
// outline so it reads on a dark page too.
var (
	noteTagBg   = ui.OKLCH(0.21, 0.006, 285.885, 1)
	noteTagText = color.NRGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff}
	noteHalo    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xb0}
)

var (
	notePopoverWidth = unit.Dp(280)
	noteRowHeight    = ui.Space(6.5)
	noteTrayHead     = ui.Space(9)
	noteTrayPad      = ui.Space(2)
	// noteRowsShown is how many notes the tray shows before it scrolls.
	noteRowsShown = 3
)

// noteScrollRange takes every wheel turn over the popover and the tray, so it
// does not scroll the page under them.
var noteScrollRange = pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}

// notesTrayHeight is the tray's height for its notes; none hides it. The
// tray floats over the page instead of taking room from it: a page that
// shrank under each new note relaid itself out and blinked.
func notesTrayHeight(gtx C, n int) int {
	if n == 0 {
		return 0
	}
	return gtx.Dp(noteTrayHead) + gtx.Dp(noteTrayPad) + min(n, noteRowsShown)*gtx.Dp(noteRowHeight) + 2*gtx.Dp(noteTrayPad)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// notesOverlay draws, over the page, the outline of the element under the
// pointer with its name, every noted element's outline and pin, and the
// open note.
func (u *window) notesOverlay(gtx C, p *browserPaneUI, session, pageURL string) {
	n := &p.notes
	u.takeFound(gtx, p, session)
	if !n.picking && len(n.list) == 0 {
		return
	}
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	scale := gtx.Metric.PxPerDp
	lit := -1
	for i, note := range n.list {
		if note.row.Hovered() || (n.open && n.editing == i) {
			lit = i
		}
	}
	for i, note := range n.list {
		if !noteVisible(note, pageURL) {
			continue
		}
		ring := noteRing(gtx, cssRect(note.box, scale))
		if i == lit {
			ringFill(gtx, ring, withAlpha(notePick, 0x1a))
			dashedRing(gtx, ring, notePick, 1.5, !n.open)
		} else {
			ringFill(gtx, ring, withAlpha(notePick, 0x0d))
			dashedRing(gtx, ring, notePick, 1.5, false)
		}
	}
	if n.open && n.editing < 0 {
		ring := noteRing(gtx, cssRect(n.draftBox(), scale))
		ringFill(gtx, ring, withAlpha(notePick, 0x1a))
		// Still while the note is written: the pick is made, and redrawing the
		// window each frame for it cost a third of a core (measured 2026-10-27).
		dashedRing(gtx, ring, notePick, 1.5, false)
	}
	if n.picking && !n.open && n.hover.Label != "" {
		ring := noteRing(gtx, cssRect(n.hoverBox(gtx.Now), scale))
		if n.gliding(gtx.Now) {
			// Every frame for the glide's moment: the march's 20 a second
			// would step it.
			gtx.Execute(op.InvalidateCmd{})
		}
		ringFill(gtx, ring, withAlpha(notePick, 0x1a))
		dashedRing(gtx, ring, notePick, 1.5, true)
		u.hoverTag(gtx, ring, n.hover)
	}
	for i, note := range n.list {
		if r := cssRect(note.box, scale); noteVisible(note, pageURL) && r.Overlaps(image.Rectangle{Max: gtx.Constraints.Max}) {
			u.notePin(gtx, n, i, noteRing(gtx, r), i == lit)
		}
	}
	if n.open {
		u.notePopover(gtx, p, scale)
	}
}

// hoverTag names the element under the pointer above it, or below when it
// sits at the page's top: its label, component and size, in the inverted
// colors of a tooltip.
func (u *window) hoverTag(gtx C, r image.Rectangle, el api.BrowserElement) {
	th := u.theme
	macro := op.Record(gtx.Ops)
	tag := gtx
	tag.Constraints.Min = image.Point{}
	d := layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1), Left: ui.Space(1.5), Right: ui.Space(1.5)}.Layout(tag, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, el.Label).Mono().In(noteTagText).Layout),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, el.Component).In(withAlpha(noteTagText, 0xb3)).Layout),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf("%d×%d", el.Box.W, el.Box.H)).In(withAlpha(noteTagText, 0xb3)).Layout),
		)
	})
	call := macro.Stop()
	gap := gtx.Dp(ui.Space(1))
	at := image.Pt(r.Min.X, r.Min.Y-d.Size.Y-gap)
	if at.Y < 0 {
		at.Y = r.Max.Y + gap
	}
	at.X = min(max(at.X, 0), gtx.Constraints.Max.X-d.Size.X)
	at.Y = min(max(at.Y, 0), gtx.Constraints.Max.Y-d.Size.Y)
	defer op.Offset(at).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, noteTagBg, clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(ui.RadiusMD)).Op(gtx.Ops))
	call.Add(gtx.Ops)
}

// notePin is note i's number as a tab on top of its element's ring, at its
// left, the way DevTools labels an element; a click opens its
// note. The tab stays whole inside the page.
func (u *window) notePin(gtx C, n *browserNotes, i int, r image.Rectangle, lit bool) {
	note := n.list[i]
	if note.pin.Clicked(gtx) {
		n.openListed(gtx, i)
	}
	macro := op.Record(gtx.Ops)
	tab := gtx
	tab.Constraints.Min = image.Point{}
	d := note.pin.Layout(tab, func(gtx C) D { return u.noteTab(gtx, i+1, lit, true) })
	call := macro.Stop()
	page := gtx.Constraints.Max
	// On the ring's straight top edge, past its rounded corner.
	at := image.Pt(r.Min.X+gtx.Dp(noteRingRadius), r.Min.Y-d.Size.Y)
	at.X = min(max(at.X, 0), page.X-d.Size.X)
	at.Y = min(max(at.Y, 0), page.Y-d.Size.Y)
	defer op.Offset(at).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// noteTab is a note's number on the pick's color: square at the bottom when it
// sits on an outline (onOutline), rounded all round in the popover and the
// tray; lit widens it.
func (u *window) noteTab(gtx C, num int, lit, onOutline bool) D {
	pad := ui.Space(1.5)
	if lit {
		pad = ui.Space(2)
	}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	macro := op.Record(gtx.Ops)
	d := layout.Inset{Left: pad, Right: pad}.Layout(gtx, u.theme.Text(ui.TextXS, strconv.Itoa(num)).Mono().In(white).Weight(font.SemiBold).Layout)
	call := macro.Stop()
	radius := gtx.Dp(ui.Space(1))
	shape := clip.RRect{Rect: image.Rectangle{Max: d.Size}, NW: radius, NE: radius, SE: radius, SW: radius}
	if onOutline {
		shape.SE, shape.SW = 0, 0
	}
	paint.FillShape(gtx.Ops, notePick, shape.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return d
}

// notePopover is the open note beside its element: what it is, a line to
// check the pick by, the note, and its buttons.
func (u *window) notePopover(gtx C, p *browserPaneUI, scale float32) {
	n := &p.notes
	th := u.theme
	n.editor.Submit = true
	for {
		ev, ok := n.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			n.commit()
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &n.editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			n.open = false
		}
	}
	if n.add.Clicked(gtx) {
		n.commit()
	}
	if n.cancel.Clicked(gtx) {
		n.open = false
	}
	if n.del.Clicked(gtx) && n.editing >= 0 {
		n.drop(n.editing)
	}
	if !n.open {
		gtx.Execute(key.FocusCmd{Tag: &p.tag})
		return
	}
	if n.focusEditor {
		n.focusEditor = false
		gtx.Execute(key.FocusCmd{Tag: &n.editor})
	}
	width := gtx.Dp(notePopoverWidth)
	macro := op.Record(gtx.Ops)
	body := gtx
	body.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	d := u.notePopoverBody(body, n)
	call := macro.Stop()
	room := gtx.Constraints.Max
	if t := notesTrayHeight(gtx, len(n.list)); t > 0 {
		room.Y -= t + gtx.Dp(gutter)
	}
	el := cssRect(n.draftBox(), scale)
	at := notePopoverAt(el, d.Size, room, gtx.Dp(ui.Space(3)))
	defer op.Offset(at).Push(gtx.Ops).Pop()
	defer noteEnter(gtx, n.openedAt, el.Sub(at), d.Size)()
	shape := clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(ui.RadiusMD))
	paint.FillShape(gtx.Ops, th.Sidebar, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	// The popover takes the pointer from the page under it.
	event.Op(gtx.Ops, &n.draft)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &n.draft, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Scroll, ScrollY: noteScrollRange}); !ok {
			break
		}
	}
	call.Add(gtx.Ops)
	area.Pop()
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
}

// noteEnterDuration and noteZoomFrom are a lich menu's animate-in, slowed a
// little: the popover appears away from the pointer, beside the element.
const (
	noteEnterDuration = 160 * time.Millisecond
	noteZoomFrom      = 0.95
)

var noteEase = ui.CubicBezier(0.25, 0.1, 0.25, 1)

// noteEnter fades and zooms a popover or the tray in from its edge nearest
// el (local to it), sliding from that side, for noteEnterDuration after
// opened. The returned func pops what it pushed.
func noteEnter(gtx C, opened time.Time, el image.Rectangle, size image.Point) func() {
	elapsed := gtx.Now.Sub(opened)
	if elapsed >= noteEnterDuration || elapsed < 0 {
		return func() {}
	}
	gtx.Execute(op.InvalidateCmd{})
	p := noteEase(float32(elapsed) / float32(noteEnterDuration))
	mid := el.Min.Add(el.Max).Div(2)
	origin := f32.Pt(float32(min(max(mid.X, 0), size.X)), float32(min(max(mid.Y, 0), size.Y)))
	toward := f32.Pt(float32(mid.X)-origin.X, float32(mid.Y)-origin.Y)
	if l := float32(math.Hypot(float64(toward.X), float64(toward.Y))); l > 0 {
		toward = toward.Mul(1 / l)
	}
	slide := toward.Mul(float32(gtx.Dp(ui.Space(2))) * (1 - p))
	zoom := noteZoomFrom + (1-noteZoomFrom)*p
	transform := op.Affine(f32.Affine2D{}.Scale(origin, f32.Pt(zoom, zoom)).Offset(slide)).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, p)
	return func() {
		opacity.Pop()
		transform.Pop()
	}
}

// notePopoverAt places a popover of size beside the element at r: to its
// right, else under it, else above it, kept inside the page.
func notePopoverAt(r image.Rectangle, size, page image.Point, gap int) image.Point {
	at := image.Pt(r.Max.X+gap, r.Min.Y)
	if at.X+size.X > page.X-gap {
		at = image.Pt(r.Min.X, r.Max.Y+gap)
		if at.Y+size.Y > page.Y-gap {
			at.Y = r.Min.Y - gap - size.Y
		}
	}
	at.X = min(max(at.X, gap), page.X-size.X-gap)
	at.Y = min(max(at.Y, gap), page.Y-size.Y-gap)
	return at
}

// notePopoverBody is the review's comment box (commentComposer) for a note:
// a muted line naming the element, the field, then Add and Cancel; a note
// being edited adds Delete.
func (u *window) notePopoverBody(gtx C, n *browserNotes) D {
	on := u.theme.On(u.theme.Sidebar)
	num := len(n.list) + 1
	if n.editing >= 0 {
		num = n.editing + 1
	}
	heading := n.draft.Label
	if n.draft.Component != "" {
		heading += " · " + n.draft.Component
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return u.noteTab(gtx, num, false, false) }),
					ui.Gap(2),
					layout.Flexed(1, on.Text(ui.TextXS, heading).Mono().Muted().Layout),
				)
			}),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(ui.Space(64)))
				return on.Textarea(&n.editor, i18n.T("layouts.browser.notePlaceholder")).Layout(gtx)
			}),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D { return u.notePopoverButtons(gtx, on, n) }),
		)
	})
}

func (u *window) notePopoverButtons(gtx C, th *ui.Theme, n *browserNotes) D {
	label := i18n.T("layouts.browser.addNote")
	if n.editing >= 0 {
		label = i18n.T("layouts.browser.saveNote")
	}
	add := th.Button(&n.add, label)
	add.Size = ui.SizeSM
	cancel := th.Button(&n.cancel, i18n.T("common.action.cancel"))
	cancel.Size, cancel.Variant = ui.SizeSM, ui.VariantGhost
	buttons := []layout.FlexChild{layout.Rigid(add.Layout), ui.Gap(2), layout.Rigid(cancel.Layout)}
	if n.editing >= 0 {
		del := th.Button(&n.del, i18n.T("layouts.browser.deleteNote"))
		del.Size, del.Variant = ui.SizeSM, ui.VariantGhost
		buttons = append(buttons,
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(del.Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
}

// notesTray is the notes waiting, floating over the page's bottom, inset
// from the pane's edges by the gutter the island keeps around its panes: a
// row each, Clear and Send.
func (u *window) notesTray(gtx C, p *browserPaneUI, session string) {
	n := &p.notes
	th := u.theme
	if n.clear.Clicked(gtx) {
		n.list, n.open, n.failure = nil, false, ""
	}
	if n.send.Clicked(gtx) && len(n.list) > 0 {
		u.sendNotes(p, session)
	}
	for i := len(n.list) - 1; i >= 0; i-- {
		note := n.list[i]
		if note.remove.Clicked(gtx) {
			n.drop(i)
			continue
		}
		if note.row.Clicked(gtx) {
			n.openListed(gtx, i)
		}
	}
	height := notesTrayHeight(gtx, len(n.list))
	if height == 0 {
		n.trayShown = time.Time{}
		return
	}
	if n.trayShown.IsZero() {
		n.trayShown = gtx.Now
	}
	edge := gtx.Dp(gutter)
	page := gtx.Constraints.Max
	size := image.Pt(max(page.X-2*edge, 0), height)
	defer op.Offset(image.Pt(edge, page.Y-height-edge)).Push(gtx.Ops).Pop()
	defer noteEnter(gtx, n.trayShown, image.Rect(size.X/2, size.Y, size.X/2, size.Y+1), size)()
	gtx.Constraints = layout.Exact(size)
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(paneRadius))
	paint.FillShape(gtx.Ops, th.Background, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	// The tray takes the pointer from the page under it.
	event.Op(gtx.Ops, &n.rows)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &n.rows, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Scroll, ScrollY: noteScrollRange}); !ok {
			break
		}
	}
	layout.UniformInset(noteTrayPad).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.notesTrayHead(gtx, n, session) }),
			layout.Rigid(layout.Spacer{Height: noteTrayPad}.Layout),
			layout.Flexed(1, func(gtx C) D {
				return n.rows.Layout(gtx, len(n.list), func(gtx C, i int) D { return u.noteRow(gtx, n, i) })
			}),
		)
	})
	area.Pop()
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
}

func (u *window) notesTrayHead(gtx C, n *browserNotes, session string) D {
	th := u.theme
	gtx.Constraints.Min.Y = gtx.Dp(noteTrayHead)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return layout.Inset{Left: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, i18n.T("layouts.browser.notes", i18n.Args{"count": len(n.list)})).Weight(font.SemiBold).Layout),
			ui.Gap(1.5),
			layout.Flexed(1, func(gtx C) D {
				if n.failure != "" {
					return th.Text(ui.TextXS, n.failure).In(th.Destructive).Layout(gtx)
				}
				return th.Text(ui.TextXS, i18n.T("layouts.browser.notesFor", i18n.Args{"label": u.model.sessionLabel(session)})).Muted().Layout(gtx)
			}),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D {
				clear := th.Button(&n.clear, i18n.T("layouts.browser.clearNotes"))
				clear.Variant, clear.Size = ui.VariantGhost, ui.SizeSM
				return clear.Layout(gtx)
			}),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D {
				send := th.Button(&n.send, i18n.T("layouts.browser.sendNotes"))
				send.Icon, send.Size = icBatchSend, ui.SizeSM
				return send.Layout(gtx)
			}),
		)
	})
}

// noteRow is one note in the tray: its pin's number, the element and the
// note; hovering it lights the element on the page, a click opens the note.
func (u *window) noteRow(gtx C, n *browserNotes, i int) D {
	th := u.theme
	note := n.list[i]
	height := gtx.Dp(noteRowHeight)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	return note.row.Layout(gtx, func(gtx C) D {
		if note.row.Hovered() {
			ui.Fill(gtx, th.Accent, ui.RadiusMD)
		}
		text := note.text
		muted := th.MutedForeground
		if text == "" {
			text = i18n.T("layouts.browser.noNote")
		}
		return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return u.noteTab(gtx, i+1, false, false) }),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(ui.Space(48)))
					return th.Text(ui.TextXS, note.el.Label).Mono().Layout(gtx)
				}),
				ui.Gap(2.5),
				layout.Flexed(1, th.Text(ui.TextXS, text).In(muted).Layout),
				layout.Rigid(func(gtx C) D {
					return cornerButton(gtx, th, &note.remove, icNoteRemove, false)
				}),
			)
		})
	})
}
