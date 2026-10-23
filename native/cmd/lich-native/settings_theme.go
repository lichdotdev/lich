package main

import (
	"context"
	"image"
	"math"
	"strings"
	"sync"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The picker's geometry.
var (
	themeTriggerHeight = ui.Space(10)
	themeTriggerPadL   = ui.Space(1.5)
	themeTriggerMini   = image.Pt(44, 28)
	themeCardWidth     = unit.Dp(152)
	themeCardPreview   = ui.Space(22)
	themeCardPad       = ui.Space(1)
	themeCardGap       = ui.Space(3)
	themeCaptionSize   = unit.Sp(11)
	icTriangleAlert    = icons.Lucide("triangle-alert")
)

// brokenRowState is one broken theme's row: its hover and its Remove.
type brokenRowState struct {
	hover  gesture.Hover
	remove widget.Clickable
}

func brokenRowOf(rows map[string]*brokenRowState, id string) *brokenRowState {
	if r, ok := rows[id]; ok {
		return r
	}
	r := new(brokenRowState)
	rows[id] = r
	return r
}

// themeStripRows is the strip's grid-rows-2: a theme more costs width,
// which scrolls, rather than height.
const themeStripRows = 2

// themePicker is Settings › Appearance › Theme: the row's trigger, the strip
// of cards it opens, the broken themes under it, and the dialogs the cards
// and the import card open.
type themePicker struct {
	ctx     context.Context
	client  *backend.Client
	look    *appearance
	toaster *ui.Toaster
	root    *ui.Root

	open         bool
	trigger      widget.Clickable
	strip        widget.List
	importCard   themeCard
	systemCard   themeCard
	follow       ui.SwitchState
	cards        map[string]*themeCard
	brokenRemove map[string]*brokenRowState
	dialogs      themeDialogs

	mu    sync.Mutex
	tasks themeTasks
}

func newThemePicker(ctx context.Context, c *backend.Client, look *appearance, toaster *ui.Toaster, root *ui.Root) *themePicker {
	p := &themePicker{ctx: ctx, client: c, look: look, toaster: toaster, root: root,
		cards: map[string]*themeCard{}, brokenRemove: map[string]*brokenRowState{}}
	p.strip.Axis = layout.Horizontal
	return p
}

// layoutRow is the Theme row, and the strip under it while it is open. A
// project's scope shows the row dimmed with no control: the theme is
// global.
func (p *themePicker) layoutRow(gtx C, th *ui.Theme, r settingRow, first bool) D {
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	if p.trigger.Clicked(gtx) {
		p.open = !p.open
	}
	v := p.look.view()
	if !p.open {
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return p.triggerButton(gtx, th, v) })
	}
	row := func(gtx C) D {
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.settingsTheme.autoLightDark")).Muted().Layout),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D { return p.followSwitch(gtx, th, v) }),
				ui.Gap(3),
				layout.Rigid(func(gtx C) D { return p.triggerButton(gtx, th, v) }),
			)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(row),
		layout.Rigid(func(gtx C) D { return p.stripLayout(gtx, th, v) }),
	)
}

// followSwitch turns following the OS to the selected theme's pair on and
// off. A selection with no pair, System included, has nothing to follow to.
func (p *themePicker) followSwitch(gtx C, th *ui.Theme, v appearanceView) D {
	if p.follow.Update(gtx) {
		p.look.setFollowSystem(p.follow.Value)
	}
	p.follow.Value = v.followSystem
	sw := th.Switch(&p.follow)
	sw.Disabled = selectedPair(v) == ""
	return sw.Layout(gtx)
}

// selectedPair is the id of the selected theme's pair, "" for none.
func selectedPair(v appearanceView) string {
	e, ok := themeByID(v.themes, v.selected)
	if !ok {
		return ""
	}
	pair, ok := pairOf(e, v.themes)
	if !ok {
		return ""
	}
	return pair.def.ID
}

