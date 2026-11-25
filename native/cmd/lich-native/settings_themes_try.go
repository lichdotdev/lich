package main

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The bar a theme on approval shows, from the approved mockup's .try
// (temas-catalogo, frame 3): bottom centre, a toast's surface.
var (
	trialBarBelow = ui.Space(3.5) // .try bottom 14px
	trialBarPad   = layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: ui.Space(3), Right: ui.Space(1.5)}
	trialBarGap   = float32(2.5) // .try gap 10px
)

// themeTrialBar is the bar's buttons, and the pack the theme on approval came
// from, which Keep installs when it is not yet.
type themeTrialBar struct {
	revert, keep ui.Clickable
	packID       string
	// motion grows the bar in from the window's foot.
	motion panelMotion
}

// layoutThemeTrial is the bar while a theme is worn on approval: Revert and
// Keep. Revert, Escape and closing Settings go back to the selection.
func (s *settingsScreen) layoutThemeTrial(gtx C, th *ui.Theme) {
	p := s.theme
	if p == nil || p.look == nil {
		return
	}
	if !s.open {
		p.look.endTrial()
		p.bar.motion.track(gtx.Now, false, "")
		return
	}
	e, ok := p.look.trying()
	if !ok {
		p.bar.motion.track(gtx.Now, false, "")
		return
	}
	b := &p.bar
	busy := p.snapshot().importing
	if b.revert.Clicked(gtx) || escapePressed(gtx) {
		p.look.endTrial()
	}
	if b.keep.Clicked(gtx) && !busy {
		p.keep(b.packID, e.def.ID)
	}
	e, ok = p.look.trying()
	b.motion.track(gtx.Now, ok, "")
	if ok {
		trialBar(gtx, th, b, e.def.Name, busy)
	}
}

// escapePressed is whether Escape went down this frame; a dialog laid out
// before the bar takes it first.
func escapePressed(gtx C) bool {
	pressed := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			return pressed
		}
		if k, isKey := ev.(key.Event); isKey && k.State == key.Press {
			pressed = true
		}
	}
}

// trialBar is the theme's name, Revert and Keep on a popover's surface,
// centred over the window's foot.
func trialBar(gtx C, th *ui.Theme, b *themeTrialBar, name string, busy bool) {
	on := th.On(th.Popover)
	revert := on.Button(&b.revert, i18n.T("settings.themesPage.revert"))
	revert.Variant, revert.Size, revert.Keys = ui.VariantOutline, ui.SizeSM, "Esc"
	keep := on.Button(&b.keep, i18n.T("settings.themesPage.keep"))
	keep.Size, keep.Disabled = ui.SizeSM, busy
	if busy {
		keep.Text = i18n.T("settings.importThemeDialog.installing")
	}
	window := gtx.Constraints.Max
	gtx.Constraints.Min = image.Point{}
	macro := op.Record(gtx.Ops)
	d := trialBarPad.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(on.Text(ui.TextSM, i18n.T("settings.themesPage.trying", i18n.Args{"name": name})).Weight(font.Medium).Layout),
			ui.Gap(trialBarGap),
			layout.Rigid(revert.Layout),
			ui.Gap(2),
			layout.Rigid(keep.Layout),
		)
	})
	call := macro.Stop()
	defer op.Offset(image.Pt((window.X-d.Size.X)/2, window.Y-gtx.Dp(trialBarBelow)-d.Size.Y)).Push(gtx.Ops).Pop()
	defer b.motion.push(gtx, d.Size, d.Size.X/2)()
	box := gtx
	box.Constraints = layout.Exact(d.Size)
	ui.ElevationMid.Paint(gtx, d.Size, gtx.Dp(ui.RadiusLG))
	ui.Fill(box, th.Popover, ui.RadiusLG)
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusLG), on.Over(ui.Alpha(on.Foreground, ui.FloatRingOpacity)))
	call.Add(gtx.Ops)
}

// keep makes the theme on approval the selection, installing its pack first
// when the theme is not installed yet.
func (p *themePicker) keep(packID, themeID string) {
	if _, ok := themeByID(p.look.view().themes, themeID); ok {
		p.look.choose(themeID)
		return
	}
	p.installCatalog(packID, false, func() { p.look.choose(themeID) })
}

// replacePack is the pack confirmation's Replace: the install it stopped,
// again with overwrite.
func (p *themePicker) replacePack(c packConflict) {
	if c.catalogID != "" {
		p.installCatalog(c.catalogID, true, nil)
		return
	}
	p.installRepository(c.url, true)
}

// installCatalog installs the listed pack id, then runs then (Keep's choice)
// once it went in. A conflict asks before replacing; a conflict or a failure
// lets a theme on approval go.
func (p *themePicker) installCatalog(id string, overwrite bool, then func()) {
	p.with(func(t *themeTasks) { t.importing = true })
	go func() {
		res, err := p.client.InstallCatalogTheme(p.ctx, id, overwrite)
		p.with(func(t *themeTasks) {
			t.importing = false
			switch {
			case err != nil:
			case len(res.Conflicts) > 0:
				t.pack = &packConflict{catalogID: id, conflicts: res.Conflicts}
			default:
				t.pack = nil
			}
		})
		switch {
		case err != nil:
			p.look.endTrial()
			p.fail(i18n.T("settings.appearanceSettings.installFailed", failedWith(err)))
		case len(res.Conflicts) > 0:
			p.look.endTrial()
		default:
			p.success(i18n.T("settings.appearanceSettings.installed", i18n.Args{"count": len(res.Themes), "pack": res.Pack, "version": res.Version}))
			p.reloadThemes()
			if then != nil {
				then()
			}
		}
	}()
}
