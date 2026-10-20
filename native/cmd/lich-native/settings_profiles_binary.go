package main

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// profileBinaryLinks are the ways out of the Profile row's binary warning.
type profileBinaryLinks struct {
	show, clear widget.Clickable
}

// profileBinaryWarning is what the Profile row says under its description
// when the open scope's sessions spawn from a binary of their own: a wrapper
// that sets its own login or config directory wins over the profile, and the
// Binary that holds it sits folded at the bottom of the screen. nil when no
// configured binary resolves.
//
// Use the default binary clears the open scope's own path only: a project
// following Global's path cannot drop it from here, so it offers Show binary
// alone.
func (s *settingsScreen) profileBinaryWarning(gtx C, c *profileSettings, values scopedValues, m providersMachine, p providerInfo, profile string) func(gtx C, th *ui.Theme) D {
	project, global := s.binLayers(values, m, p.id)
	scope, configured, _ := spawnedBin(project, global, p.binary)
	if configured == "" {
		return nil
	}
	links := &c.binary
	if links.show.Clicked(gtx) {
		s.choose(gtx, settingHit{section: sectionProviders, title: i18n.T("settings.providerBinary.title"), providerID: p.id})
	}
	ownPath := (s.scope == "") == (scope == binScopeGlobal)
	if ownPath && links.clear.Clicked(gtx) {
		s.writeBin(m, p.id, s.scope, "")
	}
	text := i18n.T("settings.settingsProfilesView.binaryOverrides", i18n.Args{"path": configured, "profile": profile})
	return func(gtx C, th *ui.Theme) D {
		buttons := []layout.FlexChild{layout.Rigid(profileBinaryLink(th, &links.show, i18n.T("settings.settingsProfilesView.showBinary")))}
		if ownPath {
			buttons = append(buttons, ui.Gap(2), layout.Rigid(profileBinaryLink(th, &links.clear, i18n.T("settings.settingsProfilesView.useDefaultBinary"))))
		}
		return layout.Flex{Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx C) D { return icBinWarn.Layout(gtx, binGlyphSize, th.ToneWait) })
			}),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(rowNoteSize, text).In(th.ToneWait)) }),
					ui.VGap(2),
					layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...) }),
				)
			}),
		)
	}
}

func profileBinaryLink(th *ui.Theme, click *widget.Clickable, label string) layout.Widget {
	b := th.Button(click, label)
	b.Size, b.Variant = ui.SizeXS, ui.VariantOutline
	return b.Layout
}
