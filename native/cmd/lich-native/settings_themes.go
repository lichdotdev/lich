package main

import (
	"image"
	"slices"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// themesPage is Appearance's Themes page: the catalog, and the installed
// themes on a tab beside it.
const themesPage = "themes"

// themesCatalogKey and themesInstalledKey name the page's two search results;
// nothing is stored under them.
const (
	themesCatalogKey   = "themes.catalog"
	themesInstalledKey = "themes.installed"
)

// The page's views, in the order of their tabs.
const (
	themesViewCatalog = iota
	themesViewInstalled
)

var (
	themesTitleBelow = ui.Space(4)
	themesViewsBelow = ui.Space(4)
)

// onThemesPage is whether the pane shows the Themes page.
func (s *settingsScreen) onThemesPage() bool {
	return s.section == sectionAppearance && s.page == themesPage
}

// themesTitle is the page's title with Install from… at its right, which
// opens the import dialog: a repository URL or a file.
func (s *settingsScreen) themesTitle(th *ui.Theme, label string) layout.Widget {
	return func(gtx C) D {
		p := s.theme
		if p.installFrom.Clicked(gtx) {
			p.openImport()
			// The dialogs were laid out before the title took the click.
			gtx.Execute(op.InvalidateCmd{})
		}
		open := th.Button(&p.installFrom, i18n.T("settings.themesPage.installFrom"))
		open.Variant, open.Size, open.Icon = ui.VariantOutline, ui.SizeSM, icImportTheme
		return layout.Inset{Bottom: themesTitleBelow}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, th.Text(settingsTitle, label).Weight(font.SemiBold).Leading(32).Layout),
				layout.Rigid(open.Layout),
			)
		})
	}
}

// themesPaneRows is the page under its title: the tabs, then the catalog or
// the installed themes.
func (s *settingsScreen) themesPaneRows(th *ui.Theme) []layout.Widget {
	p := s.theme
	v := p.look.view()
	rows := []layout.Widget{s.themesViews(th, v)}
	if p.view.Selected == themesViewInstalled {
		return append(rows, s.installedThemeRows(th, v)...)
	}
	return append(rows, s.themeCatalogRows(th, v)...)
}

// themesViews is the line of tabs on the seam over the view, and on Installed
// Check for updates at the right while a theme from a repository is installed. The view was built from the tab
// before this lays out, so a change here asks for the frame that shows it.
func (s *settingsScreen) themesViews(th *ui.Theme, v appearanceView) layout.Widget {
	return func(gtx C) D {
		p := s.theme
		tabs := th.Tabs(&p.view,
			ui.TabsTrigger{Text: i18n.T("settings.themesPage.catalogTab")},
			ui.TabsTrigger{Text: i18n.T("settings.themesPage.installedTab"), Trailing: installedThemesMark(th, v)},
		)
		tabs.Variant = ui.TabsVariantLine
		line := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return tabs.Layout(gtx) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
		}
		if p.view.Selected == themesViewInstalled {
			line = append(line, p.installedActions(gtx, th, v)...)
		}
		d := layout.Inset{Bottom: themesViewsBelow}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...) }),
			)
		})
		if p.view.Changed() {
			gtx.Execute(op.InvalidateCmd{})
		}
		return d
	}
}

// installedActions are the Installed view's controls on the tabs' line;
// Auto light/dark stands on the in-use theme's card.
func (p *themePicker) installedActions(gtx C, th *ui.Theme, v appearanceView) []layout.FlexChild {
	if !slices.ContainsFunc(v.themes, func(e themeEntry) bool { return e.def.Source != nil }) {
		return nil
	}
	checking := p.snapshot().checking
	if p.check.Clicked(gtx) && !checking {
		p.checkUpdates()
	}
	b := th.Button(&p.check, i18n.T("settings.themePicker.checkUpdates"))
	b.Variant, b.Size, b.Icon, b.Disabled = ui.VariantGhost, ui.SizeSM, icRefresh, checking
	return []layout.FlexChild{layout.Rigid(b.Layout)}
}

// installedThemesMark is what follows the Installed tab's name: how many
// themes were installed, and a dot when one waits on the tab, destructive for
// a theme that can't load, the wait tone for an update.
func installedThemesMark(th *ui.Theme, v appearanceView) layout.Widget {
	n, update := 0, false
	for _, e := range v.themes {
		if e.def.Origin == themeOriginCustom {
			n++
			update = update || e.def.Latest != nil
		}
	}
	if n == 0 && len(v.broken) == 0 {
		return nil
	}
	return func(gtx C) D {
		mark := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(n)).Mono().Muted().Layout)}
		switch {
		case len(v.broken) > 0:
			mark = append(mark, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return ui.StatusDot(gtx, th.Destructive) }))
		case update:
			mark = append(mark, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return ui.StatusDot(gtx, th.ToneWait) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, mark...)
	}
}

// installedIDs is every theme the window has, by id.
func installedIDs(v appearanceView) map[string]bool {
	ids := make(map[string]bool, len(v.themes))
	for _, e := range v.themes {
		ids[e.def.ID] = true
	}
	return ids
}
