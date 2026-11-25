package main

import (
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The theme's detail, from the approved mockup's .dlg (temas-catalogo, frame
// 2): the preview at its left, the variants, the actions and the facts in a
// column at its right.
var (
	themeDetailMaxW    = unit.Dp(980) // .dlg width 980px
	themeDetailMargin  = ui.Space(8)
	themeDetailTopPad  = layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(14), Left: ui.Space(4), Right: ui.Space(4)}
	themeDetailPad     = ui.Space(4)  // .dlg .cols padding 16px, gap 16px
	themeDetailSideW   = unit.Dp(250) // .side width 250px
	themeDetailSideGap = ui.Space(3)  // .side gap 12px
	themeDetailPreview = unit.Dp(470) // .bp height 470px
	themeDetailNameGap = unit.Dp(10)  // .top gap 10px
	icThemeTry         = icons.Lucide("eye")
	icThemeRepo        = icons.Lucide("external-link")
)

// themeVariantsCrowd is how many variants still fit the column with their
// glyphs; past it they drop them.
const themeVariantsCrowd = 2

const (
	themeDetailNameSize = unit.Sp(18) // .top .t 18px
	themeDetailFactSize = ui.TextXS
)

// themeDetail is the open pack's dialog: shown outlives id so the dialog keeps
// its words while it closes, and variant is the theme its preview shows.
type themeDetail struct {
	id, shown                      string
	variant                        ui.Enum
	state                          ui.DialogState
	close, install, use, try, repo ui.Clickable
	installTip                     ui.Tooltip
}

// show opens pack on the selected theme when it is the pack's, else on its
// cover's.
func (d *themeDetail) show(pack themePack, selected string) {
	d.id, d.shown = pack.def.ID, pack.def.ID
	d.variant.Value = pack.cover().def.ID
	if pack.holds(selected) {
		d.variant.Value = selected
	}
}

// layoutThemesDialogs draws the page's dialogs: the import dialog and the
// confirmations, and the detail of the card last clicked.
func (s *settingsScreen) layoutThemesDialogs(gtx C, th *ui.Theme) {
	p := s.theme
	p.layoutDialogs(gtx, th)
	d := &p.grid.detail
	if p.catalog == nil || (d.id == "" && !d.state.Animating(gtx)) {
		return
	}
	cat, _ := p.catalog.read()
	if cat == nil {
		return
	}
	pack, ok := cat.pack(d.shown)
	if !ok {
		d.id = ""
		return
	}
	look := pack.themes[0]
	for _, t := range pack.themes {
		if t.def.ID == d.variant.Value {
			look = t
		}
	}
	s.themeDetailClicks(gtx, pack, look)
	dlg := th.Dialog(&d.state, d.id != "", "", "")
	dlg.MaxWidth, dlg.Root = min(themeDetailMaxW, gtx.Metric.PxToDp(gtx.Constraints.Max.X)-2*themeDetailMargin), s.root
	dlg.Raw = func(gtx C) D { return s.themeDetailPanel(gtx, th.On(th.Popover), pack, look) }
	if dlg.Layout(gtx) {
		d.id = ""
	}
}

// themeDetailClicks acts on the detail's buttons. Try wears the variant on the
// window and closes the detail; Install and Use leave it open on the new state.
func (s *settingsScreen) themeDetailClicks(gtx C, pack themePack, look themeLook) {
	p := s.theme
	d := &p.grid.detail
	if d.close.Clicked(gtx) {
		d.id = ""
	}
	if d.repo.Clicked(gtx) {
		go func() { _ = s.client.OpenExternal(s.ctx, pack.def.Repo) }()
	}
	if d.install.Clicked(gtx) && s.themeRefusal(pack) == "" && !p.snapshot().importing {
		p.installCatalog(pack.def.ID, false, nil)
	}
	if d.use.Clicked(gtx) {
		p.look.choose(look.def.ID)
	}
	if d.try.Clicked(gtx) {
		p.bar.packID = pack.def.ID
		p.look.try(look.themeEntry)
		d.id = ""
	}
}

// themeRefusal is why lich will not install pack, "" when it will: a pack
// written for a newer lich than this one.
func (s *settingsScreen) themeRefusal(pack themePack) string {
	if need := pack.def.MinLichVersion; olderRelease(s.updates.version(), need) {
		return i18n.T("settings.themesPage.needsLich", i18n.Args{"version": strings.TrimPrefix(need, "v")})
	}
	return ""
}

// themeDetailPanel is the header over the preview and the column beside it,
// the preview no taller than the window leaves room for.
func (s *settingsScreen) themeDetailPanel(gtx C, th *ui.Theme, pack themePack, look themeLook) D {
	width := gtx.Constraints.Max.X
	gtx.Constraints.Min = image.Pt(width, 0)
	macro := op.Record(gtx.Ops)
	head := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.themeDetailHeader(th, pack)),
	)
	headCall := macro.Stop()
	pad, side := gtx.Dp(themeDetailPad), gtx.Dp(themeDetailSideW)
	room := gtx.Constraints.Max.Y - 2*gtx.Dp(themeDetailMargin) - head.Size.Y - 2*pad
	size := image.Pt(width-side-3*pad, max(0, min(gtx.Dp(themeDetailPreview), room)))
	headCall.Add(gtx.Ops)
	defer op.Offset(image.Pt(pad, head.Size.Y+pad)).Push(gtx.Ops).Pop()
	themePreview(gtx, th, look, size)
	at := op.Offset(image.Pt(size.X+pad, 0)).Push(gtx.Ops)
	cgtx := gtx
	cgtx.Constraints = layout.Exact(image.Pt(side, size.Y))
	s.themeDetailSide(cgtx, th, pack, look)
	at.Pop()
	return D{Size: image.Pt(width, head.Size.Y+size.Y+2*pad)}
}

