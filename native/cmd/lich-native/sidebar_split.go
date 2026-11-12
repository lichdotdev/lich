package main

import (
	"time"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// A layout's tab has a sidebar of its own: the layout's sessions and no
// others, under the project each belongs to. New Session, the filter and the
// services are a project's, so they stay on its tab.

const splitGroupPrefix = "__layout__:"

// splitGroup is one project's sessions in the layout on screen, as a block
// that hangs them from their checkouts and their mothers the way the
// project's own sidebar does (blockRows).
type splitGroup struct {
	project string // the project's name
	block   blockView
}

// splitSidebar is the layout on screen as its sidebar draws it: its name and
// its sessions by project, in the order the panes read. It reports false
// when no layout is on screen.
func (m *model) splitSidebar(now time.Time) (id, name string, groups []splitGroup, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.shownLayoutLocked()
	if !ok || m.console != "" {
		return "", "", nil, false
	}
	at := map[string]int{}
	for _, id := range t.sessions() {
		p, i := m.sessionAt(id)
		if p == nil {
			continue
		}
		g, seen := at[p.ID]
		if !seen {
			g = len(groups)
			at[p.ID] = g
			groups = append(groups, splitGroup{project: p.Name, block: blockView{key: splitGroupPrefix + p.ID, byCheckout: true, ofLayout: true, facts: map[string]checkoutFacts{}}})
		}
		sv := m.viewOf(*p, p.Sessions[i], now)
		sv.guest = true
		b := &groups[g].block
		b.sessions = append(b.sessions, sv)
		b.facts[sv.path] = m.checkoutFactsOf(*p, sv.path)
	}
	return t.ID, t.Name, groups, true
}

// aloneLocked keeps a session that is a pane of a layout off the stage on
// its own: with no layout on screen, the project shows its first session in
// none, or nothing. Callers hold m.mu.
func (m *model) aloneLocked() {
	p := m.shownProject()
	if p == nil || m.shownLayout != "" || m.layouts.of(m.active) < 0 {
		return
	}
	m.active = ""
	for _, s := range p.Sessions {
		if m.layouts.of(s.ID) < 0 {
			m.active = s.ID
			return
		}
	}
}

// everySessionInALayout reports whether the shown project has sessions and
// each is a pane of a layout, so its tab has none to show.
func (m *model) everySessionInALayout() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.shownProject()
	if p == nil || len(p.Sessions) == 0 {
		return false
	}
	for _, s := range p.Sessions {
		if m.layouts.of(s.ID) < 0 {
			return false
		}
	}
	return true
}

// splitHeader is the layout's name and the collapse button.
func (s *sidebar) splitHeader(gtx C, th *ui.Theme, name string, act *sidebarAction) D {
	h := &s.header
	if h.collapse.Clicked(gtx) {
		s.setOpen(gtx, false, act)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Left: rowInset}.Layout(gtx, th.Text(ui.TextSM, name).Weight(font.Medium).Layout)
		}),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D {
			tip := th.Tooltip(s.root, s.hint("collapse"), i18n.T("sidebar.sessionSidebar.collapse"))
			tip.Side = ui.SideBottom
			return tip.Layout(gtx, sideButton{click: &h.collapse, icon: icCollapse, morph: &s.morph, glyph: glyphToggle}.layout(th))
		}),
	)
}

// splitList is the layout's sessions, each project's under its name and in
// the project sidebar's tree and motion (blockCards): checkouts, mothers and
// children, browsers. No card is dragged or lit: the order is the project's,
// and the panes beside the list show which sessions are open and which has
// the focus.
func (s *sidebar) splitList(gtx C, th *ui.Theme, layoutID string, groups []splitGroup, act *sidebarAction) D {
	f := blockFrame{settled: s.splitShown == layoutID, act: act}
	s.splitShown = layoutID
	var rows []layout.FlexChild
	for g, group := range groups {
		if g > 0 {
			rows = append(rows, ui.VGap(1.5))
		}
		rows = append(rows,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: rowInset}.Layout(gtx, th.Text(ui.TextXS, group.project).Muted().Layout)
			}),
			layout.Rigid(func(gtx C) D {
				st := s.blockState(group.block)
				st.cards.Disabled = true
				return s.blockCards(gtx, th, group.block, st, f, false)
			}),
		)
	}
	return s.scrollColumn(gtx, th).Layout(gtx, 1, func(gtx C, _ int) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Right: ui.Space(2) - ui.ResizeWidth}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	})
}
