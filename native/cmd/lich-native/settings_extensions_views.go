package main

import (
	"image"
	"image/color"
	"slices"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The pane's views, in the order of their tabs.
const (
	extensionsViewCatalog = iota
	extensionsViewInstalled
)

var extensionsViewsBelow = ui.Space(4)

// extensionsViews is the line of tabs on the seam over the view: Catalog and
// Installed, and on Installed its own actions at the right, Check for updates
// for a list holding an extension from GitHub, then Open folder. The view was
// built from the tab before this lays out, so a change here asks for the
// frame that shows it.
func (s *settingsScreen) extensionsViews(th *ui.Theme, list []backend.Extension) layout.Widget {
	return func(gtx C) D {
		p := &s.extensions
		tabs := th.Tabs(&p.view,
			ui.TabsTrigger{Text: i18n.T("settings.extensionsSettings.catalog.title")},
			ui.TabsTrigger{Text: i18n.T("settings.extensionsSettings.installedTab"), Trailing: installedMark(th, list)},
		)
		tabs.Variant = ui.TabsVariantLine
		line := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return tabs.Layout(gtx) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
		}
		if p.view.Selected == extensionsViewInstalled {
			line = append(line, s.installedActions(gtx, th, list)...)
		}
		d := layout.Inset{Bottom: extensionsViewsBelow}.Layout(gtx, func(gtx C) D {
			return withErrorLine(gtx, th, p.snapshot().errorAt(errAtFolder), rowPadX, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...) }),
					ui.VGap(1),
				)
			})
		})
		if p.view.Changed() {
			gtx.Execute(op.InvalidateCmd{})
		}
		return d
	}
}

// installedActions are the Installed view's buttons on the tabs' line.
func (s *settingsScreen) installedActions(gtx C, th *ui.Theme, list []backend.Extension) []layout.FlexChild {
	p := &s.extensions
	var actions []layout.FlexChild
	if slices.ContainsFunc(list, func(e backend.Extension) bool { return e.Release != "" }) {
		if p.check.Clicked(gtx) {
			s.checkExtensionUpdates()
		}
		check := th.Button(&p.check, i18n.T("settings.extensionsSettings.checkUpdates"))
		check.Variant, check.Size, check.Icon = ui.VariantGhost, ui.SizeSM, icExtensionCheck
		if p.snapshot().checking {
			check.Text, check.Disabled = i18n.T("settings.extensionsSettings.checkingUpdates"), true
		}
		actions = append(actions, layout.Rigid(check.Layout), ui.Gap(2))
	}
	if p.folder.Clicked(gtx) {
		go s.openExtensionsFolder()
	}
	open := th.Button(&p.folder, i18n.T("settings.extensionsSettings.openFolder"))
	open.Variant, open.Size, open.Icon = ui.VariantGhost, ui.SizeSM, icExtensionsFolder
	return append(actions, layout.Rigid(open.Layout))
}

// installedMark is what follows the Installed tab's name: how many there are
// and the dot of installedAttention; nothing with none installed.
func installedMark(th *ui.Theme, list []backend.Extension) layout.Widget {
	if len(list) == 0 {
		return nil
	}
	return func(gtx C) D {
		mark := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(len(list))).Mono().Muted().Layout)}
		if tone, ok := installedAttention(th, list); ok {
			mark = append(mark, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return ui.StatusDot(gtx, tone) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, mark...)
	}
}

// installedAttention is the tone of the dot that calls to the Installed tab
// from the catalog: destructive when an extension failed, is invalid or was
// taken out of the catalog, else the wait tone when one has an update.
func installedAttention(th *ui.Theme, list []backend.Extension) (color.NRGBA, bool) {
	update := false
	for _, e := range list {
		if e.Revoked != "" || e.Status == api.ExtensionFailed || e.Status == api.ExtensionInvalid {
			return th.Destructive, true
		}
		update = update || e.Latest != ""
	}
	return th.ToneWait, update
}
