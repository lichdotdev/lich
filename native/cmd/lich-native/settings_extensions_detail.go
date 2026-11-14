package main

import (
	"image"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The detail, the site's dialog in the window's (settings-catalog, frame 3).
var (
	detailPad        = ui.Space(6) // .xd .sb padding 24px 24px 8px
	detailBodyBelow  = ui.Space(2)
	detailMargin     = ui.Space(16)  // max-height calc(100% - 64px)
	detailIconSize   = ui.Space(14)  // .xd-icon 56px
	detailIconRadius = unit.Dp(10)   // .xd-icon rounded 10px
	detailIconGlyph  = ui.Space(7)   // .xd-icon .i 28px
	detailHeadGap    = ui.Space(3.5) // .xd-head gap 14px
	detailCloseInset = ui.Space(3)   // .xd .x top-3 right-3
	detailDescAbove  = ui.Space(4)
	detailShotsAbove = ui.Space(4.5)
	detailShotGap    = ui.Space(2)
	detailSecAbove   = ui.Space(5) // .xd-facts and .xd-sec margin-top 20px
	detailFactsGapX  = ui.Space(7)
	detailFactsGapY  = ui.Space(2.5)
	detailPlatsGapX  = ui.Space(3.5)
	detailFootTop    = ui.Space(4) // .xd .ft padding 16px 24px 20px
	detailFootBelow  = ui.Space(5)
)

const (
	detailNameSize    = unit.Sp(18) // .xd-head h4
	detailNameLeading = unit.Sp(26)
	detailDescSize    = unit.Sp(15) // .xd-desc, prose at the Markdown size
	detailDescLeading = unit.Sp(23)
	// detailShotShare is how much of the body's width one screenshot takes,
	// so the next one shows at the edge: grid-auto-columns 88%.
	detailShotShare = 0.88
)

var (
	icDetailClose  = icons.Lucide("x")
	icDetailReadme = icons.Lucide("external-link")
)

// catalogDetail is the open listing's dialog. shown outlives id so the
// dialog keeps its words while it closes.
type catalogDetail struct {
	id, shown                      string
	state                          ui.DialogState
	close, readme, install, update widget.Clickable
	body, shots                    widget.List
}

func (d *catalogDetail) show(id string) {
	d.id, d.shown = id, id
	d.body = widget.List{List: layout.List{Axis: layout.Vertical}}
	d.shots = widget.List{List: layout.List{Axis: layout.Horizontal}}
}

// detailEntry is the listing shown, while the catalog still lists it.
func detailEntry(cat *backend.ExtensionCatalog, id string) (extension.CatalogEntry, bool) {
	if id == "" {
		return extension.CatalogEntry{}, false
	}
	i := slices.IndexFunc(cat.Catalog.Extensions, func(e extension.CatalogEntry) bool { return e.ID == id })
	if i < 0 {
		return extension.CatalogEntry{}, false
	}
	return cat.Catalog.Extensions[i], true
}

// layoutCatalogDetail draws the detail of the card last clicked, closed while
// the install confirmation it led to is open.
func (s *settingsScreen) layoutCatalogDetail(gtx C, th *ui.Theme, t extensionTasks) {
	p := &s.extensions
	d := &p.grid.detail
	if p.catalog == nil || (d.id == "" && !d.state.Animating(gtx)) {
		return
	}
	cat, _ := p.catalog.read()
	if cat == nil {
		return
	}
	e, ok := detailEntry(cat, d.shown)
	if !ok {
		d.id = ""
		return
	}
	var installed *backend.Extension
	if p.store != nil {
		if x, has := installedAs(p.store.all(), e.ID); has {
			installed = &x
		}
	}
	s.detailClicks(gtx, d, e, installed)
	dlg := th.Dialog(&d.state, d.id != "", "", "")
	dlg.MaxWidth, dlg.Root = ui.DialogWidthXL, s.root
	dlg.Raw = func(gtx C) D {
		return s.detailPanel(gtx, th.On(th.Popover), cat, e, installed, t)
	}
	if dlg.Layout(gtx) {
		d.id = ""
	}
}

// detailClicks acts on the detail's buttons: Install stages the listed
// release and leads to the confirmation, Update goes through the row's.
func (s *settingsScreen) detailClicks(gtx C, d *catalogDetail, e extension.CatalogEntry, installed *backend.Extension) {
	if d.close.Clicked(gtx) {
		d.id = ""
	}
	if d.readme.Clicked(gtx) {
		go func() { _ = s.client.OpenExternal(s.ctx, e.Repo) }()
	}
	if d.install.Clicked(gtx) {
		s.prepareInstall(installFromCatalog, e.Repo)
	}
	if d.update.Clicked(gtx) && installed != nil {
		d.id = ""
		s.updateExtension(*installed)
	}
}

// detailPanel is the scrolling body over the fixed footer, the × in the
// corner, no taller than the window less its margin.
func (s *settingsScreen) detailPanel(gtx C, th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry, installed *backend.Extension, t extensionTasks) D {
	d := &s.extensions.grid.detail
	width := gtx.Constraints.Max.X
	gtx.Constraints.Min = image.Point{}
	macro := op.Record(gtx.Ops)
	foot := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D { return s.detailFooter(gtx, th, cat, e, installed, t) }),
	)
	footCall := macro.Stop()
	body := gtx
	body.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-gtx.Dp(detailMargin)-foot.Size.Y)
	sections := s.detailSections(th, cat, e)
	bd := th.ThinScrollArea(&d.body).Layout(body, len(sections), func(gtx C, i int) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		inset := layout.Inset{Left: detailPad, Right: detailPad}
		if i == 0 {
			inset.Top = detailPad
		}
		if i == len(sections)-1 {
			inset.Bottom = detailBodyBelow
		}
		if sections[i].bleed {
			inset.Left, inset.Right = 0, 0
		}
		return inset.Layout(gtx, sections[i].w)
	})
	stack := op.Offset(image.Pt(0, bd.Size.Y)).Push(gtx.Ops)
	footCall.Add(gtx.Ops)
	stack.Pop()
	closeAt := op.Offset(image.Pt(width-gtx.Dp(detailCloseInset)-gtx.Dp(ui.Space(8)), gtx.Dp(detailCloseInset))).Push(gtx.Ops)
	b := th.Button(&d.close, "")
	b.Icon, b.Variant, b.Size = icDetailClose, ui.VariantGhost, ui.SizeIconSM
	b.Layout(gtx)
	closeAt.Pop()
	return D{Size: image.Pt(width, bd.Size.Y+foot.Size.Y)}
}

