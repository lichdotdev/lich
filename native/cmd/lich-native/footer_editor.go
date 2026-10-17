package main

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// footerItemIcons is FooterLayoutEditor's ICONS: the glyph each chip wears
// beside its example reading.
var footerItemIcons = map[string]*icons.Icon{
	"attach": icAttach, "files": icCode, "changes": icons.Lucide("file-diff"), "pr": icPR,
	"checkout": icBranch, "path": icFolder, "model": icons.Lucide("cpu"), "context": icGauge,
	"plan": icGauge, "cost": icons.Lucide("coins"), "handsOn": icTimer, "clock": icons.Lucide("clock"),
}

var footerZoneLabelKeys = map[string]string{
	footerLeft:      "settings.footerLayoutEditor.zone.left",
	footerRight:     "settings.footerLayoutEditor.zone.right",
	footerAvailable: "settings.footerLayoutEditor.zone.available",
}

// placeholderOpacity is the chip left behind's opacity-30 while it is dragged.
const placeholderOpacity = 0.3

// footerEditor is the state of Settings › Appearance › Footer, FooterSettings'
// row and the FooterLayoutEditor and FooterLayoutPreview under it.
type footerEditor struct {
	sortable ui.Sortable
	restore  widget.Clickable
}

// editorState is the row's description, and whether the editor takes no
// change: before the cost flag loads and while a change is being saved.
func (f *footer) editorState() (description string, disabled bool) {
	_, saving := f.snapshot()
	switch {
	case !f.cost.isReady():
		return i18n.T("settings.footerSettings.loading"), true
	case saving:
		return i18n.T("settings.footerSettings.saving"), true
	}
	return i18n.T("settings.footerSettings.description"), false
}

// restoreButton is the row's Restore default.
func (f *footer) restoreButton(gtx C, th *ui.Theme) D {
	_, disabled := f.editorState()
	if f.editor.restore.Clicked(gtx) && !disabled {
		f.change(defaultFooterLayout())
	}
	b := th.Button(&f.editor.restore, i18n.T("settings.footerSettings.restoreDefault"))
	b.Variant, b.Size, b.Disabled = ui.VariantGhost, ui.SizeXS, disabled
	return b.Layout(gtx)
}

// editorPanel is the editor and, mt-5 under it, the preview.
func (f *footer) editorPanel(gtx C, th *ui.Theme) D {
	l, _ := f.snapshot()
	_, disabled := f.editorState()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return f.editorBody(gtx, th, l, disabled, f.shown) }),
		ui.VGap(5),
		layout.Rigid(func(gtx C) D { return f.previewLayout(gtx, th, l, f.shown) }),
	)
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
			f.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.footerSettings.saveFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
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
		return stack(gtx, 1.5, []layout.Widget{th.Text(ui.TextXS, i18n.T(footerZoneLabelKeys[zone])).Muted().Layout, box})
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
		empty := i18n.T("settings.footerLayoutEditor.dragHere")
		if zone == footerAvailable {
			empty = i18n.T("settings.footerLayoutEditor.allPlaced")
		}
		label := on.Text(ui.TextXS, empty).Muted().Layout
		kids = append(kids, ui.WrapItem(func(gtx C) D { return layout.Inset{Left: ui.Space(1), Right: ui.Space(1)}.Layout(gtx, label) }))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			d := fillIf(gtx, bg, ui.RadiusLG)
			if f.editor.sortable.Over() == zone {
				// Inside the box rather than the web's ring-2 outside it: the
				// zone is clipped to its own bounds, which hid all but the
				// corners of an outer ring.
				insetRing(gtx, gtx.Constraints.Min, gtx.Dp(ui.RadiusLG), gtx.Dp(2), th.Ring)
			}
			return d
		},
		func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(minHeight)
			return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
				return ui.Wrap{GapX: ui.Space(1.5), GapY: ui.Space(1.5), Justify: justify, ContentStart: true}.Layout(gtx, kids...)
			})
		},
	)
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
		if item.ID != id {
			continue
		}
		if item.ExampleKey != "" {
			return i18n.T(item.ExampleKey)
		}
		return item.Example
	}
	return id
}

// previewLayout is FooterLayoutPreview: the hint over a rounded-lg
// bg-sidebar strip, rounded like the zones above it where the web's is square,
// that reads each side's items with example values. The items wrap as the
// footer's own do, so the strip never scrolls.
func (f *footer) previewLayout(gtx C, th *ui.Theme, l footerLayout, in footerInput) D {
	on := th.On(th.Sidebar)
	strip := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return fillIf(gtx, th.Sidebar, ui.RadiusLG) },
			func(gtx C) D {
				gtx.Constraints.Min.Y = gtx.Dp(ui.Space(12))
				return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					return previewSides(gtx, on, l, in)
				})
			})
	}
	return stack(gtx, 2, []layout.Widget{th.Text(ui.TextXS, i18n.T("footer.footerEditor.previewHint")).Muted().Layout, strip})
}

// previewSides is the footer's wrap: the left side, the right side at the
// far end, or the empty line when neither holds an item.
func previewSides(gtx C, th *ui.Theme, l footerLayout, in footerInput) D {
	if len(l.Left)+len(l.Right) == 0 {
		return layout.W.Layout(gtx, th.Text(ui.TextXS, i18n.T("settings.footerLayoutEditor.previewEmpty")).Muted().Layout)
	}
	side := func(items []string, justify ui.Justify) layout.Widget {
		return func(gtx C) D {
			kids := make([]ui.WrapChild, len(items))
			for i, id := range items {
				kids[i] = ui.WrapItem(func(gtx C) D { return itemReading(gtx, th, id, th.MutedForeground, in) })
			}
			return ui.Wrap{GapX: ui.Space(3), GapY: ui.Space(1), Justify: justify}.Layout(gtx, kids...)
		}
	}
	return ui.Wrap{GapX: ui.Space(8), GapY: ui.Space(1), Justify: ui.JustifyBetween}.Layout(gtx,
		ui.WrapItem(side(l.Left, ui.JustifyStart)),
		ui.WrapMLAuto(side(l.Right, ui.JustifyEnd)),
	)
}
