package main

import (
	"flag"

	"gioui.org/layout"

	"github.com/omartelo/lich/native/ui"
)

// footerRow is Settings › Appearance › Footer: the row with Restore default,
// and the editor and preview always open under it, as FooterSettings keeps
// them. A project's scope shows the row dimmed with no control: the layout
// is global.
func (s *settingsScreen) footerRow(gtx C, th *ui.Theme, r settingRow, first bool) D {
	r.description, _ = s.footer.editorState()
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return r.layout(gtx, th, first, s.footer.restoreButton) }),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: rowPadY}.Layout(gtx, func(gtx C) D { return s.footer.editorPanel(gtx, th) })
		}),
	)
}

// reveal opens Settings on section with the row titled title scrolled into
// view and lit, as a search result lands; the light starts on the next
// frame, which has the clock.
func (s *settingsScreen) reveal(section, title string, projects []settingsProject) {
	s.show("", projects)
	s.openSection(section)
	s.revealing = &settingHit{section: section, title: title}
}

var (
	flagCustomizeFooter  = flag.Bool("customize-footer", false, "start as the footer menu's Customize footer… lands, for -shot")
	flagShortcuts        = flag.Bool("shortcuts", false, "open the keyboard shortcuts overlay at start, for -shot")
	flagPalette          = flag.Bool("palette", false, "open the command palette at start, for -shot")
	flagPaletteQuery     = flag.String("palette-query", "", "type this into the command palette at start, for -shot with -palette (\">\" lists actions)")
	flagPaletteTab       = flag.String("palette-tab", "", "open the command palette on this tab (Sessions, Projects, Messages, History), for -shot with -palette")
	flagSettings         = flag.String("settings", "", "open Settings on this section at start, for -shot")
	flagSettingsQuery    = flag.String("settings-query", "", "type this into the Settings search at start, for -shot with -settings")
	flagPatchNotes       = flag.Bool("patch-notes", false, "open the patch notes dialog with Settings at start, for -shot with -settings updates")
	flagSettingsScope    = flag.String("settings-scope", "", "open Settings in this open project's scope (its id), for -shot with -settings")
	flagSettingsProvider = flag.String("settings-provider", "", "open this provider's screen in Settings › Providers at start, for -shot with -settings providers")
	flagSettingsAdd      = flag.Bool("settings-add-provider", false, "open the Add provider list in Settings › Providers at start, for -shot with -settings providers")
)