// detailSection is one child of the body; a bleeding one runs to the
// panel's edges and pads itself.
type detailSection struct {
	w     layout.Widget
	bleed bool
}

func (s *settingsScreen) detailSections(th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry) []detailSection {
	desc := th.Text(detailDescSize, entryDescription(e)).Muted().Leading(detailDescLeading).Wrapped()
	out := []detailSection{
		{w: func(gtx C) D { return s.detailHead(gtx, th, e) }},
		{w: func(gtx C) D { return layout.Inset{Top: detailDescAbove}.Layout(gtx, desc.Layout) }},
	}
	if len(e.Screenshots) > 0 {
		out = append(out, detailSection{bleed: true, w: func(gtx C) D {
			return layout.Inset{Top: detailShotsAbove}.Layout(gtx, func(gtx C) D { return s.detailShots(gtx, th, e.Screenshots) })
		}})
	}
	above := func(w layout.Widget) detailSection {
		return detailSection{w: func(gtx C) D { return layout.Inset{Top: detailSecAbove}.Layout(gtx, w) }}
	}
	return append(out,
		above(func(gtx C) D { return detailFacts(gtx, th, e) }),
		above(func(gtx C) D { return detailPermissions(gtx, th, e) }),
		above(func(gtx C) D { return detailPlatforms(gtx, th, cat, e) }),
	)
}

