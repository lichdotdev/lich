package main

import (
	"image"
	"image/color"
	"log"
	"slices"
	"sync/atomic"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	driver "github.com/lichdotdev/gio-driver"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// tourSeenKey is the global setting set once the first-run tour has ended,
// finished or skipped, so it never opens by itself again.
const tourSeenKey = "tour.basics.seen"

// tourPopoverWidth is the mockup's 320px popover.
var tourPopoverWidth = ui.Space(80)

// tourScrim is driver.js's default overlay opacity; the palette's 0.5 barely
// shows over the dark theme, which Gio blends in linear light.
const tourScrim = 0.7

// tourMotion is DESIGN.md › Motion: Presence's 160ms in and 180ms out.
var tourMotion = driver.Motion{Enter: 160 * time.Millisecond, Exit: ui.PresenceExit, Glide: driver.DefaultMotion.Glide}

// tourText is a step's words by catalog key, read when the popover draws so a
// language switch mid-tour reaches it.
type tourText struct{ title, body, hint string }

// tourStep is one Basics step. shell, when set, is its text over a project
// with no agent, whose New session opens a terminal.
type tourStep struct {
	anchor string
	side   driver.Side
	text   tourText
	shell  *tourText
	until  func(u *window) bool
	// layer is how many dialogs and menus the anchor sits under: a step in
	// the New Session menu is 1, one in a dialog's own dropdown 2. reopen is
	// the step that opens them, where the tour goes back when they close
	// before the step is done.
	layer  int
	reopen int
}

// guide is one tour of the Guides menu.
type guide struct {
	// id names the guide for the menu and the palette; order sorts the menu.
	id    string
	order int
	// label is the menu entry's catalog key.
	label string
	steps []tourStep
	// ready, when set, is whether the guide can run in the window as it is;
	// when it cannot, the menu shows it disabled with unavailable, a key.
	ready       func(u *window) bool
	unavailable string
}

// guides is every guide, in menu order; each guide_*.go registers its own.
var guides []*guide

func registerGuide(g *guide) {
	guides = append(guides, g)
	slices.SortStableFunc(guides, func(a, b *guide) int { return a.order - b.order })
}

func guideByID(id string) *guide {
	for _, g := range guides {
		if g.id == id {
			return g
		}
	}
	return nil
}

// guideItem is a guide as the menu shows it, its words read this frame.
type guideItem struct {
	id, label, unavailable string
}

func (u *window) guideItems() []guideItem {
	items := make([]guideItem, len(guides))
	for i, g := range guides {
		items[i] = guideItem{id: g.id, label: i18n.T(g.label)}
		if g.ready != nil && !g.ready(u) {
			items[i].unavailable = i18n.T(g.unavailable)
		}
	}
	return items
}

// windowTour is the window's guided tour and when it opens by itself.
type windowTour struct {
	driver.Tour
	// guide is the one running or last run.
	guide *guide
	// missing is since when the current step's dialog or menu is closed.
	missing time.Time
	// offered is the first-run check made; due asks the next frame to start.
	offered bool
	due     atomic.Bool
	skip    widget.Clickable
}

// decideFirstTour is whether the first-run tour opens: on a launch that
// showed the provider setup, once it is closed, unless a tour ran before.
func decideFirstTour(setupShown, setupDone bool, seen string) bool {
	return setupShown && setupDone && seen == ""
}

// frameTour runs the tour's frame before the window lays out: it opens the
// first-run tour when due, reads how it ended, and steps it aside while a
// dialog or a menu is on screen: a menu opened through the hole would be
// under the scrim.
func (u *window) frameTour(gtx C) {
	u.offerTour()
	if u.tour.due.CompareAndSwap(true, false) {
		u.startGuide(basicsID)
	}
	for {
		e, ok := u.tour.Update(gtx)
		if !ok {
			break
		}
		u.tourEnded(e)
	}
	u.theme.Overlays.Frame()
	u.layerTour(gtx.Now)
}

// tourReopenGrace is how long a step's dialog or menu may be missing before
// the tour goes back to the step that opens it: one menu closing as the
// dialog it chose opens leaves a frame or two with neither.
const tourReopenGrace = 300 * time.Millisecond

// layerTour steps the tour aside for a dialog or menu over its step, and
// sends it back to reopen one its step lives in when that one closes early.
func (u *window) layerTour(now time.Time) {
	shown, layer, reopen := u.theme.Overlays.Shown(), 0, 0
	if u.tour.Active() && u.tour.guide != nil {
		s := u.tour.guide.steps[u.tour.Index()]
		layer, reopen = s.layer, s.reopen
	}
	if shown > layer {
		u.tour.Pause()
	} else {
		u.tour.Resume()
	}
	if shown >= layer {
		u.tour.missing = time.Time{}
		return
	}
	if u.tour.missing.IsZero() {
		u.tour.missing = now
	} else if now.Sub(u.tour.missing) > tourReopenGrace {
		u.tour.missing = time.Time{}
		u.tour.GoTo(reopen)
	}
}

func (u *window) offerTour() {
	if u.tour.offered || !u.setup.shown || !u.setup.done {
		return
	}
	u.tour.offered = true
	go func() {
		seen, err := u.client.GetSetting(u.ctx, tourSeenKey, "")
		if err != nil {
			log.Printf("tour: seen: %v", err)
			return
		}
		if decideFirstTour(true, true, seen) {
			u.tour.due.Store(true)
			u.toaster.Invalidate()
		}
	}()
}

// startGuide opens the guide named id from its first step; a step whose
// action is already done moves on by itself.
func (u *window) startGuide(id string) {
	g := guideByID(id)
	if g == nil {
		return
	}
	steps := make([]driver.Step, len(g.steps))
	for i, s := range g.steps {
		steps[i] = driver.Step{Anchor: s.anchor, Side: s.side}
		if s.until != nil {
			steps[i].Until = func() bool { return s.until(u) }
		}
	}
	u.tour.guide, u.tour.Steps, u.tour.Popover = g, steps, u.tourPopover
	u.tour.Spotlight = driver.Spotlight{Scrim: ui.Alpha(color.NRGBA{A: 0xff}, tourScrim), Padding: ui.Space(1.5), Radius: ui.RadiusLG, Offset: ui.Space(2), Margin: ui.Space(2)}
	u.tour.Motion = &tourMotion
	u.tour.Start()
}

// tourEnded remembers the first-run tour as seen and, when a tour was
// skipped, says where to find it again.
func (u *window) tourEnded(e driver.Event) {
	go func() {
		if err := u.client.SetSetting(u.ctx, tourSeenKey, "", "1"); err != nil {
			log.Printf("tour: save seen: %v", err)
		}
	}()
	if !e.Completed {
		u.toaster.Show(ui.Toast{Title: i18n.T("tour.skipped.title"), Description: i18n.T("tour.skipped.body")})
	}
}

func (u *window) tourProjectOpen() bool { return !u.model.onHome() }

func (u *window) tourSessionOpen() bool {
	_, ok := u.model.activeSession()
	return ok
}

// spot is u.tour.Anchor, for the views that point at a widget.
func (u *window) spot(gtx C, id string, w layout.Widget) D { return u.tour.Anchor(gtx, id, w) }

// tourWords is step's text, the terminal's when no agent is on PATH.
func (u *window) tourWords(step int) tourText {
	s := u.tour.guide.steps[step]
	if s.shell != nil && u.model.newSessionKind() == "shell" {
		return *s.shell
	}
	return s.text
}

// tourPopover is the tour's step, drawn with the window's kit: a Popover
// surface with its title, body and hint, the progress and the actions.
func (u *window) tourPopover(gtx C, s driver.StepState) D {
	th := u.theme.On(u.theme.Popover)
	width := gtx.Dp(tourPopoverWidth)
	gtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			ui.Fill(gtx, th.Border, ui.RadiusLG)
			inner := gtx
			inner.Constraints.Min = gtx.Constraints.Min.Sub(image.Pt(2, 2))
			defer op.Offset(image.Pt(1, 1)).Push(gtx.Ops).Pop()
			ui.Fill(inner, th.Popover, ui.RadiusLG-1)
			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			return layout.UniformInset(ui.Space(4)).Layout(gtx, func(gtx C) D { return u.tourCard(gtx, th, s) })
		}),
	)
}

