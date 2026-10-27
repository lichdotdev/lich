package main

import (
	"context"
	"log"
	"slices"
	"strings"
	"time"
)

// The layouts on the model: which one is on screen, and the routing that
// keeps a session in one place. A session a layout holds is always shown in
// that layout; any other is shown alone in its project, which takes the
// layout off screen.

// loadLayouts reads the stored layouts once the workspace is loaded, and
// drops the panes of sessions closed while the window was away.
func (m *model) loadLayouts(ctx context.Context) error {
	raw, err := m.client.GetSetting(ctx, layoutsKey, "")
	if err != nil {
		return err
	}
	ls, err := parseLayouts(raw)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.layouts = ls
	pruned := m.layouts.prune(m.liveLocked)
	m.mu.Unlock()
	if pruned {
		m.saveLayouts()
	}
	return nil
}

// saveLayouts stores the layouts as they are now, off the caller.
func (m *model) saveLayouts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveLayoutsLocked()
}

// saveLayoutsLocked is saveLayouts for a caller holding m.mu.
func (m *model) saveLayoutsLocked() {
	raw, err := m.layouts.marshal()
	if err != nil {
		log.Printf("save layouts: %v", err)
		return
	}
	go func() {
		if err := m.client.SetSetting(context.Background(), layoutsKey, "", raw); err != nil {
			log.Printf("save layouts: %v", err)
		}
	}()
}

// liveLocked reports whether an open project holds session id. Callers hold
// m.mu.
func (m *model) liveLocked(id string) bool {
	p, _ := m.sessionAt(id)
	return p != nil
}

// routeLocked puts on screen the place session id is shown in: the layout
// holding it, focused on it, else its project with no layout. It reports
// whether the layouts changed. Callers hold m.mu.
func (m *model) routeLocked(id string) bool {
	p, _ := m.sessionAt(id)
	if p == nil {
		return false
	}
	m.activeProject, m.home = p.ID, p.ID == m.homeID
	i := m.layouts.of(id)
	if i < 0 {
		m.shownLayout = ""
		return false
	}
	t := &m.layouts.tabs[i]
	m.shownLayout = t.ID
	if t.Focus == id {
		return false
	}
	t.Focus = id
	return true
}

// refocusLayoutLocked puts the focused pane of the layout on screen back
// on screen after the layouts changed under it, and reports false when that
// layout is gone, which takes it off screen. Callers hold m.mu.
func (m *model) refocusLayoutLocked() bool {
	t := m.layouts.byID(m.shownLayout)
	if t == nil {
		m.shownLayout = ""
		return false
	}
	m.active = paneSession(t.Focus)
	if p, _ := m.sessionAt(m.active); p != nil {
		m.activeProject = p.ID
	}
	return true
}

// showLayout puts layout id on screen on the pane it last had in focus, and
// reports false when no layout has that id.
func (m *model) showLayout(ctx context.Context, id string) bool {
	m.mu.Lock()
	t := m.layouts.byID(id)
	if t == nil {
		m.mu.Unlock()
		return false
	}
	focus := t.Focus
	m.mu.Unlock()
	m.setActive(ctx, paneSession(focus))
	return true
}

// shownLayoutTab is a copy of the layout on screen, false with none.
func (m *model) shownLayoutTab() (layoutTab, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shownLayoutLocked()
}

func (m *model) shownLayoutLocked() (layoutTab, bool) {
	if m.shownLayout == "" {
		return layoutTab{}, false
	}
	t := m.layouts.byID(m.shownLayout)
	if t == nil {
		return layoutTab{}, false
	}
	c := *t
	c.Root = cloneNode(t.Root)
	return c, true
}

func cloneNode(n *paneNode) *paneNode {
	if n == nil {
		return nil
	}
	c := *n
	c.First, c.Second = cloneNode(n.First), cloneNode(n.Second)
	return &c
}

// layoutNameOfLocked is the name of the layout holding session id, "" for
// none. Callers hold m.mu.
func (m *model) layoutNameOfLocked(id string) string {
	if i := m.layouts.of(id); i >= 0 {
		return m.layouts.tabs[i].Name
	}
	return ""
}

// layoutOf is the id of the layout holding session id, "" for none.
func (m *model) layoutOf(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i := m.layouts.of(id); i >= 0 {
		return m.layouts.tabs[i].ID
	}
	return ""
}

