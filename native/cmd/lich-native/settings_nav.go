package main

import (
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The nav's geometry and its dimmed rows' opacity, from the mockup.
const (
	navDimOpacity = 0.45
	// navWhereOpacity is the provider's name on the Providers row, opacity-70.
	navWhereOpacity = 0.7
)

var (
	navRowHeight   = ui.Space(8)
	navRowPadX     = rowInset // a tree row's, so the nav reads on the sidebar's grid
	navScopeHeight = ui.Space(10)
	navScopeMenuW  = settingsNavWidth - 2*ui.Space(2) // the trigger's width, the nav's inside
	navBadgeSize   = unit.Dp(18)
	navBadgePadX   = unit.Dp(5)
	navBadgeFill   = float32(0.2)
	icScopeChevron = icons.Lucide("chevrons-up-down")
	icScopeCheck   = icons.Lucide("check")
	icNavSearch    = icons.Lucide("search")
)

// settingsNav is the nav island's state: the scope selector, the search
// field and the rows under it.
type settingsNav struct {
	scopeMenu  ui.DropdownPanelState
	scopeHover ui.Clickable
	scopeRows  map[string]*ui.Clickable
	query      widget.Editor
	clear      ui.Clickable
	cursor     int
	searched   string // the query the cursor was placed for
	rows       map[string]*ui.Clickable
	hits       []ui.Clickable
	list       widget.List
	sections   widget.List
	closeTip   ui.Tooltip
	scopeTip   ui.Tooltip
}

func (n *settingsNav) init() {
	n.scopeRows, n.rows = map[string]*ui.Clickable{}, map[string]*ui.Clickable{}
	n.list.Axis, n.sections.Axis = layout.Vertical, layout.Vertical
}

// navEntry is one nav row: a section, or one page of a split one, cut down
// to that page so its badge and dimming read as the section's do.
type navEntry struct {
	sec     settingsSection
	section string
	page    string
}

func (e navEntry) key() string { return e.section + pageSeparator + e.page }

// navEntries are the nav's rows in order, the footer's or the body's.
func navEntries(footer bool) []navEntry {
	var entries []navEntry
	for _, sec := range settingsSections {
		if sec.footer != footer {
			continue
		}
		if len(sec.pages) == 0 {
			entries = append(entries, navEntry{sec: sec, section: sec.id})
		}
		for _, p := range sec.pages {
			entries = append(entries, navEntry{sec: sec.only(p), section: sec.id, page: p.id})
		}
	}
	return entries
}

// on reports whether e is the entry the pane shows.
func (s *settingsScreen) on(e navEntry) bool {
	if e.section != s.section {
		return false
	}
	p, split := sectionByID(s.section).page(s.page)
	return !split || p.id == e.page
}

// navLayout is the nav island: py-3 px-2, the scope selector, the search
// field, the sections (or the results while searching), and the footer group.
func (s *settingsScreen) navLayout(gtx C, th *ui.Theme, values scopedValues) D {
	n := &s.nav
	hits := searchSettings(n.query.Text(), s.providers.snapshot().launchEnabled()...)
	if q := n.query.Text(); q != n.searched {
		n.searched, n.cursor = q, 0
	}
	s.searchKeys(gtx, hits)
	s.focusSearchKey(gtx)
	return layout.Inset{Top: ui.Space(2), Left: ui.Space(2), Right: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					return s.scopeSelector(gtx, th, values)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					field := th.Input(&n.query, i18n.T("settings.settings.search"))
					field.Leading, field.LeadingSize, field.Height, field.TextSize, field.Clear = icNavSearch, unit.Dp(15), ui.Space(8), 13, &n.clear
					field.Trailing = th.KeyCap(formatCombo(searchShortcut, runtime.GOOS == "darwin")).Layout
					return field.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				if strings.TrimSpace(n.query.Text()) != "" {
					return s.results(gtx, th, hits)
				}
				return s.sectionList(gtx, th, values, false)
			}),
			layout.Rigid(func(gtx C) D { return s.navFooter(gtx, th, values) }),
		)
	})
}

