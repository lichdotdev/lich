package main

import (
	"cmp"
	"gioui.org/io/semantic"
	"gioui.org/op/clip"
	"image"
	"slices"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The theme catalog's cards, from the approved mockup (temas-catalogo, frame
// 1). The grid and the card's surface are the extension catalog's.
var (
	themeFiltersBelow  = ui.Space(3)  // .filters margin-bottom 12px
	themeCardMetaAbove = ui.Space(2)  // .card .row margin-top 8px
	themeDotSize       = ui.Space(3)  // .dot 12px
	themeDotGap        = ui.Space(1)  // .dots gap 4px
	themeSkelName      = ui.Space(28) // the skeleton's bars
	themeSkelBy        = ui.Space(18)
	themeSkelMeta      = ui.Space(40)
	themeDotRing       = unit.Dp(1) // .dot box-shadow 0 0 0 1px
)

const (
	themeNameLeading = unit.Sp(20) // text-sm
	themeMetaLeading = unit.Sp(16) // text-xs
)

var (
	icThemeDark  = icons.Lucide("moon")
	icThemeLight = icons.Lucide("sun")
)

// The scheme filter's tabs.
const (
	themeTabAll = iota
	themeTabDark
	themeTabLight
)

// themeGrid is the catalog view's state: its filter and the controls that set
// it, each card's click, and the detail a card opens. shownQuery is the query
// the grid was last built from.
type themeGrid struct {
	scheme     ui.TabsState
	query      widget.Editor
	clearQuery ui.Clickable
	sortMenu   ui.MenuState
	sortButton ui.Clickable
	order      catalogSort
	shownQuery string
	retry      ui.Clickable
	cards      map[string]*ui.Clickable
	detail     themeDetail
}

func (g *themeGrid) card(id string) *ui.Clickable {
	if g.cards == nil {
		g.cards = map[string]*ui.Clickable{}
	}
	return clickableOf(g.cards, id)
}

// themeCatalogRows is the Catalog view: the filters over the grid, a skeleton
// while the catalog is read, or why it could not be.
func (s *settingsScreen) themeCatalogRows(th *ui.Theme, v appearanceView) []layout.Widget {
	p := s.theme
	if p.catalog == nil {
		return nil
	}
	cat, err := p.catalog.read()
	switch {
	case cat == nil && err != "":
		return []layout.Widget{s.themeCatalogFailed(th, err)}
	case cat == nil:
		return []layout.Widget{themeSkeletonRow(th)}
	}
	g := &p.grid
	g.shownQuery = g.query.Text()
	rows := []layout.Widget{s.themeFilters(th, len(cat.packs))}
	packs := themesShown(cat.packs, g.scheme.Selected, g.shownQuery, g.order)
	if len(packs) == 0 {
		return append(rows, func(gtx C) D {
			return layout.Inset{Left: rowPadX, Top: catalogBodyPad}.Layout(gtx, th.Text(ui.TextSM, i18n.T("settings.themesPage.noMatch")).Muted().Layout)
		})
	}
	installed := installedIDs(v)
	return append(rows, func(gtx C) D {
		return catalogCards(gtx, len(packs), func(gtx C, i int) D {
			return s.themeCard(gtx, th, packs[i], packState(packs[i], v.selected, installed))
		})
	})
}

// themesShown is the packs the scheme tab and the query leave, matched on
// name, publisher and description, in order.
func themesShown(packs []themePack, scheme int, query string, order catalogSort) []themePack {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []themePack
	for _, p := range packs {
		dark, light := p.schemes()
		if scheme == themeTabDark && !dark || scheme == themeTabLight && !light {
			continue
		}
		if q != "" && !packMatches(p, q) {
			continue
		}
		out = append(out, p)
	}
	slices.SortStableFunc(out, func(a, b themePack) int { return comparePacks(a, b, order) })
	return out
}

func packMatches(p themePack, query string) bool {
	for _, field := range []string{p.def.Name, p.def.Publisher, p.def.Description} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func comparePacks(a, b themePack, order catalogSort) int {
	byName := strings.Compare(strings.ToLower(a.def.Name), strings.ToLower(b.def.Name))
	switch order {
	case sortStars:
		return cmp.Or(cmp.Compare(b.def.Stars, a.def.Stars), byName)
	case sortName:
		return byName
	}
	return cmp.Or(strings.Compare(b.def.Updated, a.def.Updated), byName)
}

// The words a card says beside its name.
const (
	packListed = iota
	packInstalled
	packInUse
)

// packState is In use when the selected theme is the pack's, Installed when
// one of its themes is, nothing otherwise.
func packState(p themePack, selected string, installed map[string]bool) int {
	switch {
	case p.holds(selected):
		return packInUse
	case slices.ContainsFunc(p.themes, func(t themeLook) bool { return installed[t.def.ID] }):
		return packInstalled
	}
	return packListed
}

// themeFilters is the scheme tabs, the search and the order on one line. The
// grid was built from the filter before this lays out, so a change here asks
// for the frame that shows it.
func (s *settingsScreen) themeFilters(th *ui.Theme, total int) layout.Widget {
	return func(gtx C) D {
		g := &s.theme.grid
		tabs := th.Tabs(&g.scheme,
			ui.TabsTrigger{Text: i18n.T("settings.themesPage.all"), Trailing: th.Text(ui.TextXS, strconv.Itoa(total)).Mono().Muted().Layout},
			ui.TabsTrigger{Text: i18n.T("settings.themesPage.dark"), Icon: icThemeDark},
			ui.TabsTrigger{Text: i18n.T("settings.themesPage.light"), Icon: icThemeLight},
		)
		tabs.Variant = ui.TabsVariantGhost
		d := layout.Inset{Bottom: themeFiltersBelow}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					// No minimum: a ghost list given one shares it out.
					gtx.Constraints.Min.X = 0
					return tabs.Layout(gtx)
				}),
				ui.Gap(2),
				layout.Flexed(1, func(gtx C) D { return s.themeSearch(gtx, th) }),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D { return s.themeSort(gtx, th) }),
			)
		})
		if g.scheme.Changed() || g.query.Text() != g.shownQuery {
			gtx.Execute(op.InvalidateCmd{})
		}
		return d
	}
}