// splitBeside puts session against target's edge and brings it into focus:
// in the layout holding target, else in a new layout of the two named after
// target. A session that is in another layout already stays where it is
// unless move is set, and a browser tab takes no one; it reports false when
// nothing was split.
func (m *model) splitBeside(ctx context.Context, target, session string, edge paneEdge, move bool) bool {
	m.mu.Lock()
	if target == session || !m.liveLocked(target) || !m.liveLocked(session) || m.layouts.locked(m.layouts.of(target)) {
		m.mu.Unlock()
		return false
	}
	if at := m.layouts.of(session); at >= 0 {
		if !move {
			m.mu.Unlock()
			return false
		}
		m.layouts.take(session)
	}
	if i := m.layouts.of(target); i >= 0 {
		m.layouts.tabs[i].split(target, session, edge)
	} else {
		p, i := m.sessionAt(target)
		m.layouts.open(p.Sessions[i].Label, target, session, edge)
	}
	m.mu.Unlock()
	m.setActive(ctx, session)
	m.saveLayouts()
	return true
}

// unsplit takes session out of its layout, leaving the session running. The
// pane that takes the focus comes on screen, or, with one pane left and so
// no layout, that pane's session alone.
func (m *model) unsplit(ctx context.Context, session string) {
	m.mu.Lock()
	i := m.layouts.of(session)
	if i < 0 {
		m.mu.Unlock()
		return
	}
	id := m.layouts.tabs[i].ID
	left, _ := m.layouts.take(session)
	next := left
	if t := m.layouts.byID(id); t != nil {
		next = paneSession(t.Focus)
	}
	if next == "" {
		// The session left its browser tab, which goes with it: it is what
		// stays on screen.
		next = session
	}
	m.mu.Unlock()
	m.setActive(ctx, next)
	m.saveLayouts()
}

// closeLayout drops layout id; its sessions keep running, the one it had in
// focus coming on screen alone.
func (m *model) closeLayout(ctx context.Context, id string) {
	m.mu.Lock()
	t := m.layouts.byID(id)
	if t == nil {
		m.mu.Unlock()
		return
	}
	focus, shown := paneSession(t.Focus), m.shownLayout == id
	m.layouts.close(id)
	m.mu.Unlock()
	if shown {
		m.setActive(ctx, focus)
	}
	m.saveLayouts()
}

// toggleZoom zooms the focused pane of the layout on screen over the others,
// or back.
func (m *model) toggleZoom() {
	m.mu.Lock()
	t := m.layouts.byID(m.shownLayout)
	if t == nil {
		m.mu.Unlock()
		return
	}
	t.Zoomed = !t.Zoomed
	m.mu.Unlock()
	m.saveLayouts()
}

// cyclePane moves the focus of the layout on screen to the pane after it in
// reading order.
func (m *model) cyclePane(ctx context.Context, step int) {
	t, ok := m.shownLayoutTab()
	if !ok {
		return
	}
	ids := t.panes()
	at := 0
	for i, id := range ids {
		if id == t.Focus {
			at = i
		}
	}
	m.focusPane(ctx, ids[(at+step+len(ids))%len(ids)])
}

// focusPane puts pane in focus: its session on screen, and in the layout
// holding it, the pane itself, which for a page is not its session's.
func (m *model) focusPane(ctx context.Context, pane string) {
	session := paneSession(pane)
	if active, ok := m.activeSession(); !ok || active.ID != session {
		m.setActive(ctx, session)
	}
	m.mu.Lock()
	changed := false
	if i := m.layouts.of(session); i >= 0 && m.layouts.tabs[i].Focus != pane {
		m.layouts.tabs[i].Focus = pane
		changed = true
	}
	m.mu.Unlock()
	if changed {
		m.saveLayouts()
	}
}

// setRatio stores a divider's new ratio once its drag ends.
func (m *model) setRatio(layoutID string, path []bool, ratio float64) {
	m.mu.Lock()
	t := m.layouts.byID(layoutID)
	if t == nil {
		m.mu.Unlock()
		return
	}
	n := t.Root
	for _, second := range path {
		if n.leaf() {
			m.mu.Unlock()
			return
		}
		if second {
			n = n.Second
		} else {
			n = n.First
		}
	}
	if n.leaf() {
		m.mu.Unlock()
		return
	}
	n.Ratio = clampRatio(ratio)
	m.mu.Unlock()
	m.saveLayouts()
}