// triggerButton is the outline h-10 button: the selection's miniature, its
// name, and the chevron that turns while the strip is open.
func (p *themePicker) triggerButton(gtx C, th *ui.Theme, v appearanceView) D {
	return p.trigger.Layout(gtx, func(gtx C) D {
		bg := th.Over(ui.Alpha(th.Palette.Input, 0.3))
		if p.trigger.Hovered() {
			bg = th.Over(ui.Alpha(th.Palette.Input, 0.5))
		}
		on := th.On(bg)
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Pt(0, gtx.Dp(themeTriggerHeight))
		d := layout.Inset{Left: themeTriggerPadL, Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return p.selectionMiniature(gtx, on, v, themeTriggerMini) }),
				ui.Gap(2.5),
				layout.Rigid(on.Text(ui.TextSM, selectionName(v)).Weight(font.Medium).Layout),
				ui.Gap(3),
				layout.Rigid(func(gtx C) D { return triggerChevron(gtx, on, p.open) }),
			)
		})
		call := macro.Stop()
		box := gtx
		box.Constraints = layout.Exact(d.Size)
		ui.Fill(box, bg, ui.RadiusMD)
		insetBorder(gtx, d.Size, gtx.Dp(ui.RadiusMD), th.Over(th.Palette.Input))
		call.Add(gtx.Ops)
		return d
	})
}

func selectionName(v appearanceView) string {
	if v.selected == themeSystem {
		return i18n.T("settings.themePicker.system")
	}
	return v.resolved.def.Name
}

func (p *themePicker) selectionMiniature(gtx C, th *ui.Theme, v appearanceView, size image.Point) D {
	px := image.Pt(gtx.Dp(unit.Dp(size.X)), gtx.Dp(unit.Dp(size.Y)))
	if v.selected == themeSystem {
		light, dark := bundledPair(v.themes)
		return systemMiniature(gtx, th, light.palette, dark.palette, px)
	}
	return themeMiniature(gtx, th, v.resolved.palette, px, true)
}

// triggerChevron is the size-4 chevron-down, turned half a circle while
// open.
func triggerChevron(gtx C, th *ui.Theme, open bool) D {
	size := gtx.Dp(ui.Space(4))
	if open {
		c := f32.Pt(float32(size)/2, float32(size)/2)
		defer op.Affine(f32.AffineId().Rotate(c, math.Pi)).Push(gtx.Ops).Pop()
	}
	return icChevronDown.Layout(gtx, ui.Space(4), th.Foreground)
}

// stripLayout is the count line, the two-row grid of cards and the broken
// themes.
func (p *themePicker) stripLayout(gtx C, th *ui.Theme, v appearanceView) D {
	cards := p.cardWidgets(th, v)
	columns := (len(cards) + themeStripRows - 1) / themeStripRows
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, themeCount(len(v.themes))).Muted().Layout),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return th.ScrollArea(&p.strip).Layout(gtx, columns, func(gtx C, col int) D {
				return p.stripColumn(gtx, cards, col, columns)
			})
		}),
		layout.Rigid(func(gtx C) D { return p.brokenList(gtx, th, v.broken) }),
	)
}

// stripColumn is one column of the grid-flow-col grid: cards fill a column
// top to bottom before the next one starts, gap-3 between them all, and
// px-0.5 pb-3 around, which keeps the ring and the scrollbar apart.
func (p *themePicker) stripColumn(gtx C, cards []layout.Widget, col, columns int) D {
	inset := layout.Inset{Bottom: ui.Space(3), Left: themeCardGap}
	if col == 0 {
		inset.Left = ui.Space(0.5)
	}
	if col == columns-1 {
		inset.Right = ui.Space(0.5)
	}
	return inset.Layout(gtx, func(gtx C) D {
		var children []layout.FlexChild
		for row := range themeStripRows {
			i := col*themeStripRows + row
			if i >= len(cards) {
				break
			}
			if row > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: themeCardGap}.Layout))
			}
			children = append(children, layout.Rigid(cards[i]))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// cardWidgets are the strip's cards in order: import, System, every theme.
