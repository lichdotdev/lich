package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The installed themes' rows, from the approved mockup's .irow
// (temas-catalogo, frame 4).
var (
	themeRowPadY = ui.Space(2.5) // .irow padding 10px 8px
	themeRowMini = image.Pt(64, 40)
)

// themeRowState is one installed row's controls and its menu.
type themeRowState struct {
	use, update, more ui.Clickable
	menu              ui.MenuState
	moreTip           ui.Tooltip
}

func (p *themePicker) row(id string) *themeRowState {
	if r, ok := p.rows[id]; ok {
		return r
	}
	r := new(themeRowState)
	p.rows[id] = r
	return r
}

// themeRowSpec is what one row shows: the miniature, the name and the line
// under it, and for an imported theme its newer version and its menu.
type themeRowSpec struct {
	id, name, line string
	mini           func(gtx C, th *ui.Theme, size image.Point) D
	latest         string
	custom         bool
}

// installedThemeRows is Bundled, then Installed, then the themes that can't
// load.
func (s *settingsScreen) installedThemeRows(th *ui.Theme, v appearanceView) []layout.Widget {
	p := s.theme
	light, dark := bundledPair(v.themes)
	rows := []layout.Widget{
		groupHeading(th, i18n.T("settings.themesPage.bundledGroup"), true),
		s.themeListRow(th, v, themeRowSpec{id: themeSystem, name: i18n.T("settings.themePicker.system"), line: i18n.T("settings.themePicker.followsOs"),
			mini: func(gtx C, th *ui.Theme, size image.Point) D {
				return systemMiniature(gtx, th, light.palette, dark.palette, size)
			}}),
	}
	var custom []layout.Widget
	var cat *themeCatalog
	if p.catalog != nil {
		cat, _ = p.catalog.read()
	}
	for _, e := range v.themes {
		spec := themeRowSpec{id: e.def.ID, name: e.def.Name, line: themeCaption(e.def, ""),
			mini: func(gtx C, th *ui.Theme, size image.Point) D { return themeMiniature(gtx, th, e.palette, size, true) }}
		if e.def.Origin != themeOriginCustom {
			rows = append(rows, s.themeListRow(th, v, spec))
			continue
		}
		spec.custom, spec.line = true, installedLine(e.def, cat)
		if e.def.Latest != nil {
			spec.latest = e.def.Latest.Version
		}
		custom = append(custom, s.themeListRow(th, v, spec))
	}
	if len(custom) > 0 {
		rows = append(rows, groupHeading(th, i18n.T("settings.themesPage.installedTab"), false))
		rows = append(rows, custom...)
	}
	return append(rows, func(gtx C) D { return p.brokenList(gtx, th, v.broken) })
}

// installedLine is where an imported theme came from: the catalog's name of
// its pack and the version, else the repository's, and the newer version when
// there is one.
func installedLine(def backend.Theme, cat *themeCatalog) string {
	line := themeCaption(def, "")
	if pack, ok := cat.fromRepo(def.Source); ok {
		line = pack.def.Name + " " + def.Source.Version
	}
	if def.Latest != nil {
		line += " · " + i18n.T("settings.themePicker.updateAvailable", i18n.Args{"version": def.Latest.Version})
	}
	return line
}

// themeListRow is one theme's row on its seam: the miniature, the name over
// the line, then Update while a newer version waits, In use or Use, and an
// imported theme's ⋯. A bundled row keeps the ⋯'s room so the rows' buttons
// line up.
func (s *settingsScreen) themeListRow(th *ui.Theme, v appearanceView, r themeRowSpec) layout.Widget {
	return func(gtx C) D {
		p := s.theme
		st := p.row(r.id)
		busy := p.snapshot().updating
		if st.use.Clicked(gtx) {
			p.look.choose(r.id)
		}
		if st.update.Clicked(gtx) && busy == "" {
			p.updateTheme(r.id)
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		line := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return r.mini(gtx, th, image.Pt(gtx.Dp(unit.Dp(themeRowMini.X)), gtx.Dp(unit.Dp(themeRowMini.Y))))
			}),
			ui.Gap(3),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextSM, r.name).Truncated().Layout),
					layout.Rigid(th.Text(ui.TextXS, r.line).Mono().Muted().Truncated().Layout),
				)
			}),
		}
		if r.latest != "" {
			b := th.Button(&st.update, i18n.T("settings.themesPage.update"))
			b.Variant, b.Size, b.Icon, b.Disabled = ui.VariantOutline, ui.SizeSM, icRefresh, busy != ""
			line = append(line, ui.Gap(2), layout.Rigid(b.Layout))
		}
		line = append(line, ui.Gap(2), layout.Rigid(func(gtx C) D { return themeUse(gtx, th, st, r.id == v.selected) }),
			ui.Gap(1), layout.Rigid(func(gtx C) D { return s.themeRowMore(gtx, th, st, r) }))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: rowPadX, Right: rowPadX, Top: themeRowPadY, Bottom: themeRowPadY}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...)
				})
			}),
			layout.Rigid(th.Separator),
		)
	}
}

// themeUse is In use with its check on the selected theme, Use on the rest.
func themeUse(gtx C, th *ui.Theme, st *themeRowState, inUse bool) D {
	if !inUse {
		b := th.Button(&st.use, i18n.T("settings.themesPage.use"))
		b.Variant, b.Size = ui.VariantGhost, ui.SizeSM
		return b.Layout(gtx)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return icCatalogCheck.Layout(gtx, ui.Space(3.5), th.Foreground) }),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.themesPage.inUse")).Layout),
	)
}

// themeRowMore is an imported theme's ⋯ and its menu, Remove, which asks
// first; a bundled theme's row keeps its room empty.
func (s *settingsScreen) themeRowMore(gtx C, th *ui.Theme, st *themeRowState, r themeRowSpec) D {
	side := gtx.Dp(ui.Space(8))
	if !r.custom {
		return D{Size: image.Pt(side, side)}
	}
	p := s.theme
	entries := []ui.MenuEntry{{Label: i18n.T("settings.appearanceSettings.removeTheme"), Icon: icTrash, Variant: ui.VariantDestructive, OnClick: func() {
		p.askRemove(r.id, r.name)
		// The dialogs were laid out before the menu took the click.
		s.invalidate()
	}}}
	hint := i18n.T("settings.themesPage.rowOptions", i18n.Args{"name": r.name})
	return th.DropdownMenu(s.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &st.moreTip, hint).Layout(gtx, func(gtx C) D {
			b := th.Button(&st.more, "")
			b.Icon, b.Variant, b.Size = icExtensionMore, ui.VariantGhost, ui.SizeIconSM
			return b.Layout(gtx)
		})
	})
}