// detailHead is the icon, the name and the publisher, clear of the ×.
func (s *settingsScreen) detailHead(gtx C, th *ui.Theme, e extension.CatalogEntry) D {
	icon := func(gtx C) D {
		size := gtx.Dp(detailIconSize)
		gtx.Constraints = layout.Exact(image.Pt(size, size))
		defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(detailIconRadius)).Push(gtx.Ops).Pop()
		if img := s.extensions.catalog.image(e.Icon); img != nil {
			return ui.ImageStyle{Src: img, Fit: widget.Cover, Position: layout.Center}.Layout(gtx)
		}
		ui.Fill(gtx, ui.MixOKLab(th.Surface, th.Background, catalogCoverMix), 0)
		return ui.CenteredIcon(gtx, s.extensions.glyph(string(e.Glyph)), detailIconGlyph, th.MutedForeground)
	}
	return layout.Inset{Right: ui.Space(9)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon),
			layout.Rigid(layout.Spacer{Width: detailHeadGap}.Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(detailNameSize, entryName(e)).Weight(font.SemiBold).Leading(detailNameLeading).Truncated().Layout),
					ui.VGap(0.5),
					layout.Rigid(func(gtx C) D { return publisherLine(gtx, th, e) }),
				)
			}),
		)
	})
}

// detailShots is the screenshots side by side, each most of the body wide so
// the next one shows, with its alt for a screen reader.
func (s *settingsScreen) detailShots(gtx C, th *ui.Theme, shots []extension.CatalogImage) D {
	d := &s.extensions.grid.detail
	pad := gtx.Dp(detailPad)
	w := int(float32(gtx.Constraints.Max.X-2*pad) * detailShotShare)
	h := w * catalogCoverH / catalogCoverW
	gtx.Constraints.Max.Y = h
	return d.shots.Layout(gtx, len(shots), func(gtx C, i int) D {
		inset := layout.Inset{Left: detailShotGap}
		if i == 0 {
			inset.Left = detailPad
		}
		if i == len(shots)-1 {
			inset.Right = detailPad
		}
		return inset.Layout(gtx, func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(w, h))
			defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
			semantic.DescriptionOp(shots[i].Alt).Add(gtx.Ops)
			if img := s.extensions.catalog.image(shots[i].URL); img != nil {
				ui.ImageStyle{Src: img, Fit: widget.Cover, Position: layout.NW}.Layout(gtx)
			} else {
				ui.Fill(gtx, th.Over(th.Accent), 0)
			}
			ui.InsetBorder(gtx, gtx.Constraints.Min, gtx.Dp(ui.RadiusLG), th.Over(th.Border))
			return D{Size: gtx.Constraints.Min}
		})
	})
}

// capsLabel is a section's uppercase label.
func capsLabel(th *ui.Theme, text string) layout.Widget {
	return th.Text(ui.Text2XS, strings.ToUpper(text)).Weight(font.SemiBold).Leading(text2XSLeading).Muted().Layout
}

// detailFacts is Version, Released and License, each under its label.
func detailFacts(gtx C, th *ui.Theme, e extension.CatalogEntry) D {
	fact := func(label, value string) ui.WrapChild {
		return ui.WrapItem(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(capsLabel(th, label)),
				ui.VGap(1),
				layout.Rigid(th.Text(ui.TextSM, value).Layout),
			)
		})
	}
	facts := []ui.WrapChild{
		fact(i18n.T("settings.extensionsSettings.catalog.version"), e.Version),
		fact(i18n.T("settings.extensionsSettings.catalog.released"), releasedDate(e.Released)),
	}
	if e.License != "" {
		facts = append(facts, fact(i18n.T("settings.extensionsSettings.catalog.license"), e.License))
	}
	return ui.Wrap{GapX: detailFactsGapX, GapY: detailFactsGapY}.Layout(gtx, facts...)
}