func (s *settingsScreen) themeSearch(gtx C, th *ui.Theme) D {
	g := &s.theme.grid
	field := th.InputGroup(&g.query, i18n.T("settings.themesPage.search"))
	field.Height, field.TextSize, field.Clear = catalogSearchH, ui.TextXS, &g.clearQuery
	field.Start = ui.InputGroupAddon{Items: []layout.Widget{field.Inner().InputGroupIcon(icCatalogSearch)}}
	return field.Layout(gtx)
}

// themeSort is the order's menu: Recently updated, Stars, Name.
func (s *settingsScreen) themeSort(gtx C, th *ui.Theme) D {
	g := &s.theme.grid
	orders := []catalogSort{sortRecent, sortStars, sortName}
	entries := make([]ui.MenuEntry, len(orders))
	for i, o := range orders {
		entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: o.label(), Checked: g.order == o, OnClick: func() {
			g.order = o
			s.invalidate()
		}}
	}
	return th.DropdownMenu(s.root, &g.sortMenu, entries).Layout(gtx, func(gtx C) D {
		b := th.Button(&g.sortButton, g.order.label())
		b.Variant, b.Size, b.Icon = ui.VariantOutline, ui.SizeSM, icCatalogSort
		return b.Layout(gtx)
	})
}

// themeCard is one pack: its cover drawn in its own colors, then its name, the
// publisher and the line of its variants. A click opens its detail.
func (s *settingsScreen) themeCard(gtx C, th *ui.Theme, pack themePack, state int) D {
	g := &s.theme.grid
	click := g.card(pack.def.ID)
	if click.Clicked(gtx) {
		g.detail.show(pack, s.theme.look.view().selected)
		// The dialogs were laid out before the card took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Card
		if click.Hovered() {
			bg = ui.MixOKLab(th.Card, th.Over(th.Accent), catalogHoverMix)
		}
		on := th.On(bg)
		return catalogCardSurface(gtx, bg, func(gtx C) D { return themeCover(gtx, on, pack.cover()) }, func(gtx C) D {
			return themeCardText(gtx, on, pack, state)
		})
	})
}

// themeCardText is the card under its cover; no text on it changes its height.
func themeCardText(gtx C, th *ui.Theme, pack themePack, state int) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return themeCardName(gtx, th, pack.def.Name, state) }),
		ui.VGap(0.5),
		layout.Rigid(th.Text(ui.TextXS, pack.def.Publisher).Mono().Muted().Leading(themeMetaLeading).Truncated().Layout),
		layout.Rigid(layout.Spacer{Height: themeCardMetaAbove}.Layout),
		layout.Rigid(func(gtx C) D { return themeCardMeta(gtx, th, pack) }),
	)
}

// themeCardName is the name, and a check at its right for In use (bright) or
// Installed (muted), its word for screen readers.
func themeCardName(gtx C, th *ui.Theme, name string, state int) D {
	line := []layout.FlexChild{layout.Flexed(1, th.Text(ui.TextSM, name).Weight(font.SemiBold).Leading(themeNameLeading).Truncated().Layout)}
	if state != packListed {
		words, ink := i18n.T("settings.themesPage.installed"), th.MutedForeground
		if state == packInUse {
			words, ink = i18n.T("settings.themesPage.inUse"), th.Foreground
		}
		line = append(line, ui.Gap(2),
			layout.Rigid(func(gtx C) D {
				d := icCatalogCheck.Layout(gtx, ui.Space(3), ink)
				defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
				semantic.LabelOp(words).Add(gtx.Ops)
				return d
			}),
		)
	}
	gtx.Constraints.Min.Y = gtx.Sp(themeNameLeading)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...)
}

