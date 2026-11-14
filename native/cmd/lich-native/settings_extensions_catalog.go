package main

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The catalog's grid, from the approved mockup (settings-catalog, frame 1):
// the site's cards, as many to a line as keep each between catalogCardMin and
// catalogCardMax wide, two in the pane at the window's default size.
var (
	catalogCardGap   = ui.Space(3)   // .cards gap-3
	catalogCardMin   = ui.Space(65)  // a card's narrowest, before a line drops a column
	catalogCardMax   = ui.Space(90)  // its widest, past which a line takes a column more
	catalogGlyphSize = ui.Space(11)  // .cover .i, 44px
	catalogTextTop   = ui.Space(3.5) // .ctxt padding 14px 16px 16px
	catalogTextX     = ui.Space(4)
	catalogTextBelow = ui.Space(4)
	catalogMetaAbove = ui.Space(3.5) // .cmeta padding-top
	catalogStripGap  = ui.Space(2)   // .strip gap-2
	catalogSearchH   = ui.Space(8)   // the field's h-8
	catalogBodyPad   = ui.Space(1.5) // the error's and the no-match line's top
	catalogSkelName  = ui.Space(36)  // the skeleton's bars, frame 8
	catalogSkelBy    = ui.Space(22)
	catalogSkelMeta  = ui.Space(20)
)

const (
	catalogNameSize    = unit.Sp(15) // .ctxt h5
	catalogNameLeading = unit.Sp(22)
	catalogDescLeading = unit.Sp(19) // .ctxt p, at settingsDescSize
	catalogDescLines   = 2
	catalogMetaLeading = unit.Sp(16) // text-xs
	// catalogCoverW by catalogCoverH is the cover's 16:10, the listing's own.
	catalogCoverW, catalogCoverH = 16, 10
	// catalogCoverMix is the empty cover's ground, color-mix(card 55%, background).
	catalogCoverMix = 0.45
	// catalogHoverMix is a card under the pointer, color-mix(card 55%, accent).
	catalogHoverMix = 0.45
	// catalogSkelShort is the skeleton's second line of description, 60% wide.
	catalogSkelShort = 0.6
	catalogSkelLong  = 0.95
)

var (
	icCatalogSearch   = icons.Lucide("search")
	icCatalogSort     = icons.Lucide("arrow-down-wide-narrow")
	icCatalogOfficial = icons.Lucide("badge-check")
	icCatalogStar     = icons.Lucide("star")
	icCatalogCheck    = icons.Lucide("check")
	icCatalogRetry    = icons.Lucide("rotate-ccw")
)

// catalogGrid is the catalog group's state: its filter and the controls
// that set it, each card's click, and the detail a card opens.
type catalogGrid struct {
	filter     catalogFilter
	tabs       ui.TabsState
	query      widget.Editor
	clearQuery widget.Clickable
	sortMenu   ui.MenuState
	sortButton widget.Clickable
	retry      widget.Clickable
	cards      map[string]*widget.Clickable
	detail     catalogDetail
}

func (g *catalogGrid) card(id string) *widget.Clickable {
	if g.cards == nil {
		g.cards = map[string]*widget.Clickable{}
	}
	c, ok := g.cards[id]
	if !ok {
		c = new(widget.Clickable)
		g.cards[id] = c
	}
	return c
}