func (p *themePicker) cardWidgets(th *ui.Theme, v appearanceView) []layout.Widget {
	busy := p.snapshot()
	cards := []layout.Widget{
		func(gtx C) D { return p.importCardLayout(gtx, th) },
		func(gtx C) D {
			light, dark := bundledPair(v.themes)
			preview := func(gtx C, th *ui.Theme, size image.Point) D {
				return systemMiniature(gtx, th, light.palette, dark.palette, size)
			}
			return p.cardLayout(gtx, th, &p.systemCard, themeCardSpec{
				id: themeSystem, name: i18n.T("settings.themePicker.system"), caption: i18n.T("settings.themePicker.followsOs"), selected: v.selected == themeSystem, preview: preview})
		},
	}
	pair := selectedPair(v)
	for _, e := range v.themes {
		caption := themeCaption(e.def, v.selected)
		if e.def.ID == pair {
			caption = pairCaption(e.def.Scheme)
		}
		spec := themeCardSpec{id: e.def.ID, name: e.def.Name, caption: caption,
			selected: v.selected == e.def.ID, custom: e.def.Origin == themeOriginCustom, fromRepo: e.def.Source != nil,
			updating: busy.updating, preview: func(gtx C, th *ui.Theme, size image.Point) D {
				return themeMiniature(gtx, th, e.palette, size, true)
			}}
		card := themeCardOf(p.cards, e.def.ID)
		cards = append(cards, func(gtx C) D { return p.cardLayout(gtx, th, card, spec) })
	}
	return cards
}

// pairCaption is the card line of the selected theme's pair: the scheme it
// is the other half of.
func pairCaption(scheme string) string {
	if scheme == themeSchemeLight {
		return i18n.T("settings.settingsTheme.pairLight")
	}
	return i18n.T("settings.settingsTheme.pairDark")
}

// brokenList is BrokenThemeList: a theme with no colors to preview is a row
// under the strip, its reason written out.
func (p *themePicker) brokenList(gtx C, th *ui.Theme, broken []backend.BrokenTheme) D {
	if len(broken) == 0 {
		return D{}
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return ruleAbove(gtx, th) }),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icTriangleAlert.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
					ui.Gap(1.5),
					layout.Rigid(th.Text(text2XS, strings.ToUpper(i18n.T("settings.brokenThemeList.heading", i18n.Args{"count": len(broken)}))).Leading(text2XSLeading).Muted().Weight(font.Medium).Layout),
				)
			})
		}),
	}
	for _, b := range broken {
		row := brokenRowOf(p.brokenRemove, b.ID)
		children = append(children, layout.Rigid(func(gtx C) D { return p.brokenRow(gtx, th, b, row) }))
	}
	children = append(children, ui.VGap(3))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// brokenRow is one grid row: the id, the reason, and Remove, the row's
// hover fill reaching px-2 past the list on both sides (-mx-2 px-2).
func (p *themePicker) brokenRow(gtx C, th *ui.Theme, b backend.BrokenTheme, row *brokenRowState) D {
	if row.remove.Clicked(gtx) {
		p.askRemove(b.ID, b.ID)
	}
	pad := gtx.Dp(ui.Space(2))
	defer op.Offset(image.Pt(-pad, 0)).Push(gtx.Ops).Pop()
	gtx.Constraints.Max.X += 2 * pad
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	bg := th.Surface
	if row.hover.Update(gtx.Source) {
		bg = th.Over(ui.Alpha(th.Accent, 0.5))
	}
	on := th.On(bg)
	macro := op.Record(gtx.Ops)
	d := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		reason := on.Text(ui.TextXS, b.Reason).Muted()
		reason.MaxLines = 0
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(ui.Space(24))
				return on.Text(ui.TextXS, b.ID).Mono().Layout(gtx)
			}),
			ui.Gap(4),
			layout.Flexed(1, reason.Layout),
			ui.Gap(4),
			layout.Rigid(func(gtx C) D {
				return ghostTextButton(gtx, on, &row.remove, i18n.T("settings.brokenThemeList.remove"), on.Destructive)
			}),
		)
	})
	call := macro.Stop()
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	row.hover.Add(gtx.Ops)
	area.Pop()
	if bg != th.Surface {
		box := gtx
		box.Constraints = layout.Exact(d.Size)
		ui.Fill(box, bg, ui.RadiusMD)
	}
	call.Add(gtx.Ops)
	return d
}
