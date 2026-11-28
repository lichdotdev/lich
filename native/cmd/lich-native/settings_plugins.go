package main

import (
	"slices"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// Settings › Plugins: lich's own plugin first, locked, then every plugin the
// providers load in the scope; one opens to a switch per provider, and per
// profile where each has its own, which the provider's own CLI flips
// (internal/library/plugins.go).

// pluginsPaneRows is the Plugins screen under its title, or a plugin's page.
func (s *settingsScreen) pluginsPaneRows(th *ui.Theme, sec settingsSection, lib backend.Library, project string) []layout.Widget {
	l := &s.library
	if name := l.open[api.LibraryPluginKind]; name != "" {
		for _, it := range lib.Plugins {
			if it.Name == name {
				return s.pluginDetail(th, sec, it, lib.Providers)
			}
		}
		l.open[api.LibraryPluginKind] = ""
	}
	rows := []layout.Widget{s.pageTitle(th, sec), func(gtx C) D { return s.lichPluginRow(gtx, th, lib.Plugin) }}
	for _, it := range lib.Plugins {
		rows = append(rows, func(gtx C) D { return s.pluginRow(gtx, th, it) })
	}
	return rows
}

// pluginRow is one provider plugin: its name, its version and the marks of
// the providers it is on in; a click opens its page.
func (s *settingsScreen) pluginRow(gtx C, th *ui.Theme, it backend.LibraryItem) D {
	l := &s.library
	row := clickableOf(l.rows, it.Kind+"/"+it.Name)
	if row.Clicked(gtx) {
		l.open[it.Kind] = it.Name
		s.invalidate()
	}
	var on []string
	for _, sw := range it.Switches {
		if sw.On && !slices.Contains(on, sw.Provider) {
			on = append(on, sw.Provider)
		}
	}
	return libraryRowFrame(gtx, th, row, func(gtx C) D {
		children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, it.Name).Weight(font.Medium).Layout)}
		if it.Description != "" {
			children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, it.Description).Muted().Mono().Layout))
		}
		children = append(children, layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return libraryMarks(gtx, th, on) }))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// pluginDetail is a plugin's page: a switch for each provider that loads it,
// under its name and one per profile when the provider has a switch for each,
// and the ones lich cannot press saying why.
func (s *settingsScreen) pluginDetail(th *ui.Theme, sec settingsSection, it backend.LibraryItem, list []backend.LibraryProvider) []layout.Widget {
	rows := s.libraryDetailHead(th, sec, it)
	for _, p := range list {
		var mine []backend.LibrarySwitch
		for _, sw := range it.Switches {
			if sw.Provider == p.ID {
				mine = append(mine, sw)
			}
		}
		switch {
		case len(mine) == 0:
		case mine[0].Reason != "":
			reason := i18n.T("settings.librarySettings.reason."+mine[0].Reason, i18n.Args{"provider": p.Name})
			rows = append(rows, func(gtx C) D { return libraryCannotRow(gtx, th, p, reason) })
		case len(mine) == 1 && mine[0].Profile == "":
			rows = append(rows, func(gtx C) D { return s.librarySwitchRow(gtx, th, it, p.ID, p.Name, "", "", 0) })
		default:
			rows = append(rows, func(gtx C) D { return libraryProviderHeading(gtx, th, p) })
			for _, sw := range mine {
				name := profileName(p, sw.Profile)
				rows = append(rows, func(gtx C) D {
					return s.librarySwitchRow(gtx, th, it, p.ID, name, sw.Profile, "", libraryProfileIndent)
				})
			}
		}
	}
	return rows
}

// profileName is the name of p's profile slug, the slug itself when the
// list no longer has it.
func profileName(p backend.LibraryProvider, slug string) string {
	for _, prof := range p.Profiles {
		if prof.Slug == slug {
			return prof.Name
		}
	}
	return slug
}

// lichPluginRow is lich's own plugin: its release and what it brings, and a
// lock, since every session lich starts runs it.
func (s *settingsScreen) lichPluginRow(gtx C, th *ui.Theme, p api.LibraryPlugin) D {
	l := &s.library
	version := i18n.T("settings.pluginSetting.notInstalled")
	if p.Version != "" {
		version = "v" + p.Version
	}
	brings := i18n.T("settings.librarySettings.pluginBrings", i18n.Args{"hooks": p.Hooks, "skills": p.Skills, "mcp": p.MCP})
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, "lich").Weight(font.Medium).Layout),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextXS, version).Muted().Mono().Layout),
			layout.Flexed(1, func(gtx C) D { return layout.E.Layout(gtx, th.Text(ui.TextXS, brings).Muted().Layout) }),
			ui.Gap(2.5),
			layout.Rigid(func(gtx C) D {
				return th.Tooltip(s.root, &l.lockTip, i18n.T("settings.librarySettings.pluginLocked")).Layout(gtx, func(gtx C) D {
					return icLibraryLock.Layout(gtx, ui.Space(3.5), th.MutedForeground)
				})
			}),
		)
	})
}
