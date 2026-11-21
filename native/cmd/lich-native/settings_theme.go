package main

import (
	"context"
	"image"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The trigger's geometry.
var (
	themeTriggerHeight = ui.Space(10)
	themeTriggerPadL   = ui.Space(1.5)
	themeTriggerMini   = image.Pt(44, 28)
	icTriangleAlert    = icons.Lucide("triangle-alert")
	icThemesBrowse     = icons.Lucide("paintbrush")
)

// brokenRowState is one broken theme's row: its hover and its Remove.
type brokenRowState struct {
	hover  gesture.Hover
	remove ui.Clickable
}

func brokenRowOf(rows map[string]*brokenRowState, id string) *brokenRowState {
	if r, ok := rows[id]; ok {
		return r
	}
	r := new(brokenRowState)
	rows[id] = r
	return r
}

// themePicker is the themes in Settings: Interface's Theme row, a select of
// the installed ones, and the Themes page, the catalog and the installed
// themes, with the dialogs both open.
type themePicker struct {
	ctx     context.Context
	client  *backend.Client
	look    *appearance
	toaster *ui.Toaster
	root    *ui.Root
	catalog *themeCatalogStore

	trigger      ui.Clickable
	menu         ui.MenuState
	check        ui.Clickable
	follow       ui.SwitchState
	brokenRemove map[string]*brokenRowState
	dialogs      themeDialogs

	// view is the Themes page's tab, themesViewCatalog each time it opens.
	view        ui.TabsState
	installFrom ui.Clickable
	grid        themeGrid
	rows        map[string]*themeRowState
	bar         themeTrialBar

	mu    sync.Mutex
	tasks themeTasks
}

func newThemePicker(ctx context.Context, c *backend.Client, look *appearance, toaster *ui.Toaster, root *ui.Root) *themePicker {
	return &themePicker{ctx: ctx, client: c, look: look, toaster: toaster, root: root,
		brokenRemove: map[string]*brokenRowState{}, rows: map[string]*themeRowState{}}
}

// themeRow is Interface's Theme row: a select of System and every installed
// theme, and the way to the Themes page. A project's scope shows the row
// dimmed with no control: the theme is global.
func (s *settingsScreen) themeRow(gtx C, th *ui.Theme, r settingRow, first bool) D {
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	p := s.theme
	v := p.look.view()
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		menu := th.DropdownMenu(s.root, &p.menu, s.themeMenu(v))
		return menu.Layout(gtx, func(gtx C) D { return p.triggerButton(gtx, th, v) })
	})
}

// themeMenu is System and every installed theme, the selected one checked,
// then Browse themes.
func (s *settingsScreen) themeMenu(v appearanceView) []ui.MenuEntry {
	p := s.theme
	item := func(id, name string) ui.MenuEntry {
		e := ui.MenuEntry{Label: name, Inset: true, OnClick: func() { p.look.choose(id) }}
		if id == v.selected {
			e.Icon, e.Inset = icCheck, false
		}
		return e
	}
	entries := []ui.MenuEntry{item(themeSystem, i18n.T("settings.themePicker.system"))}
	for _, e := range v.themes {
		entries = append(entries, item(e.def.ID, e.def.Name))
	}
	return append(entries,
		ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: i18n.T("settings.themesPage.browse"), Icon: icThemesBrowse, OnClick: func() {
			s.openPage(sectionAppearance, themesPage)
			s.invalidate()
		}},
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
// name, and the chevron that turns while the menu is open.
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
				layout.Rigid(func(gtx C) D { return on.TriggerChevron(gtx, p.menu.Opened()) }),
			)
		})
		call := macro.Stop()
		box := gtx
		box.Constraints = layout.Exact(d.Size)
		ui.Fill(box, bg, ui.RadiusMD)
		ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusMD), th.Over(th.Palette.Input))
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

// brokenList is BrokenThemeList: a theme with no colors to preview is a row
// under the installed ones, its reason written out.
func (p *themePicker) brokenList(gtx C, th *ui.Theme, broken []backend.BrokenTheme) D {
	if len(broken) == 0 {
		return D{}
	}
	children := []layout.FlexChild{
		ui.VGap(4),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: rowPadX, Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icTriangleAlert.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
					ui.Gap(1.5),
					layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(i18n.T("settings.brokenThemeList.heading", i18n.Args{"count": len(broken)}))).Leading(text2XSLeading).Muted().Weight(font.Medium).Layout),
				)
			})
		}),
	}
	for _, b := range broken {
		row := brokenRowOf(p.brokenRemove, b.ID)
		children = append(children, layout.Rigid(func(gtx C) D { return p.brokenRow(gtx, th, b, row) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// brokenRow is one grid row: the id, the reason, and Remove, the row's
// hover fill on the list's own edge.
func (p *themePicker) brokenRow(gtx C, th *ui.Theme, b backend.BrokenTheme, row *brokenRowState) D {
	if row.remove.Clicked(gtx) {
		p.askRemove(b.ID, b.ID)
		// The dialogs were laid out before the row took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	bg := th.Surface
	if row.hover.Update(gtx.Source) {
		bg = th.Over(ui.Alpha(th.Accent, 0.5))
	}
	on := th.On(bg)
	macro := op.Record(gtx.Ops)
	d := layout.Inset{Left: rowPadX, Right: rowPadX, Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
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
				return on.GhostTextButton(gtx, &row.remove, i18n.T("settings.brokenThemeList.remove"), on.Destructive)
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
