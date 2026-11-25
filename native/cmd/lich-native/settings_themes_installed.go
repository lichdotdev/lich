package main

import (
	"gioui.org/io/semantic"
	"image"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The Installed view, from the approved mockup (temas-instalados, option A):
// the theme in use on a wide card at the top, every other theme a card of the
// catalog's grid under it.
var (
	themeHeroPad    = ui.Space(2.5) // the hero's padding
	themeHeroCoverW = ui.Space(80)  // its cover's width, 16:10
	themeHeroGap    = ui.Space(4)
)

// themeHeroName is the in-use theme's name on its card.
const themeHeroName = unit.Sp(20)

// themeRowState is one installed theme's card: its click, its Update, and the
// ⋯ with its menu.
type themeRowState struct {
	card, update, more ui.Clickable
	hover              gesture.Hover
	menu               ui.MenuState
	moreTip            ui.Tooltip
}

func (p *themePicker) row(id string) *themeRowState {
	if r, ok := p.rows[id]; ok {
		return r
	}
	r := new(themeRowState)
	p.rows[id] = r
	return r
}

// installedTheme is one theme the window has, as its card shows it.
type installedTheme struct {
	id, name, line string
	look           themeLook
	latest         string
	custom         bool
}

// installedThemeRows is In use, the theme drawn wide, then the rest as cards:
// the imported themes in the window's order, then the bundled ones, then the
// themes that can't load.
func (s *settingsScreen) installedThemeRows(th *ui.Theme, v appearanceView) []layout.Widget {
	all := s.installedThemes(v)
	var inUse installedTheme
	var rest []installedTheme
	for _, t := range all {
		if t.id == v.selected {
			inUse = t
			continue
		}
		rest = append(rest, t)
	}
	rows := []layout.Widget{
		groupHeading(th, i18n.T("settings.themesPage.inUseGroup"), true),
		func(gtx C) D { return s.themeHero(gtx, th, v, inUse) },
		groupHeading(th, i18n.T("settings.themesPage.otherGroup"), false),
		func(gtx C) D {
			return catalogCards(gtx, len(rest), func(gtx C, i int) D { return s.installedCard(gtx, th, rest[i]) })
		},
	}
	return append(rows, func(gtx C) D { return s.theme.brokenList(gtx, th, v.broken) })
}

// installedThemes is every theme the window has, System first among the
// bundled ones, each with the look its card draws.
func (s *settingsScreen) installedThemes(v appearanceView) []installedTheme {
	p := s.theme
	var cat *themeCatalog
	if p.catalog != nil {
		cat, _ = p.catalog.read()
	}
	var custom, bundled []installedTheme
	for _, e := range v.themes {
		t := installedTheme{id: e.def.ID, name: e.def.Name, line: i18n.T("settings.themePicker.bundled"), look: p.lookOf(e)}
		if e.def.Origin != themeOriginCustom {
			bundled = append(bundled, t)
			continue
		}
		t.custom, t.line = true, installedLine(e.def, cat)
		if e.def.Latest != nil {
			t.latest = e.def.Latest.Version
		}
		custom = append(custom, t)
	}
	system := installedTheme{id: themeSystem, name: i18n.T("settings.themePicker.system"), line: i18n.T("settings.themePicker.followsOs"), look: p.lookOf(v.resolved)}
	if v.selected != themeSystem {
		if _, dark := bundledPair(v.themes); dark.def.ID != "" {
			system.look = p.lookOf(dark)
		}
	}
	return append(append(custom, system), bundled...)
}

// lookOf is e as a cover draws it, kept while the theme stays the same.
func (p *themePicker) lookOf(e themeEntry) themeLook {
	key := e.def.ID + "\x00" + e.def.Name
	if e.def.Source != nil {
		key += "\x00" + e.def.Source.Version
	}
	if l, ok := p.looks[key]; ok {
		return l
	}
	l, err := readLook(e.def)
	if err != nil {
		l = themeLook{themeEntry: e}
	}
	if p.looks == nil {
		p.looks = map[string]themeLook{}
	}
	p.looks[key] = l
	return l
}

// installedLine is where an imported theme came from: the catalog's name of
// its pack and the version, else the repository's, "imported" for a single
// file, and the newer version when there is one.
func installedLine(def backend.Theme, cat *themeCatalog) string {
	line := i18n.T("settings.themesPage.imported")
	if def.Source != nil {
		line = themeCaption(def, "")
	}
	if pack, ok := cat.fromRepo(def.Source); ok {
		line = pack.def.Name + " " + def.Source.Version
	}
	if def.Latest != nil {
		line += " · " + i18n.T("settings.themePicker.updateAvailable", i18n.Args{"version": def.Latest.Version})
	}
	return line
}

// themeHero is the theme in use on an Accent card: its cover, then In use, the
// name, where it came from, its pair, and Auto light/dark at the foot; Update
// and the ⋯ at the top right for an imported one.
func (s *settingsScreen) themeHero(gtx C, th *ui.Theme, v appearanceView, t installedTheme) D {
	bg := th.Over(th.Accent)
	on := th.On(bg)
	macro := op.Record(gtx.Ops)
	d := layout.UniformInset(themeHeroPad).Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				w := gtx.Dp(themeHeroCoverW)
				gtx.Constraints = layout.Exact(image.Pt(w, w*catalogCoverH/catalogCoverW))
				return themeCover(gtx, on, t.look)
			}),
			layout.Rigid(layout.Spacer{Width: themeHeroGap}.Layout),
			layout.Flexed(1, func(gtx C) D {
				// The text stands as tall as the cover: Auto light/dark sits at its foot.
				h := gtx.Dp(themeHeroCoverW) * catalogCoverH / catalogCoverW
				gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
				return s.themeHeroText(gtx, on, v, t)
			}),
		)
	})
	call := macro.Stop()
	fill := gtx
	fill.Constraints = layout.Exact(d.Size)
	ui.Fill(fill, bg, ui.RadiusLG)
	call.Add(gtx.Ops)
	return d
}