// catalogRows is the catalog group: its heading, then the filter strip over
// the grid, a skeleton while the catalog is read, or why it could not be.
// first is the group opening the pane, with nothing installed above it.
func (s *settingsScreen) catalogRows(th *ui.Theme, installed []backend.Extension, first bool) []layout.Widget {
	p := &s.extensions
	cat, err := p.catalog.read()
	title := i18n.T("settings.extensionsSettings.catalog.title")
	switch {
	case cat == nil && err != "":
		return []layout.Widget{s.catalogRetryHeading(th, groupHeading(th, title, first)), catalogReadFailed(th, err)}
	case cat == nil:
		return []layout.Widget{groupHeading(th, title, first), catalogSkeletonRow(th)}
	}
	counts := catalogCounts(cat.Catalog)
	g := &p.grid
	if g.tabs.Selected >= len(counts) {
		g.tabs.Selected = 0
	}
	g.filter.category, g.filter.query = counts[g.tabs.Selected].id, g.query.Text()
	// The heading carries no count: All, under it, already says how many.
	heading := groupHeading(th, title, first)
	rows := []layout.Widget{func(gtx C) D {
		errText := ""
		if g.detail.id == "" {
			errText = p.snapshot().errorAt(installFromCatalog)
		}
		return withErrorLine(gtx, th, errText, rowPadX, heading)
	}, s.catalogStrip(th, counts)}
	entries := g.filter.entries(cat.Catalog)
	if len(entries) == 0 {
		return append(rows, func(gtx C) D {
			return layout.Inset{Left: rowPadX, Top: catalogBodyPad}.Layout(gtx, th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.catalog.noMatch")).Muted().Layout)
		})
	}
	return append(rows, func(gtx C) D {
		return catalogCards(gtx, len(entries), func(gtx C, i int) D {
			_, has := installedAs(installed, entries[i].ID)
			return s.catalogCard(gtx, th, cat, entries[i], has)
		})
	})
}

// catalogColumns is how many cards a line width px wide holds: enough that
// none is wider than catalogCardMax, unless that would make one narrower than
// catalogCardMin; one at the least. A wide pane takes columns, never a bigger
// cover.
func catalogColumns(gtx C, width int) int {
	gap := gtx.Dp(catalogCardGap)
	enough := (width + gap + gtx.Dp(catalogCardMax)) / (gtx.Dp(catalogCardMax) + gap)
	room := (width + gap) / (gtx.Dp(catalogCardMin) + gap)
	return max(1, min(enough, room))
}

// catalogCards lays n cards in lines of as many as the width holds, each an
// equal share of its line, the grid's gap between and under them.
func catalogCards(gtx C, n int, card func(gtx C, i int) D) D {
	width, gap := gtx.Constraints.Max.X, gtx.Dp(catalogCardGap)
	cols := catalogColumns(gtx, width)
	w := (width - (cols-1)*gap) / cols
	y := 0
	for first := 0; first < n; first += cols {
		line := 0
		for i := first; i < min(first+cols, n); i++ {
			c := gtx
			c.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
			at := op.Offset(image.Pt((i-first)*(w+gap), y)).Push(gtx.Ops)
			line = max(line, card(c, i).Size.Y)
			at.Pop()
		}
		y += line + gap
	}
	return D{Size: image.Pt(width, y)}
}

// catalogRetryHeading is the group's heading with Try again on its right.
func (s *settingsScreen) catalogRetryHeading(th *ui.Theme, heading layout.Widget) layout.Widget {
	return func(gtx C) D {
		p := &s.extensions
		if p.grid.retry.Clicked(gtx) {
			p.catalog.retry()
		}
		b := th.Button(&p.grid.retry, i18n.T("common.action.tryAgain"))
		b.Variant, b.Size, b.Icon = ui.VariantGhost, ui.SizeSM, icCatalogRetry
		return layout.Flex{Alignment: layout.End}.Layout(gtx, layout.Flexed(1, heading), layout.Rigid(b.Layout))
	}
}

// catalogReadFailed says the catalog could not be read and that a GitHub
// URL still installs, over the backend's words.
func catalogReadFailed(th *ui.Theme, err string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: rowPadX, Right: rowPadX, Top: catalogBodyPad}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.catalog.readFailed")).Muted().Wrapped().Layout),
				ui.VGap(0.5),
				layout.Rigid(th.Text(ui.TextXS, err).Mono().Muted().Wrapped().Layout),
			)
		})
	}
}

