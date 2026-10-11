package main

import (
	"image"
	"image/color"
	"log"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.

// footerItemIcons is FooterLayoutEditor's ICONS: the glyph each chip wears
// beside its example reading.
var footerItemIcons = map[string]*icons.Icon{
	"attach": icAttach, "files": icCode, "changes": icons.Lucide("file-diff"), "pr": icPR,
	"checkout": icBranch, "path": icFolder, "model": icons.Lucide("cpu"), "context": icGauge,
	"plan": icGauge, "cost": icons.Lucide("coins"), "handsOn": icTimer, "clock": icons.Lucide("clock"),
}

var footerZoneLabels = map[string]string{footerLeft: "Left", footerRight: "Right", footerAvailable: "Available"}

// placeholderOpacity is the chip left behind's opacity-30 while it is dragged.
const placeholderOpacity = 0.3

// footerEditor is the Customize footer dialog: FooterSettings' row and the
// FooterLayoutEditor under it.
//
// ponytail: no FooterLayoutPreview and no spend ceiling. The real footer
// stays in sight under the dialog's light scrim and takes every change live,
// and the ceiling lives in the web's localStorage, out of this window's reach.
type footerEditor struct {
	open     bool
	dialog   ui.DialogState
	sortable ui.Sortable
	restore  widget.Clickable
}

// editorLayout draws the dialog when open. Call it before the window's
// content, so it takes Escape ahead of the terminal.
func (f *footer) editorLayout(gtx C, th *ui.Theme, in footerInput) {
	e := &f.editor
	l, saving := f.snapshot()
	ready := f.cost.isReady()
	disabled := !ready || saving
	if e.restore.Clicked(gtx) && !disabled {
		f.change(defaultFooterLayout())
	}
	description := "Drag an item between the sides, or out to hide it. Applies to every project."
	switch {
	case !ready:
		description = "Loading saved choices…"
	case saving:
		description = "Saving…"
	}
	pop := th.On(th.Popover)
	d := th.Dialog(&e.dialog, e.open, "Footer", description)
	d.MaxWidth = ui.DialogWidthXL
	d.Body = func(gtx C) D { return f.editorBody(gtx, pop, l, disabled, in) }
	restore := pop.Button(&e.restore, "Restore default")
	restore.Variant, restore.Size, restore.Disabled = ui.VariantGhost, ui.SizeXS, disabled
	d.Actions = []layout.Widget{restore.Layout}
	if d.Layout(gtx) {
		e.open = false
	}
}

// editorBody is the two sides, one column per end of the real footer, over
// the items on neither.
func (f *footer) editorBody(gtx C, th *ui.Theme, l footerLayout, disabled bool, in footerInput) D {
	s := &f.editor.sortable
	s.Disabled = disabled
	if d, ok := s.Update(gtx); ok && !disabled {
		f.drop(d)
	}
	sides := func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, f.zone(th, footerLeft, l.Left, in)),
			ui.Gap(3),
			layout.Flexed(1, f.zone(th, footerRight, l.Right, in)),
		)
	}
	return stack(gtx, 4, []layout.Widget{sides, f.zone(th, footerAvailable, availableItems(l), in)})
}

// drop applies a drag that landed, as FooterLayoutEditor's onDragEnd does.
func (f *footer) drop(d ui.SortDrop) {
	l, _ := f.snapshot()
	if next, moved := moveFooterItem(l, d.ID, d.Over); moved {
		f.change(next)
	}
}

// change is FooterSettings' change: one write at a time, the cost flag
// first when the cost item crossed in or out, since it also turns pricing
// on or off, then the layout, applied at once and saved.
func (f *footer) change(next footerLayout) {
	f.mu.Lock()
	if f.saving || !f.cost.isReady() {
		f.mu.Unlock()
		return
	}
	f.saving = true
	f.mu.Unlock()
	go func() {
		if err := f.commit(next); err != nil {
			// ponytail: logged; the web toasts it, and this window has no toasts yet.
			log.Printf("Could not save the footer: %v", err)
		}
	}()
}

func (f *footer) commit(next footerLayout) error {
	defer func() {
		f.mu.Lock()
		f.saving = false
		f.mu.Unlock()
		f.invalidate()
	}()
	if cost := next.has("cost"); cost != f.cost.on() {
		if err := f.cost.setOn(f.ctx, cost); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.layout = next
	f.mu.Unlock()
	f.invalidate()
	return saveFooterLayout(f.ctx, f.client, next)
}

// availableItems is every item in neither side, in FOOTER_ITEMS order.
func availableItems(l footerLayout) []string {
	var out []string
	for _, item := range footerItems {
		if !l.has(item.ID) {
			out = append(out, item.ID)
		}
	}
	return out
}

// zone is FooterZoneView: a text-xs heading over the drop area.
func (f *footer) zone(th *ui.Theme, zone string, items []string, in footerInput) layout.Widget {
	return func(gtx C) D {
		box := func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return f.editor.sortable.Zone(gtx, zone, func(gtx C) D { return f.zoneBox(gtx, th, zone, items, in) })
		}
		return stack(gtx, 1.5, []layout.Widget{th.Text(ui.TextXS, footerZoneLabels[zone]).Muted().Layout, box})
	}
}