// themeDetailHeader is the pack's name, its publisher and its stars, the × at
// the right.
func (s *settingsScreen) themeDetailHeader(th *ui.Theme, pack themePack) layout.Widget {
	return func(gtx C) D {
		d := &s.theme.grid.detail
		closeB := th.Button(&d.close, "")
		closeB.Icon, closeB.Variant, closeB.Size = icDetailClose, ui.VariantGhost, ui.SizeIconXS
		return themeDetailTopPad.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(themeDetailNameSize, pack.def.Name).Weight(font.SemiBold).Layout),
				layout.Rigid(layout.Spacer{Width: themeDetailNameGap}.Layout),
				layout.Rigid(th.Text(ui.TextXS, pack.def.Publisher).Mono().Muted().Layout),
				ui.Gap(3),
				layout.Rigid(func(gtx C) D { return icCatalogStar.Layout(gtx, ui.Space(3), th.MutedForeground) }),
				ui.Gap(1),
				layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(pack.def.Stars)).Mono().Muted().Layout),
				layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
				layout.Rigid(closeB.Layout),
			)
		})
	}
}

// themeDetailSide is the variants, the description, the actions, the facts and
// the repository.
func (s *settingsScreen) themeDetailSide(gtx C, th *ui.Theme, pack themePack, look themeLook) D {
	d := &s.theme.grid.detail
	gap := layout.Rigid(layout.Spacer{Height: themeDetailSideGap}.Layout)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return themeVariants(gtx, th, &d.variant, pack) }),
		gap,
	}
	if pack.def.Description != "" {
		children = append(children, layout.Rigid(th.Text(settingsDescSize, pack.def.Description).Muted().Leading(catalogDescLeading).Wrapped().Layout), gap)
	}
	children = append(children,
		layout.Rigid(func(gtx C) D { return s.themeDetailActions(gtx, th, pack, look) }),
		gap,
	)
	for _, f := range themeFacts(pack) {
		children = append(children, layout.Rigid(func(gtx C) D { return themeFact(gtx, th, f[0], f[1]) }), ui.VGap(2))
	}
	children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
		return th.TrailingIconLink(gtx, &d.repo, i18n.T("settings.themesPage.repository"), icThemeRepo)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// themeVariants is the segmented control of the pack's themes, each with the