// catalogStrip is the categories with their counts over the search and the
// order. The grid was built from the filter before this lays out, so a
// change here asks for the frame that shows it.
func (s *settingsScreen) catalogStrip(th *ui.Theme, counts []catalogCount) layout.Widget {
	return func(gtx C) D {
		g := &s.extensions.grid
		triggers := make([]ui.TabsTrigger, len(counts))
		for i, c := range counts {
			n := strconv.Itoa(c.n)
			triggers[i] = ui.TabsTrigger{Text: c.name, Trailing: th.Text(ui.TextXS, n).Mono().Muted().Layout}
		}
		tabs := th.Tabs(&g.tabs, triggers...)
		tabs.Variant = ui.TabsVariantGhost
		d := layout.Inset{Bottom: catalogStripGap}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					// No minimum: a ghost list given one shares it out, and the
					// categories keep their own widths as the site's do.
					gtx.Constraints.Min.X = 0
					return tabs.Layout(gtx)
				}),
				ui.VGap(2),
				layout.Rigid(func(gtx C) D { return s.catalogSearchLine(gtx, th) }),
			)
		})
		if g.tabs.Changed() || g.query.Text() != g.filter.query {
			gtx.Execute(op.InvalidateCmd{})
		}
		return d
	}
}

// catalogSearchLine is the search field holding the line, the order on its
// right.
func (s *settingsScreen) catalogSearchLine(gtx C, th *ui.Theme) D {
	g := &s.extensions.grid
	field := th.InputGroup(&g.query, i18n.T("settings.extensionsSettings.catalog.search"))
	field.Height, field.TextSize, field.Clear = catalogSearchH, ui.TextXS, &g.clearQuery
	field.Start = ui.InputGroupAddon{Items: []layout.Widget{field.Inner().InputGroupIcon(icCatalogSearch)}}
	orders := []catalogSort{sortRecent, sortStars, sortName}
	entries := make([]ui.MenuEntry, len(orders))
	for i, o := range orders {
		entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: o.label(), Checked: g.filter.sort == o, OnClick: func() {
			g.filter.sort = o
			s.invalidate()
		}}
	}
	sort := func(gtx C) D {
		return th.DropdownMenu(s.root, &g.sortMenu, entries).Layout(gtx, func(gtx C) D {
			b := th.Button(&g.sortButton, g.filter.sort.label())
			b.Variant, b.Size, b.Icon = ui.VariantOutline, ui.SizeSM, icCatalogSort
			return b.Layout(gtx)
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Flexed(1, field.Layout), ui.Gap(2), layout.Rigid(sort))
}

// catalogCard is one listing as the site draws it: the cover or the glyph,
// then the name, the publisher, two lines of description and the meta line.
// A click opens its detail.
func (s *settingsScreen) catalogCard(gtx C, th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry, installed bool) D {
	g := &s.extensions.grid
	click := g.card(e.ID)
	if click.Clicked(gtx) {
		g.detail.show(e.ID)
		// The dialogs were laid out before the card took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Card
		if click.Hovered() {
			bg = ui.MixOKLab(th.Card, th.Over(th.Accent), catalogHoverMix)
		}
		on := th.On(bg)
		return catalogCardSurface(gtx, bg, func(gtx C) D { return s.catalogCover(gtx, on, e) }, func(gtx C) D {
			return catalogCardText(gtx, on, cat, e, installed)
		})
	})
}

// catalogCardSurface is the card's rounded fill under its cover and text.
func catalogCardSurface(gtx C, bg color.NRGBA, cover, text layout.Widget) D {
	macro := op.Record(gtx.Ops)
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(cover),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: catalogTextTop, Left: catalogTextX, Right: catalogTextX, Bottom: catalogTextBelow}.Layout(gtx, text)
		}),
	)
	call := macro.Stop()
	defer clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bg)
	call.Add(gtx.Ops)
	return d
}

// catalogCover is the listing's cover, 16:10 across the card, or its glyph
// on an empty ground while there is no picture.
func (s *settingsScreen) catalogCover(gtx C, th *ui.Theme, e extension.CatalogEntry) D {
	w := gtx.Constraints.Max.X
	gtx.Constraints = layout.Exact(image.Pt(w, w*catalogCoverH/catalogCoverW))
	if img := s.extensions.catalog.image(e.Cover); img != nil {
		return ui.ImageStyle{Src: img, Fit: widget.Cover, Position: layout.NW}.Layout(gtx)
	}
	ui.Fill(gtx, ui.MixOKLab(th.Surface, th.Background, catalogCoverMix), 0)
	return ui.CenteredIcon(gtx, s.extensions.glyph(string(e.Glyph)), catalogGlyphSize, th.MutedForeground)
}