// zoneBox is the drop area: flex-wrap content-start gap-1.5 rounded-lg p-2,
// bg-sidebar min-h-18 on a side, min-h-11 for available, ring-2 ring-ring
// only while the item in the air would land on it.
func (f *footer) zoneBox(gtx C, th *ui.Theme, zone string, items []string, in footerInput) D {
	var bg color.NRGBA
	minHeight := ui.Space(11)
	if zone != footerAvailable {
		bg, minHeight = th.Sidebar, ui.Space(18)
	}
	on := onFill(th, bg)
	justify := ui.JustifyStart
	if zone == footerRight {
		justify = ui.JustifyEnd
	}
	kids := make([]ui.WrapChild, 0, len(items)+1)
	for _, id := range items {
		kids = append(kids, ui.WrapItem(f.chip(on, id, zone != footerAvailable, in)))
	}
	if len(items) == 0 {
		empty := "Drag items here"
		if zone == footerAvailable {
			empty = "Every item is in the footer"
		}
		label := on.Text(ui.TextXS, empty).Muted().Layout
		kids = append(kids, ui.WrapItem(func(gtx C) D { return layout.Inset{Left: ui.Space(1), Right: ui.Space(1)}.Layout(gtx, label) }))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			if f.editor.sortable.Over() == zone {
				ringOutside(gtx, th.Ring, 2, ui.RadiusLG)
			}
			return fillIf(gtx, bg, ui.RadiusLG)
		},
		func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(minHeight)
			return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
				return ui.Wrap{GapX: ui.Space(1.5), GapY: ui.Space(1.5), Justify: justify, ContentStart: true}.Layout(gtx, kids...)
			})
		},
	)
}

// ringOutside is a ring-* box-shadow width wide around the minimum
// constraint, drawn before the box it rings.
func ringOutside(gtx C, c color.NRGBA, width, radius unit.Dp) {
	w := gtx.Dp(width)
	box := image.Rectangle{Min: image.Pt(-w, -w), Max: gtx.Constraints.Min.Add(image.Pt(w, w))}
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(box, gtx.Dp(radius)+w).Op(gtx.Ops))
}

// chip is FooterEditorItem: the item's reading on a ghost xs button with an
// inset ring-border, bg-background once in the footer and transparent and
// muted before; ring-ring on focus and as the keyboard's target, opacity-30
// while it is the placeholder of the chip in the air.
func (f *footer) chip(th *ui.Theme, id string, inFooter bool, in footerInput) layout.Widget {
	s := &f.editor.sortable
	lifted := func(gtx C) D {
		return chipBox(gtx, th.On(th.Popover), ui.Over(th.Popover, th.Border), th.Popover, th.Foreground, id, in)
	}
	return func(gtx C) D {
		bg, fg, ring := th.Surface, th.MutedForeground, th.Over(th.Border)
		if inFooter {
			bg, fg = th.Background, th.Foreground
		}
		if s.Focused(id) || s.Over() == id {
			ring = th.Ring
		}
		if s.Dragging() == id {
			fade := func(c color.NRGBA) color.NRGBA { return th.Over(ui.Alpha(c, placeholderOpacity)) }
			bg, fg, ring = fade(bg), fade(fg), fade(ring)
		}
		return s.Item(gtx, id, func(gtx C) D { return chipBox(gtx, th.On(bg), ring, bg, fg, id, in) }, lifted)
	}
}

// chipBox is buttonVariants ghost xs (h-6 px-2 gap-1 text-xs rounded-md,
// size-3 glyph) inside a 1px ring.
func chipBox(gtx C, th *ui.Theme, ring, bg, fg color.NRGBA, id string, in footerInput) D {
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, ring, ui.RadiusMD) },
		func(gtx C) D {
			return layout.UniformInset(1).Layout(gtx, func(gtx C) D {
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD-1) },
					func(gtx C) D {
						gtx.Constraints.Min.Y = gtx.Dp(ui.Space(6)) - 2
						return layout.Inset{Left: ui.Space(2) - 1, Right: ui.Space(2) - 1}.Layout(gtx, func(gtx C) D {
							return layout.W.Layout(gtx, func(gtx C) D { return itemReading(gtx, th, id, fg, in) })
						})
					},
				)
			})
		},
	)
}

// itemReading is ItemReading: the glyph and example the item shows, except
// the model, which shows the active session's own provider and model once
// it has reported them.
func itemReading(gtx C, th *ui.Theme, id string, fg color.NRGBA, in footerInput) D {
	ic, size, text := footerItemIcons[id], ui.Space(3), footerExample(id)
	model, _, hasModel := sessionModelText(in.usage)
	if id == "model" && in.hasUsage && hasModel && in.provider != "" {
		ic, size, text = providerIcon(in.provider), 14, model
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, size, fg) }),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, text).In(fg).Layout),
	)
}

func footerExample(id string) string {
	for _, item := range footerItems {
		if item.ID == id {
			return item.Example
		}
	}
	return id
}