func (u *window) tourCard(gtx C, th *ui.Theme, s driver.StepState) D {
	words := u.tourWords(s.Index)
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return u.tourHeader(gtx, th, words, s) }),
		ui.VGap(1.5),
		layout.Rigid(th.Text(ui.TextSM, i18n.T(words.body)).Muted().Wrapped().Layout),
	}
	if words.hint != "" {
		rows = append(rows, ui.VGap(3), layout.Rigid(func(gtx C) D { return tourHint(gtx, th, i18n.T(words.hint)) }))
	}
	rows = append(rows, ui.VGap(4), layout.Rigid(func(gtx C) D { return u.tourActions(gtx, th, s) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (u *window) tourHeader(gtx C, th *ui.Theme, words tourText, s driver.StepState) D {
	closeB := th.Button(s.Close, "")
	closeB.Icon, closeB.Variant, closeB.Size = icMenuClose, ui.VariantGhost, ui.SizeIconXS
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, th.Text(ui.TextSM, i18n.T(words.title)).Weight(font.SemiBold).Wrapped().Layout),
		ui.Gap(2),
		layout.Rigid(closeB.Layout),
	)
}

// tourHint is the action a waiting step expects, by a dot in the tone for
// waiting on the user.
func tourHint(gtx C, th *ui.Theme, hint string) D {
	dot := gtx.Dp(ui.Space(1.5))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min = image.Pt(dot, dot)
			return ui.Fill(gtx, th.ToneWait, unit.Dp(dot))
		}),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextXS, hint).Wrapped().Layout),
	)
}

// tourActions is the progress, then Skip on a step that waits for its action,
// or Back and Next on one that does not.
func (u *window) tourActions(gtx C, th *ui.Theme, s driver.StepState) D {
	button := func(click *widget.Clickable, key string, v ui.Variant) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			b := th.Button(click, i18n.T(key))
			b.Variant, b.Size = v, ui.SizeSM
			return b.Layout(gtx)
		})
	}
	if u.tour.skip.Clicked(gtx) {
		s.Close.Click()
	}
	progress := i18n.T("tour.progress", i18n.Args{"index": s.Index + 1, "total": s.Total})
	children := []layout.FlexChild{layout.Flexed(1, th.Text(ui.TextXS, progress).Mono().Muted().Layout)}
	switch {
	case s.Waiting:
		children = append(children, button(&u.tour.skip, "tour.skip", ui.VariantGhost))
	case s.Last:
		children = append(children, tourBack(s, button)...)
		children = append(children, button(s.Next, "tour.done", ui.VariantDefault))
	default:
		children = append(children, tourBack(s, button)...)
		children = append(children, button(s.Next, "tour.next", ui.VariantDefault))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// tourBack is Back and its gap, when the step can go back.
func tourBack(s driver.StepState, button func(*widget.Clickable, string, ui.Variant) layout.FlexChild) []layout.FlexChild {
	if !s.CanBack {
		return nil
	}
	return []layout.FlexChild{button(s.Back, "tour.back", ui.VariantGhost), ui.Gap(2)}
}