// catalogCardText is the card under its cover. The description always holds
// its two lines, so every card in the grid stands as tall as the next.
func catalogCardText(gtx C, th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry, installed bool) D {
	desc := th.Text(settingsDescSize, entryDescription(e)).Muted().Leading(catalogDescLeading)
	desc.MaxLines = catalogDescLines
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Text(catalogNameSize, entryName(e)).Weight(font.SemiBold).Leading(catalogNameLeading).Truncated().Layout),
		ui.VGap(0.5),
		layout.Rigid(func(gtx C) D { return publisherLine(gtx, th, e) }),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			d := desc.Layout(gtx)
			d.Size.Y = max(d.Size.Y, catalogDescLines*gtx.Sp(catalogDescLeading))
			return d
		}),
		layout.Rigid(layout.Spacer{Height: catalogMetaAbove}.Layout),
		layout.Rigid(func(gtx C) D { return catalogMeta(gtx, th, cat, e, installed) }),
	)
}

// publisherLine is the publisher in mono, with the Official mark for the one
// the catalog calls official.
func publisherLine(gtx C, th *ui.Theme, e extension.CatalogEntry) D {
	children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, e.Publisher).Mono().Muted().Truncated().Layout)}
	if e.Official {
		children = append(children, ui.Gap(2),
			layout.Rigid(func(gtx C) D { return icCatalogOfficial.Layout(gtx, ui.Space(3), th.MutedForeground) }),
			ui.Gap(1),
			layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.catalog.official")).Muted().Layout),
		)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// catalogMeta is the card's foot: Installed, Not for this system and the
// categories on the left, the stars on the right.
func catalogMeta(gtx C, th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry, installed bool) D {
	var left []layout.FlexChild
	add := func(w layout.Widget) {
		if len(left) > 0 {
			left = append(left, ui.Gap(3))
		}
		left = append(left, layout.Rigid(w))
	}
	if installed {
		add(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icCatalogCheck.Layout(gtx, ui.Space(3), th.TonePass) }),
				ui.Gap(1),
				layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.catalog.installed")).Muted().Layout),
			)
		})
	}
	if _, ok := e.Builds[cat.Platform]; !ok {
		add(th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.catalog.notFor", i18n.Args{"os": systemName(cat.Platform)})).Muted().Layout)
	}
	if names := entryCategories(cat.Catalog, e); names != "" {
		add(th.Text(ui.TextXS, names).Muted().Truncated().Layout)
	}
	stars := func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icCatalogStar.Layout(gtx, ui.Space(3), th.MutedForeground) }),
			ui.Gap(1),
			layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(e.Stars)).Mono().Muted().Layout),
		)
	}
	gtx.Constraints.Min.Y = gtx.Sp(catalogMetaLeading)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, left...) }),
		ui.Gap(3),
		layout.Rigid(stars),
	)
}

// catalogSkeletonRow is a line of cards while the catalog is read: the
// cover's ground and a bar where each line of text lands, in the loaded
// card's measures.
func catalogSkeletonRow(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		cols := catalogColumns(gtx, gtx.Constraints.Max.X)
		return catalogCards(gtx, cols, func(gtx C, _ int) D { return catalogSkeletonCard(gtx, th) })
	}
}

func catalogSkeletonCard(gtx C, th *ui.Theme) D {
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
		w := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			bar(catalogSkelName, catalogNameLeading, catalogNameSize),
			ui.VGap(0.5),
			bar(catalogSkelBy, catalogMetaLeading, ui.TextXS),
			ui.VGap(1.5),
			bar(w*catalogSkelLong, catalogDescLeading, settingsDescSize),
			bar(w*catalogSkelShort, catalogDescLeading, settingsDescSize),
			layout.Rigid(layout.Spacer{Height: catalogMetaAbove}.Layout),
			bar(catalogSkelMeta, catalogMetaLeading, ui.TextXS),
		)
	})
}