// layoutStripTabs are the layouts as strip tabs, each but the one on screen
// with the most urgent status of its panes. Callers hold m.mu.
func (m *model) layoutStripTabs() []projectTab {
	tabs := make([]projectTab, 0, len(m.layouts.tabs))
	for _, t := range m.layouts.tabs {
		ids := t.sessions()
		labels := make([]string, 0, len(ids))
		for _, id := range ids {
			if p, i := m.sessionAt(id); p != nil {
				labels = append(labels, p.Sessions[i].Label)
			}
		}
		tab := projectTab{ID: t.ID, Name: t.Name, Path: strings.Join(labels, ", "), layout: true}
		if t.ID != m.shownLayout {
			tab.status = m.status.projectStatus(ids)
		}
		tabs = append(tabs, tab)
	}
	return tabs
}

// stripTabs is the whole strip, the projects and then the layouts, and the
// tab on screen.
func (m *model) stripTabs() ([]projectTab, string) {
	tabs, active := m.projectTabs()
	m.mu.Lock()
	defer m.mu.Unlock()
	tabs = append(tabs, m.layoutStripTabs()...)
	if _, ok := m.shownLayoutLocked(); ok {
		active = m.shownLayout
	}
	return tabs, active
}

// isLayout reports whether a strip tab id is a layout's.
func (m *model) isLayout(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.layouts.byID(id) != nil
}

// moveLayout puts layout id where over is in the strip, both being layouts;
// a project dragged among layouts, or a layout among projects, stays put.
func (m *model) moveLayout(id, over string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	from, to := -1, -1
	for i, t := range m.layouts.tabs {
		switch t.ID {
		case id:
			from = i
		case over:
			to = i
		}
	}
	if from < 0 || to < 0 {
		return false
	}
	t := m.layouts.tabs[from]
	m.layouts.tabs = slices.Insert(slices.Delete(m.layouts.tabs, from, from+1), to, t)
	return true
}

// viewsOf is what each of the sessions ids shows at now, by id; a session no
// open project holds is left out.
func (m *model) viewsOf(ids []string, now time.Time) map[string]sessionView {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]sessionView, len(ids))
	for _, id := range ids {
		if p, i := m.sessionAt(id); p != nil {
			out[id] = m.viewOf(*p, p.Sessions[i], now)
		}
	}
	return out
}

// projectNamesAndPaths are every open project's name and path, by id.
func (m *model) projectNamesAndPaths() (names, paths map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	names, paths = make(map[string]string, len(m.projects)), make(map[string]string, len(m.projects))
	for _, p := range m.projects {
		names[p.ID], paths[p.ID] = p.Name, p.Path
	}
	return names, paths
}

// layoutName is layout id's name, "" for no such layout.
func (m *model) layoutName(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.layouts.byID(id); t != nil {
		return t.Name
	}
	return ""
}

// sessionLabel is session id's label, "" for a session no open project holds.
func (m *model) sessionLabel(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, i := m.sessionAt(id); p != nil {
		return p.Sessions[i].Label
	}
	return ""
}

// renameLayout names layout id name, reporting false when no layout has
// that id.
func (m *model) renameLayout(id, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.layouts.byID(id)
	if t == nil {
		return false
	}
	t.Name = name
	return true
}

// movePane puts source's pane against target's edge, or swaps the two on the
// middle, in the layout holding both, and focuses source. It reports false
// when they are not panes of one layout.
func (m *model) movePane(ctx context.Context, source, target string, edge paneEdge) bool {
	m.mu.Lock()
	i := m.layouts.of(target)
	if i < 0 || m.layouts.of(source) != i || !m.layouts.tabs[i].move(source, target, edge) {
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()
	m.setActive(ctx, source)
	m.saveLayouts()
	return true
}

// rotateFocused turns the split holding the focused pane of the layout on
// screen between side by side and stacked.
func (m *model) rotateFocused() bool {
	m.mu.Lock()
	t := m.layouts.byID(m.shownLayout)
	if t == nil || !t.rotate(t.Focus) {
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()
	m.saveLayouts()
	return true
}

// showOtherThan puts another session of the project on screen when the one
// it shows alone is id, as when the session being dragged is the one its
// project last showed: what is dragged is never the only place to drop it.
// A session in no layout is picked first.
func (m *model) showOtherThan(ctx context.Context, id string) {
	m.mu.Lock()
	p := m.shownProject()
	if p == nil || m.active != id || m.shownLayout != "" {
		m.mu.Unlock()
		return
	}
	pick := ""
	for _, s := range p.Sessions {
		if s.ID == id {
			continue
		}
		if m.layouts.of(s.ID) < 0 {
			pick = s.ID
			break
		}
		if pick == "" {
			pick = s.ID
		}
	}
	m.mu.Unlock()
	if pick != "" {
		m.setActive(ctx, pick)
	}
}
