package main

import (
	"context"
	"image"
	"net/url"
	"time"

	"gioui.org/f32"
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

// The JavaScript dialog the page holds open, as a card floating over the top
// of the page, with no scrim: the page runs nothing until the person or the
// agent answers it. Option B of the approved mockup (2026-10-28).

var (
	dialogCardWidth  = unit.Dp(400)
	dialogCardRadius = unit.Dp(12)
	dialogCardPad    = ui.Space(4)
	dialogCardGap    = ui.Space(2.5)
	dialogCardTop    = ui.Space(3)
)

// dialogCardRing is the card's outline, ui.Dialog's ring-foreground/10.
const dialogCardRing = 0.1

// dialogMessageLines is how many lines of the page's message the card shows:
// a page can put anything in one, and the card must leave its buttons on screen.
const dialogMessageLines = 8

// dialogActions are the agent's answers to the page's dialog, which the
// cursor plays on the card's buttons.
var dialogActions = map[string]bool{"accept": true, "dismiss": true}

// dialogCard is the card of one page pane: the dialog it shows, its widgets,
// and where its buttons were last drawn, in pixels of the pane, for the
// agent's cursor to press.
type dialogCard struct {
	shown      *api.BrowserDialog
	ok, cancel widget.Clickable
	field      widget.Editor
	focusField bool
	okAt, noAt image.Rectangle
}

// sync takes the page's dialog: a new one replaces the card's, focusing a
// prompt's field with its text selected. None takes the card away, unless the
// agent's cursor still has to press its answer on it (holding): the state
// without the dialog follows the agent's answer at once, before the cursor
// played it.
func (d *dialogCard) sync(page *api.BrowserDialog, holding bool) {
	switch {
	case page != nil:
		if d.shown != nil && *d.shown == *page {
			return
		}
		shown := *page
		d.shown = &shown
		d.field.SingleLine, d.field.Submit = true, true
		d.field.SetText(page.Default)
		d.field.SetCaret(d.field.Len(), 0)
		d.focusField = page.Type == "prompt"
	case !holding:
		d.shown = nil
	}
}

// target is the button the agent's cursor presses for action: OK or Leave to
// accept, Cancel or Stay to dismiss; an alert has only OK.
func (d *dialogCard) target(action string) image.Rectangle {
	if action == "dismiss" && (d.shown == nil || d.shown.Type != "alert") {
		return d.noAt
	}
	return d.okAt
}

// point is the middle of target(action), or the card's top middle over a pane
// width wide whose page starts at top, before the card was ever drawn.
func (d *dialogCard) point(action string, width, top int) f32.Point {
	r := d.target(action)
	if r.Empty() {
		return f32.Pt(float32(width)/2, float32(top))
	}
	mid := r.Min.Add(r.Max).Div(2)
	return f32.Pt(float32(mid.X), float32(mid.Y))
}

// dialogKey is the answer a key gives a dialog of type kind: Enter accepts, Esc
// dismisses, but on an alert, whose only answer is OK, Esc accepts too. ok is
// false for any other key.
func dialogKey(kind string, name key.Name) (accept, ok bool) {
	switch name {
	case key.NameReturn, key.NameEnter:
		return true, true
	case key.NameEscape:
		return kind == "alert", true
	}
	return false, false
}

// dialogFrom is the card's muted first line: the host of the page that opened
// the dialog says it.
func dialogFrom(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return i18n.T("layouts.browser.dialogFrom", i18n.Args{"host": u.Host})
	}
	return i18n.T("layouts.browser.dialogFromPage")
}

// answering reports whether the agent's cursor still has an answer to the
// dialog to press at now: queued, or pressing.
func (p *browserPaneUI) answering(now time.Time) bool {
	for _, m := range p.cursor.pending {
		if dialogActions[m.action] {
			return true
		}
	}
	return dialogActions[p.flash.what] && now.Before(p.flash.at.Add(toolbarFlash))
}

// answerDialog is the person's answer to the dialog on the card: a prompt
// they accept returns its field's text.
func (u *window) answerDialog(p *browserPaneUI, session string, accept bool) {
	o := api.BrowserDialogOptions{ID: session, Accept: accept}
	if accept && p.dialog.shown != nil && p.dialog.shown.Type == "prompt" {
		text := p.dialog.field.Text()
		o.Text = &text
	}
	u.sendBrowser(session, "answer", func(ctx context.Context) error { return u.client.BrowserAnswer(ctx, o) })
}

// dialogPageKey takes the keys the page would get while the card is up: Enter
// and Esc answer it, and the rest go nowhere, since the page runs nothing. It
// reports whether it took e.
func (u *window) dialogPageKey(p *browserPaneUI, session string, e key.Event) bool {
	if p.dialog.shown == nil {
		return false
	}
	if accept, ok := dialogKey(p.dialog.shown.Type, e.Name); ok && e.State == key.Press {
		u.answerDialog(p, session, accept)
	}
	return true
}