func (s *settingsScreen) themeHeroText(gtx C, th *ui.Theme, v appearanceView, t installedTheme) D {
	p := s.theme
	head := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			d := icCatalogCheck.Layout(gtx, ui.Space(3.5), th.Foreground)
			defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
			semantic.LabelOp(i18n.T("settings.themesPage.inUse")).Add(gtx.Ops)
			return d
		}),
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
	}
	head = append(head, s.installedActionsOf(th, t)...)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, head...) }),
		ui.VGap(2),
		layout.Rigid(th.Text(themeHeroName, t.name).Weight(font.SemiBold).Truncated().Layout),
		ui.VGap(1),
		layout.Rigid(th.Text(ui.TextXS, t.line).Mono().Muted().Truncated().Layout),
	}
	if pair := selectedPair(v); pair != "" {
		if e, ok := themeByID(v.themes, pair); ok {
			children = append(children, ui.VGap(2),
				layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.themesPage.pairsWith", i18n.Args{"name": e.def.Name})).Muted().Layout))
		}
	}
	children = append(children,
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(0, gtx.Constraints.Min.Y)} }),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.settingsTheme.autoLightDark")).Muted().Layout),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D { return p.followSwitch(gtx, th, v) }),
			)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// installedCard is one theme the window has as the catalog draws a pack: its
// cover, then its name with Update or the ⋯, and where it came from. A click
// on it uses the theme.
func (s *settingsScreen) installedCard(gtx C, th *ui.Theme, t installedTheme) D {
	p := s.theme
	st := p.row(t.id)
	if st.card.Clicked(gtx) {
		p.look.choose(t.id)
		// The view laid out before the card took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	bg := th.Card
	if st.hover.Update(gtx.Source) || st.menu.Opened() {
		bg = ui.MixOKLab(th.Card, th.Over(th.Accent), catalogHoverMix)
	}
	on := th.On(bg)
	// The card's press sits under its Update and ⋯ rather than around them,
	// so a press on either is theirs alone.
	macro := op.Record(gtx.Ops)
	d := catalogCardSurface(gtx, bg, func(gtx C) D { return themeCover(gtx, on, t.look) }, func(gtx C) D {
		name := []layout.FlexChild{layout.Flexed(1, on.Text(ui.TextSM, t.name).Weight(font.SemiBold).Leading(themeNameLeading).Truncated().Layout)}
		name = append(name, s.installedActionsOf(on, t)...)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.Y = gtx.Dp(ui.Space(8))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, name...)
			}),
			layout.Rigid(on.Text(ui.TextXS, t.line).Mono().Muted().Leading(themeMetaLeading).Truncated().Layout),
		)
	})
	call := macro.Stop()
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	st.hover.Add(gtx.Ops)
	area.Pop()
	box := gtx
	box.Constraints = layout.Exact(d.Size)
	st.card.Layout(box, func(gtx C) D { return D{Size: gtx.Constraints.Min} })
	call.Add(gtx.Ops)
	return d
}

// installedActionsOf is Update while a newer version waits, then an imported
// theme's ⋯; a bundled theme has neither.
func (s *settingsScreen) installedActionsOf(th *ui.Theme, t installedTheme) []layout.FlexChild {
	p := s.theme
	st := p.row(t.id)
	var out []layout.FlexChild
	if t.latest != "" {
		out = append(out, ui.Gap(2), layout.Rigid(func(gtx C) D {
			busy := p.snapshot().updating
			if st.update.Clicked(gtx) && busy == "" {
				p.updateTheme(t.id)
			}
			b := th.Button(&st.update, i18n.T("settings.themesPage.update"))
			b.Size, b.Icon, b.Disabled = ui.SizeXS, icRefresh, busy != ""
			return b.Layout(gtx)
		}))
	}
	if t.custom {
		out = append(out, ui.Gap(1), layout.Rigid(func(gtx C) D { return s.themeCardMore(gtx, th, st, t) }))
	}
	return out
}

// themeCardMore is an imported theme's ⋯ and its menu, Remove, which asks
// first.
func (s *settingsScreen) themeCardMore(gtx C, th *ui.Theme, st *themeRowState, t installedTheme) D {
	p := s.theme
	entries := []ui.MenuEntry{{Label: i18n.T("settings.appearanceSettings.removeTheme"), Icon: icTrash, Variant: ui.VariantDestructive, OnClick: func() {
		p.askRemove(t.id, t.name)
		// The dialogs were laid out before the menu took the click.
		s.invalidate()
	}}}
	hint := i18n.T("settings.themesPage.rowOptions", i18n.Args{"name": t.name})
	return th.DropdownMenu(s.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &st.moreTip, hint).Layout(gtx, func(gtx C) D {
			b := th.Button(&st.more, "")
			b.Icon, b.Variant, b.Size = icExtensionMore, ui.VariantGhost, ui.SizeIconXS
			return b.Layout(gtx)
		})
	})
}
