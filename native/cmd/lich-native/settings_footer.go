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
	flagCustomizeFooter = flag.Bool("customize-footer", false, "start as the footer menu's Customize footer… lands, for -shot")
	flagSettings        = flag.String("settings", "", "open Settings on this section at start, for -shot")
	flagSettingsQuery   = flag.String("settings-query", "", "type this into the Settings search at start, for -shot with -settings")
)