// dialogCardAt draws the card over the page in gtx, whose top is top pixels
// down the pane.
func (u *window) dialogCardAt(gtx C, p *browserPaneUI, session string, top int) {
	d := &p.dialog
	if d.shown == nil {
		return
	}
	u.dialogCardInput(gtx, p, session)
	th := u.theme.On(u.theme.Popover)
	page := gtx.Constraints.Max
	width := min(gtx.Dp(dialogCardWidth), page.X-2*gtx.Dp(gutter))
	pad, gap := gtx.Dp(dialogCardPad), gtx.Dp(dialogCardGap)
	if width <= 2*pad {
		return
	}
	inner := gtx
	inner.Constraints = layout.Constraints{Min: image.Pt(width-2*pad, 0), Max: image.Pt(width-2*pad, page.Y)}
	bodyRec := op.Record(gtx.Ops)
	body := u.dialogCardBody(inner, th, d)
	bodyCall := bodyRec.Stop()
	ok, no := dialogButtons(gtx, th, p)
	rowY := pad + body.Size.Y + gap
	size := image.Pt(width, rowY+ok.size.Y+pad)
	at := image.Pt((page.X-width)/2, gtx.Dp(dialogCardTop))
	ok.at = image.Pt(width-pad-ok.size.X, rowY)
	no.at = image.Pt(ok.at.X-gtx.Dp(ui.Space(2))-no.size.X, rowY)
	d.okAt, d.noAt = ok.rect().Add(at).Add(image.Pt(0, top)), image.Rectangle{}
	if no.size.X > 0 {
		d.noAt = no.rect().Add(at).Add(image.Pt(0, top))
	}

	defer op.Offset(at).Push(gtx.Ops).Pop()
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(dialogCardRadius))
	paint.FillShape(gtx.Ops, th.Popover, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	// The card takes the pointer from the page under it.
	event.Op(gtx.Ops, d)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: d, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Scroll, ScrollY: noteScrollRange}); !ok {
			break
		}
	}
	placed{at: image.Pt(pad, pad), call: bodyCall}.add(gtx)
	ok.add(gtx)
	no.add(gtx)
	area.Pop()
	paint.FillShape(gtx.Ops, ui.Alpha(th.Foreground, dialogCardRing), clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
}

// placed is something laid out ahead, to draw at at.
type placed struct {
	at   image.Point
	size image.Point
	call op.CallOp
}

func (pl placed) rect() image.Rectangle { return image.Rectangle{Min: pl.at, Max: pl.at.Add(pl.size)} }

func (pl placed) add(gtx C) {
	defer op.Offset(pl.at).Push(gtx.Ops).Pop()
	pl.call.Add(gtx.Ops)
}

// dialogCardInput takes the person's answer from the card's buttons and
// field: Enter in a prompt's field is OK, Esc is Cancel.
func (u *window) dialogCardInput(gtx C, p *browserPaneUI, session string) {
	d := &p.dialog
	if d.ok.Clicked(gtx) {
		u.answerDialog(p, session, true)
	}
	if d.cancel.Clicked(gtx) {
		u.answerDialog(p, session, false)
	}
	for {
		ev, ok := d.field.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			u.answerDialog(p, session, true)
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &d.field, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			u.answerDialog(p, session, false)
		}
	}
	if d.focusField {
		d.focusField = false
		gtx.Execute(key.FocusCmd{Tag: &d.field})
	}
}

// dialogButtons lays out the card's buttons ahead of placing them: OK, primary,
// and Cancel; Leave, destructive, and Stay on a beforeunload; OK alone on an
// alert, where no is empty. The one the agent's cursor presses is lit.
func dialogButtons(gtx C, th *ui.Theme, p *browserPaneUI) (ok, no placed) {
	kind := p.dialog.shown.Type
	okLabel, noLabel, okVariant := i18n.T("layouts.browser.dialogOK"), i18n.T("common.action.cancel"), ui.VariantDefault
	if kind == "beforeunload" {
		okLabel, noLabel, okVariant = i18n.T("layouts.browser.dialogLeave"), i18n.T("layouts.browser.dialogStay"), ui.VariantDestructive
	}
	gtx.Constraints.Min = image.Point{}
	record := func(b ui.ButtonStyle) placed {
		rec := op.Record(gtx.Ops)
		dims := b.Layout(gtx)
		return placed{size: dims.Size, call: rec.Stop()}
	}
	okBtn := th.Button(&p.dialog.ok, okLabel)
	okBtn.Variant, okBtn.Lit = okVariant, p.flashing(gtx, "accept") || kind == "alert" && p.flashing(gtx, "dismiss")
	ok = record(okBtn)
	if kind != "alert" {
		noBtn := th.Button(&p.dialog.cancel, noLabel)
		noBtn.Variant, noBtn.Lit = ui.VariantOutline, p.flashing(gtx, "dismiss")
		no = record(noBtn)
	}
	return ok, no
}

// dialogCardBody is who says it, the message, and a prompt's field.
func (u *window) dialogCardBody(gtx C, th *ui.Theme, d *dialogCard) D {
	msg := th.Text(ui.TextSM, d.shown.Message)
	msg.MaxLines = dialogMessageLines
	children := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextXS, dialogFrom(d.shown.URL)).Muted().Layout),
		ui.VGap(0.5),
		layout.Rigid(msg.Layout),
	}
	if d.shown.Type == "prompt" {
		children = append(children, layout.Rigid(layout.Spacer{Height: dialogCardGap}.Layout),
			layout.Rigid(th.Input(&d.field, "").Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
