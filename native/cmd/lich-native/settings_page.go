package main

import (
	"image"
	"log"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// settingsColumn is the pane's reading column, centred in a wider pane.
var settingsColumn = unit.Dp(760)

// pageSeparator joins a section id and a page id in the remembered section
// and the -settings flag: "general/notifications".
const pageSeparator = "/"

// settingsPage is one nav entry of a split section: the rows of one group,
// titled with the group's own name.
type settingsPage struct {
	id       string
	groupKey string
	icon     *icons.Icon
}

func (p settingsPage) label() string { return i18n.T(p.groupKey) }

// settingsPageOf is the page part of a "section/page" id, "" without one.
func settingsPageOf(raw string) string {
	_, page, _ := strings.Cut(raw, pageSeparator)
	return page
}

// page is the section's page id, its first page when id names none, and
// false for a section that is not split.
func (s settingsSection) page(id string) (settingsPage, bool) {
	if len(s.pages) == 0 {
		return settingsPage{}, false
	}
	if i := slices.IndexFunc(s.pages, func(p settingsPage) bool { return p.id == id }); i >= 0 {
		return s.pages[i], true
	}
	return s.pages[0], true
}

// pageOfRow is the page of the section whose group holds the row titled
// title, "" when none does.
func (s settingsSection) pageOfRow(title string) string {
	for _, e := range s.rows {
		if e.titleText() != title {
			continue
		}
		for _, p := range s.pages {
			if p.groupKey == e.groupKey {
				return p.id
			}
		}
	}
	return ""
}

// only is the section cut down to page p: its rows, its label and its icon,
// so a page reads its badge and its dimming the way a section does. The rows
// lose their group, which the page's title already names.
func (s settingsSection) only(p settingsPage) settingsSection {
	s.labelKey, s.icon, s.pages = p.groupKey, p.icon, nil
	s.rows, s.unported = rowsIn(s.rows, p.groupKey), rowsIn(s.unported, p.groupKey)
	return s
}

func rowsIn(entries []settingEntry, groupKey string) []settingEntry {
	var in []settingEntry
	for _, e := range entries {
		if e.groupKey == groupKey {
			e.groupKey = ""
			in = append(in, e)
		}
	}
	return in
}

// shown is the section as the pane draws it: the page open on it when it is
// split.
func (s *settingsScreen) shown() settingsSection {
	sec := sectionByID(s.section)
	if p, ok := sec.page(s.page); ok {
		return sec.only(p)
	}
	return sec
}

// openPage opens section on page, "" for its first, and remembers both.
func (s *settingsScreen) openPage(section, page string) {
	s.section, s.page = section, page
	s.extensions.view.Selected = extensionsViewCatalog
	if s.theme != nil {
		s.theme.view.Selected = themesViewCatalog
	}
	saved := section
	if page != "" {
		saved += pageSeparator + page
	}
	go func() {
		if err := s.client.SetSetting(s.ctx, settingsSectionKey, "", saved); err != nil {
			log.Printf("settings: save section: %v", err)
		}
	}()
}

// readingColumn lays w out in the pane's reading column, centred. The
// extension and theme catalogs keep the pane's width: their grids take
// columns as the pane grows.
func (s *settingsScreen) readingColumn(gtx C, w layout.Widget) D {
	if s.section == sectionExtensions || s.onThemesPage() {
		return w(gtx)
	}
	full := gtx.Constraints.Max.X
	width := min(full, gtx.Dp(settingsColumn))
	gtx.Constraints.Max.X, gtx.Constraints.Min.X = width, min(gtx.Constraints.Min.X, width)
	defer op.Offset(image.Pt((full-width)/2, 0)).Push(gtx.Ops).Pop()
	d := w(gtx)
	d.Size.X = full
	return d
}

// pageTitle is the pane's title, the page's count beside it where it has
// one. The Footer page's title also carries the editor's description and its
// Restore default, which its own row would otherwise repeat under the same
// name.
func (s *settingsScreen) pageTitle(th *ui.Theme, sec settingsSection) layout.Widget {
	return func(gtx C) D {
		footer := sec.labelKey == groupFooter && s.scope == ""
		title := func(gtx C) D {
			line := []layout.FlexChild{layout.Rigid(th.Text(settingsTitle, sec.label()).Weight(font.SemiBold).Leading(32).Layout)}
			if count := s.pageCount(sec); count != "" {
				line = append(line, ui.Gap(2.5), layout.Rigid(th.Text(ui.TextXS, count).Muted().Mono().Layout))
			}
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, line...)
		}
		if !footer {
			return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, title)
		}
		desc, _ := s.footer.editorState()
		return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, title),
						layout.Rigid(func(gtx C) D { return s.footer.restoreButton(gtx, th) }),
					)
				}),
				ui.VGap(1),
				layout.Rigid(th.Text(settingsDescSize, desc).Muted().Leading(20).Layout),
			)
		})
	}
}

// pageCount is the count beside a page's title: how many of the
// Notifications switches are on, how many items the footer shows; "" on any
// other page.
func (s *settingsScreen) pageCount(sec settingsSection) string {
	switch sec.labelKey {
	case groupNotifications:
		p, _ := s.notify.snapshot()
		on := 0
		if p.attention == optInYes {
			on++
		}
		if p.finishedTurn {
			on++
		}
		return i18n.T("settings.notificationsSettings.onCount", i18n.Args{"on": on, "count": len(sec.rows)})
	case groupFooter:
		if !s.footer.cost.isReady() {
			return ""
		}
		l, _ := s.footer.snapshot()
		return i18n.T("settings.footerSettings.shownCount", i18n.Args{"count": len(l.Left) + len(l.Right)})
	}
	return ""
}