// searchShortcut is the chord that puts the cursor in the search field, only
// while Settings is open.
var searchShortcut = combo{Mod: true, Key: "F"}

// focusSearchKey moves the focus to the search field on searchShortcut.
func (s *settingsScreen) focusSearchKey(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.Name(searchShortcut.Key), Required: key.ModShortcut})
		if !ok {
			return
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
			gtx.Execute(key.FocusCmd{Tag: &s.nav.query})
		}
	}
}

// searchKeys is onSearchKey: the arrows move through the results, Enter
// opens the one they are on, Escape empties the field.
func (s *settingsScreen) searchKeys(gtx C, hits []settingHit) {
	n := &s.nav
	names := []key.Name{key.NameEscape, key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter}
	filters := make([]event.Filter, len(names))
	for i, name := range names {
		filters[i] = key.Filter{Focus: &n.query, Name: name}
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		n.cursor = s.searchKey(gtx, e.Name, hits, n.cursor)
	}
}

// searchKey carries out one key and answers where the cursor is after it.
func (s *settingsScreen) searchKey(gtx C, name key.Name, hits []settingHit, cursor int) int {
	if name == key.NameEscape {
		s.nav.query.SetText("")
		return 0
	}
	if len(hits) == 0 {
		return 0
	}
	switch name {
	case key.NameDownArrow:
		return (cursor + 1) % len(hits)
	case key.NameUpArrow:
		return (cursor + len(hits) - 1) % len(hits)
	}
	s.choose(gtx, hits[min(cursor, len(hits)-1)])
	return cursor
}

// sectionList is the nav's entries under their headings, the scope note
// under them in a project's scope; the footer's entries have no heading.
func (s *settingsScreen) sectionList(gtx C, th *ui.Theme, values scopedValues, footer bool) D {
	var children []layout.Widget
	heading := ""
	for _, e := range navEntries(footer) {
		if g := e.sec.navGroupKey; g != heading {
			children = append(children, navHeading(th, i18n.T(g), heading == ""))
			heading = g
		}
		children = append(children, func(gtx C) D {
			return layout.Inset{Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return s.sectionRow(gtx, th, e, values) })
		})
	}
	if !footer && s.scope != "" {
		children = append(children, func(gtx C) D {
			return layout.Inset{Top: ui.Space(2.5), Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				l := th.Text(ui.TextXS, i18n.T("settings.settingsNav.scopeNote")).Muted().Leading(17.4)
				l.MaxLines = 0
				return l.Layout(gtx)
			})
		})
	}
	if footer {
		flex := make([]layout.FlexChild, len(children))
		for i, c := range children {
			flex[i] = layout.Rigid(c)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, flex...)
	}
	return th.ThinScrollArea(&s.nav.sections).Layout(gtx, len(children), func(gtx C, i int) D { return children[i](gtx) })
}

// navHeading is a group's label over its entries: small, muted and
// uppercase, a hairline running from it to the nav's edge.
func navHeading(th *ui.Theme, label string, first bool) layout.Widget {
	inset := layout.Inset{Left: navRowPadX, Right: navRowPadX, Top: ui.Space(2.5), Bottom: ui.Space(1)}
	if first {
		inset.Top = ui.Space(0.5)
	}
	return func(gtx C) D {
		return inset.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(label)).Muted().Weight(font.SemiBold).Layout),
				ui.Gap(2),
				layout.Flexed(1, th.Separator),
			)
		})
	}
}

// navFooter is the footer group: a separator, then Updates and Help.
func (s *settingsScreen) navFooter(gtx C, th *ui.Theme, values scopedValues) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return s.sectionList(gtx, th, values, true) }),
	)
}