// glyph of its scheme while they are few, on two tracks when one is too wide
// for the column.
func themeVariants(gtx C, th *ui.Theme, variant *ui.Enum, pack themePack) D {
	items := make([]ui.ToggleItem, len(pack.themes))
	for i, t := range pack.themes {
		items[i] = ui.ToggleItem{Value: t.def.ID, Text: pack.variantName(t)}
		if len(pack.themes) <= themeVariantsCrowd {
			items[i].Icon = icThemeDark
			if t.light() {
				items[i].Icon = icThemeLight
			}
		}
	}
	if variant.Update(gtx) {
		gtx.Execute(op.InvalidateCmd{})
	}
	// Measured at its own width: the clicks were taken above, so the measure
	// that is thrown away takes none.
	macro := op.Record(gtx.Ops)
	wide := gtx
	wide.Constraints.Max.X = math.MaxInt32
	d := th.ToggleGroup(variant, items...).Layout(wide)
	call := macro.Stop()
	if d.Size.X <= gtx.Constraints.Max.X {
		call.Add(gtx.Ops)
		return d
	}
	half := (len(items) + 1) / 2
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.ToggleGroup(variant, items[:half]...).Layout),
		ui.VGap(1),
		layout.Rigid(th.ToggleGroup(variant, items[half:]...).Layout),
	)
}

// themeDetailActions is In use, Use or Install for the variant shown, and Try
// while it is not the theme in use.
func (s *settingsScreen) themeDetailActions(gtx C, th *ui.Theme, pack themePack, look themeLook) D {
	p := s.theme
	d := &p.grid.detail
	v := p.look.view()
	if look.def.ID == v.selected {
		return labeledGlyph(gtx, i18n.T("settings.themesPage.inUse"), func(gtx C) D { return icCatalogCheck.Layout(gtx, ui.Space(3.5), th.MutedForeground) })
	}
	var main layout.Widget
	if _, ok := themeByID(v.themes, look.def.ID); ok {
		b := th.Button(&d.use, i18n.T("settings.themesPage.use"))
		b.Size = ui.SizeSM
		main = b.Layout
	} else {
		main = s.themeInstallButton(th, pack)
	}
	try := th.Button(&d.try, i18n.T("settings.themesPage.try"))
	try.Variant, try.Size, try.Icon = ui.VariantOutline, ui.SizeSM, icThemeTry
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(main), ui.Gap(2), layout.Rigid(try.Layout))
}

// themeInstallButton is Install, busy while an install runs, disabled with
// the reason in its tooltip for a pack lich refuses.
func (s *settingsScreen) themeInstallButton(th *ui.Theme, pack themePack) layout.Widget {
	p := s.theme
	d := &p.grid.detail
	b := th.Button(&d.install, i18n.T("settings.importThemeDialog.install"))
	b.Size = ui.SizeSM
	if p.snapshot().importing {
		b.Text, b.Disabled = i18n.T("settings.importThemeDialog.installing"), true
	}
	why := s.themeRefusal(pack)
	if why == "" {
		return b.Layout
	}
	b.Disabled = true
	return func(gtx C) D { return th.Tooltip(s.root, &d.installTip, why).Layout(gtx, b.Layout) }
}

// themeFacts are the pack's facts as label and value: its version, the themes
// that pair, its license and when it was last updated.
func themeFacts(pack themePack) [][2]string {
	facts := [][2]string{{i18n.T("settings.themesPage.version"), pack.def.Version}}
	if pairs := packPairs(pack); pairs != "" {
		facts = append(facts, [2]string{i18n.T("settings.themesPage.pairs"), pairs})
	}
	if pack.def.License != "" {
		facts = append(facts, [2]string{i18n.T("settings.themesPage.license"), pack.def.License})
	}
	return append(facts, [2]string{i18n.T("settings.themesPage.updated"), releasedDate(pack.def.Updated)})
}

// packPairs names the themes that are each other's pair, "Dawn ↔ Rosé Pine".
func packPairs(pack themePack) string {
	var pairs []string
	for i, a := range pack.themes {
		for _, b := range pack.themes[i+1:] {
			if a.def.Pair == b.def.ID && b.def.Pair == a.def.ID {
				pairs = append(pairs, pack.variantName(a)+" ↔ "+pack.variantName(b))
			}
		}
	}
	return strings.Join(pairs, ", ")
}

// themeFact is one fact, its label muted at the left and its value at the
// right.
func themeFact(gtx C, th *ui.Theme, label, value string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(themeDetailFactSize, label).Muted().Layout),
		ui.Gap(3),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, th.Text(themeDetailFactSize, value).Weight(font.Medium).Truncated().Layout)
		}),
	)
}