// themeCardMeta is a dot per variant in its background, how many there are
// and of which schemes, and the stars at the right.
func themeCardMeta(gtx C, th *ui.Theme, pack themePack) D {
	dots := make([]layout.FlexChild, 0, 2*len(pack.themes))
	for i, t := range pack.themes {
		if i > 0 {
			dots = append(dots, layout.Rigid(layout.Spacer{Width: themeDotGap}.Layout))
		}
		dots = append(dots, layout.Rigid(func(gtx C) D { return variantDot(gtx, th, t) }))
	}
	stars := func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icCatalogStar.Layout(gtx, ui.Space(3), th.MutedForeground) }),
			ui.Gap(1),
			layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(pack.def.Stars)).Mono().Muted().Layout),
		)
	}
	gtx.Constraints.Min.Y = gtx.Sp(themeMetaLeading)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, dots...) }),
		ui.Gap(1.5),
		layout.Flexed(1, th.Text(ui.TextXS, variantsLine(pack)).Muted().Truncated().Layout),
		ui.Gap(2),
		layout.Rigid(stars),
	)
}

// variantDot is a disc in the variant's background, ringed in the border so a
// background like the card's still shows.
func variantDot(gtx C, th *ui.Theme, t themeLook) D {
	size := gtx.Dp(themeDotSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	ui.Fill(gtx, th.Over(th.Border), themeDotSize/2)
	ring := gtx.Dp(themeDotRing)
	defer op.Offset(image.Pt(ring, ring)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(size-2*ring, size-2*ring))
	ui.Fill(gtx, t.palette.Background, themeDotSize/2)
	return D{Size: image.Pt(size, size)}
}

// variantsLine is "3 variants · dark & light", in one sentence per scheme mix.
func variantsLine(pack themePack) string {
	args := i18n.Args{"count": len(pack.themes)}
	switch dark, light := pack.schemes(); {
	case dark && light:
		return i18n.T("settings.themesPage.variantsBoth", args)
	case light:
		return i18n.T("settings.themesPage.variantsLight", args)
	}
	return i18n.T("settings.themesPage.variantsDark", args)
}

// themeCatalogFailed says the catalog could not be read and that a repository
// URL still installs, over the backend's words, Try again on its right.
func (s *settingsScreen) themeCatalogFailed(th *ui.Theme, err string) layout.Widget {
	return func(gtx C) D {
		p := s.theme
		if p.grid.retry.Clicked(gtx) {
			p.catalog.retry()
		}
		b := th.Button(&p.grid.retry, i18n.T("common.action.tryAgain"))
		b.Variant, b.Size, b.Icon = ui.VariantGhost, ui.SizeSM, icCatalogRetry
		return layout.Inset{Left: rowPadX}.Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.themesPage.readFailed")).Muted().Wrapped().Layout),
						ui.VGap(0.5),
						layout.Rigid(th.Text(ui.TextXS, err).Mono().Muted().Wrapped().Layout),
					)
				}),
				ui.Gap(3),
				layout.Rigid(b.Layout),
			)
		})
	}
}

// themeSkeletonRow is a line of cards while the catalog is read, in the
// loaded card's measures.
func themeSkeletonRow(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		cols := catalogColumns(gtx, gtx.Constraints.Max.X)
		return catalogCards(gtx, cols, func(gtx C, _ int) D { return themeSkeletonCard(gtx, th) })
	}
}

func themeSkeletonCard(gtx C, th *ui.Theme) D {
	bg := th.Card
	on := th.On(bg)
	bar := func(width unit.Dp, line, size unit.Sp) layout.FlexChild {
		return layout.Rigid(on.SkeletonBar(width, unit.Dp(line), unit.Dp(size)).Layout)
	}
	cover := func(gtx C) D {
		w := gtx.Constraints.Max.X
		gtx.Constraints = layout.Exact(image.Pt(w, w*catalogCoverH/catalogCoverW))
		return ui.Fill(gtx, ui.MixOKLab(bg, th.Background, catalogCoverMix), 0)
	}
	return catalogCardSurface(gtx, bg, cover, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			bar(themeSkelName, themeNameLeading, ui.TextSM),
			ui.VGap(0.5),
			bar(themeSkelBy, themeMetaLeading, ui.TextXS),
			layout.Rigid(layout.Spacer{Height: themeCardMetaAbove}.Layout),
			bar(themeSkelMeta, themeMetaLeading, ui.TextXS),
		)
	})
}