// navClicks opens the section whose row was clicked before any row is
// drawn: a row handling its own click would leave the rows above it lit.
func (s *settingsScreen) navClicks(gtx C) {
	for _, e := range append(navEntries(false), navEntries(true)...) {
		if !clickableOf(s.nav.rows, e.key()).Clicked(gtx) {
			continue
		}
		s.openPage(e.section, e.page)
		if e.section == sectionProviders {
			s.providers.open = ""
		}
	}
}

// sectionRow is one nav row: the icon, the label and the override count,
// dimmed in a project's scope when the section has nothing a project sets.
func (s *settingsScreen) sectionRow(gtx C, th *ui.Theme, e navEntry, values scopedValues) D {
	sec := e.sec
	click := clickableOf(s.nav.rows, e.key())
	where := ""
	if p, ok := s.providerScreen(); ok && sec.id == sectionProviders && sec.id == s.section {
		where = p.name
	}
	badge := sectionBadge(sec, s.scope, s.projects, values)
	dim := s.scope != "" && !sec.projectScoped()
	return th.NavRow(gtx, click, s.on(e), navRowHeight, navRowPadX, func(gtx C, th *ui.Theme) D {
		fg, icon := th.Foreground, th.MutedForeground
		if dim {
			fg = th.Over(ui.Alpha(th.MutedForeground, navDimOpacity))
			icon = fg
		}
		children := []layout.FlexChild{
			layout.Rigid(inNodeColumn(func(gtx C) D { return sec.icon.Layout(gtx, ui.Space(4), icon) })),
			ui.Gap(1.5),
			layout.Flexed(1, th.Text(ui.TextSM, sec.label()).In(fg).Layout),
		}
		if where != "" {
			children = append(children, layout.Rigid(th.Text(ui.TextXS, where).In(th.Over(ui.Alpha(fg, navWhereOpacity))).Layout), ui.Gap(1.5))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			append(children, layout.Rigid(func(gtx C) D { return th.CountBadge(gtx, badge) }))...)
	})
}

// results are the nav while the field has text: the count, then each hit as
// its path over its title, the one the arrows are on filled.
func (s *settingsScreen) results(gtx C, th *ui.Theme, hits []settingHit) D {
	n := &s.nav
	if len(hits) == 0 {
		return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5), Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			l := th.Text(ui.TextXS, i18n.T("settings.settings.noMatch", i18n.Args{"query": strings.TrimSpace(n.query.Text())})).Muted()
			l.MaxLines = 0
			return l.Layout(gtx)
		})
	}
	if len(n.hits) < len(hits) {
		n.hits = make([]ui.Clickable, len(hits))
	}
	for i := range hits {
		if n.hits[i].Clicked(gtx) {
			n.cursor = i
			s.choose(gtx, hits[i])
		}
	}
	count := i18n.T("settings.settings.resultCount", i18n.Args{"count": len(hits)})
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Bottom: ui.Space(0.5)}.Layout(gtx, th.Text(ui.Text2XS, strings.ToUpper(count)).Muted().Mono().Layout)
		}),
		layout.Flexed(1, func(gtx C) D {
			return n.list.Layout(gtx, len(hits), func(gtx C, i int) D {
				return layout.Inset{Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return s.hitRow(gtx, th, i, hits[i]) })
			})
		}),
	)
}

func (s *settingsScreen) hitRow(gtx C, th *ui.Theme, i int, h settingHit) D {
	click := &s.nav.hits[i]
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		switch {
		case i == s.nav.cursor:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		on := th.On(bg)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: navRowPadX, Right: navRowPadX, Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					children := []layout.FlexChild{}
					if p := h.path(); p != "" {
						children = append(children, layout.Rigid(on.Text(ui.Text2XS, p).Muted().Mono().Layout))
					}
					children = append(children, layout.Rigid(on.Text(ui.TextSM, h.title).Leading(19).Layout))
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}