// detailPermissions is that it runs as you, then what else it can do, in the
// confirmation's words and tones.
func detailPermissions(gtx C, th *ui.Theme, e extension.CatalogEntry) D {
	children := []layout.FlexChild{
		layout.Rigid(capsLabel(th, i18n.T("settings.extensionsSettings.catalog.permissions"))),
		ui.VGap(1.5),
		layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.catalog.runsAsYou")).Muted().Wrapped().Layout),
	}
	if len(e.Permissions) > 0 {
		names := make([]string, len(e.Permissions))
		for i, p := range e.Permissions {
			names[i] = string(p)
		}
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return permissionList(gtx, th, names, nil) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// detailPlatforms is every system a listing can be built for, the ones
// without a build muted.
func detailPlatforms(gtx C, th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry) D {
	plats := make([]ui.WrapChild, len(catalogPlatforms))
	for i, p := range catalogPlatforms {
		l := th.Text(settingsDescSize, p.name)
		if _, ok := e.Builds[p.key]; !ok {
			l = l.Muted()
		}
		plats[i] = ui.WrapItem(l.Layout)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(capsLabel(th, i18n.T("settings.extensionsSettings.catalog.platforms"))),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return ui.Wrap{GapX: detailPlatsGapX, GapY: ui.Space(1)}.Layout(gtx, plats...) }),
	)
}

// detailFooter is the note on the left, README on GitHub and the action the
// listing allows: Install, Update to a newer release of the installed one, or
// nothing when there is no build for this system.
func (s *settingsScreen) detailFooter(gtx C, th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry, installed *backend.Extension, t extensionTasks) D {
	d := &s.extensions.grid.detail
	readme := th.Button(&d.readme, i18n.T("settings.extensionsSettings.catalog.readme"))
	readme.Variant, readme.Size, readme.Icon = ui.VariantGhost, ui.SizeSM, icDetailReadme
	note, action := s.detailAction(th, cat, e, installed, t)
	children := []layout.FlexChild{layout.Flexed(1, note), ui.Gap(2), layout.Rigid(readme.Layout)}
	if action != nil {
		children = append(children, ui.Gap(2), layout.Rigid(action))
	}
	return layout.Inset{Top: detailFootTop, Bottom: detailFootBelow, Left: detailPad, Right: detailPad}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// detailAction is the footer's note and its button, nil for none.
func (s *settingsScreen) detailAction(th *ui.Theme, cat *backend.ExtensionCatalog, e extension.CatalogEntry, installed *backend.Extension, t extensionTasks) (note, action layout.Widget) {
	d := &s.extensions.grid.detail
	note = layout.Spacer{}.Layout
	if text := t.errorAt(installFromCatalog); text != "" {
		note = th.Text(ui.TextXS, text).In(th.Destructive).Wrapped().Layout
	}
	switch _, built := e.Builds[cat.Platform]; {
	case installed != nil:
		if t.errorAt(installFromCatalog) == "" {
			note = installedNote(th, installed.Version)
		}
		if installed.Latest == "" {
			return note, nil
		}
		b := th.Button(&d.update, i18n.T("settings.extensionsSettings.updateTo", i18n.Args{"version": installed.Latest}))
		b.Variant, b.Size, b.Disabled = ui.VariantOutline, ui.SizeSM, t.installing != ""
		return note, b.Layout
	case !built:
		return th.Text(settingsDescSize, i18n.T("settings.extensionsSettings.catalog.noBuild", i18n.Args{"platform": platformName(cat.Platform)})).Muted().Wrapped().Layout, nil
	}
	b := th.Button(&d.install, i18n.T("settings.extensionsSettings.install"))
	if t.installing == installFromCatalog {
		b.Text = i18n.T("settings.extensionsSettings.installing")
	}
	b.Size, b.Disabled = ui.SizeSM, t.installing != ""
	return note, b.Layout
}

// installedNote is the installed version beside the pass dot.
func installedNote(th *ui.Theme, version string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ui.StatusDot(gtx, th.TonePass) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.catalog.installedVersion", i18n.Args{"version": version})).Muted().Layout),
		)
	}
}
