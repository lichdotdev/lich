package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	"github.com/lichdotdev/lich/native/backend"
)

// A session's page is drawn beside its terminal wherever that terminal is,
// alone in its project or as a pane of a layout. No stored layout holds it,
// so it makes no tab and follows its session in and out of every split: the
// stage draws a copy of the layout on screen with the pages put in
// (withPages), and a session alone with a page as a layout of the two that
// no strip lists.

// pagesKey is the backend setting the shown pages persist under.
const pagesKey = "lich.native.pages"

// soloLayoutPrefix starts the id of the layout a session alone makes with
// its page, which names its divider and is no stored layout's.
const soloLayoutPrefix = "solo:"

// pageView is how a session's page sits against its terminal. Zoomed is for
// a session alone: in a layout, the layout's zoom covers the page too.
type pageView struct {
	Axis   splitAxis `json:"axis"`
	Ratio  float64   `json:"ratio"`
	Zoomed bool      `json:"zoomed,omitempty"`
	// focused is the page taking the keys instead of the terminal.
	focused bool
}

// pageViews are the pages on screen with their sessions, by session. A
// browser with no entry runs minimized.
type pageViews map[string]*pageView

// withPages puts each shown page in t as a pane after its session's, and the
// focus on the page that holds it. t is the caller's copy.
func (pv pageViews) withPages(t layoutTab) layoutTab {
	var walk func(n *paneNode)
	walk = func(n *paneNode) {
		if !n.leaf() {
			walk(n.First)
			walk(n.Second)
			return
		}
		p := pv[n.Session]
		if p == nil {
			return
		}
		page := browserPaneOf(n.Session)
		if p.focused && t.Focus == n.Session {
			t.Focus = page
		}
		*n = paneNode{Axis: p.Axis, Ratio: p.Ratio, First: &paneNode{Session: n.Session}, Second: &paneNode{Session: page}}
	}
	walk(t.Root)
	return t
}

func parsePages(raw string) (pageViews, error) {
	pv := pageViews{}
	if raw == "" {
		return pv, nil
	}
	if err := json.Unmarshal([]byte(raw), &pv); err != nil {
		return nil, fmt.Errorf("%s: %w", pagesKey, err)
	}
	for session, p := range pv {
		if p == nil || p.Axis != splitRow && p.Axis != splitColumn {
			return nil, fmt.Errorf("%s: page of %q: split axis, want row or column", pagesKey, session)
		}
		p.Ratio = clampRatio(p.Ratio)
	}
	return pv, nil
}

// loadPages reads the stored pages once the workspace is loaded, and drops
// those of sessions closed while the window was away.
func (m *model) loadPages(ctx context.Context) error {
	raw, err := m.client.GetSetting(ctx, pagesKey, "")
	if err != nil {
		return err
	}
	pv, err := parsePages(raw)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pages = pv
	m.prunePagesLocked(m.liveLocked)
	return nil
}

// prunePagesLocked drops every page whose session keep refuses. Callers hold
// m.mu.
func (m *model) prunePagesLocked(keep func(session string) bool) {
	pruned := false
	for session := range m.pages {
		if !keep(session) {
			delete(m.pages, session)
			pruned = true
		}
	}
	if pruned {
		m.savePagesLocked()
	}
}

// savePagesLocked stores the pages as they are now, off the caller, who
// holds m.mu.
func (m *model) savePagesLocked() {
	b, err := json.Marshal(m.pages)
	if err != nil {
		log.Printf("save pages: %v", err)
		return
	}
	go func() {
		if err := m.client.SetSetting(context.Background(), pagesKey, "", string(b)); err != nil {
			log.Printf("save pages: %v", err)
		}
	}()
}

// pageShown reports whether session's page is on screen with its terminal.
func (m *model) pageShown(session string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pages[session] != nil
}

// showPage puts session's page beside its terminal, at its right the first
// time, and brings it on screen in focus. It reports false for a session no
// open project holds.
func (m *model) showPage(ctx context.Context, session string) bool {
	m.mu.Lock()
	if !m.liveLocked(session) {
		m.mu.Unlock()
		return false
	}
	if m.pages[session] == nil {
		m.pages[session] = &pageView{Axis: splitRow, Ratio: 0.5}
		m.savePagesLocked()
	}
	m.mu.Unlock()
	m.focusPane(ctx, browserPaneOf(session))
	return true
}

// minimizePage takes session's page off the screen and leaves its browser
// running, for the agent and for showPage to bring back. It reports whether
// there was one.
func (m *model) minimizePage(session string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dropPageLocked(session)
}

func (m *model) dropPageLocked(session string) bool {
	if m.pages[session] == nil {
		return false
	}
	delete(m.pages, session)
	m.savePagesLocked()
	return true
}

// soloLayoutLocked is the active session alone with its page, false when it
// shows none or is not on screen. Callers hold m.mu.
func (m *model) soloLayoutLocked() (layoutTab, bool) {
	page := m.pages[m.active]
	p := m.shownProject()
	if page == nil || p == nil || !slices.ContainsFunc(p.Sessions, func(s backend.Session) bool { return s.ID == m.active }) {
		return layoutTab{}, false
	}
	return layoutTab{ID: soloLayoutPrefix + m.active, Root: &paneNode{Session: m.active}, Focus: m.active, Zoomed: page.Zoomed}, true
}
