package main

import (
	"context"
	"slices"

	"github.com/omartelo/lich/native/lichclient"
)

// homeScreen is the route last-screen.ts saves for Home.
const homeScreen = "/"

// screenPath is the route the web saves under lastScreenKey for a project's
// screen, Home for "".
func screenPath(projectID string) string {
	if projectID == "" {
		return homeScreen
	}
	return "/projects/" + projectID
}

// saveLastScreen is last-screen.ts saveLastScreen: the screen the next
// window opens on, a global setting.
func saveLastScreen(ctx context.Context, c *lichclient.Client, projectID string) error {
	return c.SetSetting(ctx, lastScreenKey, "", screenPath(projectID))
}

// projectTab is what one project tab draws.
type projectTab struct {
	ID, Name, Path string
	// status is the badge, statusNone on the active tab: its cards are on
	// screen saying the same, per session (ProjectTab.tsx).
	status string
}

// projectTabs is the strip: every open project in order with its badge, and
// the one on screen, "" on Home.
func (m *model) projectTabs() (tabs []projectTab, active string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p := m.shownProject(); p != nil {
		active = p.ID
	}
	for _, p := range m.projects {
		t := projectTab{ID: p.ID, Name: p.Name, Path: p.Path}
		if p.ID != active {
			t.status = m.status.projectStatus(sessionIDs(p))
		}
		tabs = append(tabs, t)
	}
	return tabs, active
}

func sessionIDs(p lichclient.Project) []string {
	ids := make([]string, len(p.Sessions))
	for i, s := range p.Sessions {
		ids[i] = s.ID
	}
	return ids
}

// showHome puts Home on screen. The active project is kept, so the sidebar
// and footer have nothing to show until a tab is chosen again.
func (m *model) showHome() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.home = true
}

// onHome reports whether Home is on screen: chosen, or no project is open.
func (m *model) onHome() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shownProject() == nil
}

// runningIn counts a project's sessions holding a turn, which closing it
// would kill (ProjectTabs.tsx requestClose).
func (m *model) runningIn(projectID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == projectID })
	if i < 0 {
		return 0
	}
	return len(m.status.running(sessionIDs(m.projects[i])))
}

// closedProject is what closing a project leaves to do: stop its sessions'
// terminals and, when it was on screen, show next ("" for Home).
type closedProject struct {
	sessions []string
	moved    bool
	next     string
}

// closeProject drops a project from the strip. Closing the one on screen
// moves to the tab before it, else the one after, else Home; any other
// leaves the screen alone (projects.tsx closeProject). ok is false when no
// open project has that id.
func (m *model) closeProject(id string) (closedProject, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == id })
	if i < 0 {
		return closedProject{}, false
	}
	shown := m.shownProject()
	out := closedProject{sessions: sessionIDs(m.projects[i]), moved: shown != nil && shown.ID == id}
	if out.moved {
		switch {
		case i > 0:
			out.next = m.projects[i-1].ID
		case i+1 < len(m.projects):
			out.next = m.projects[i+1].ID
		}
	}
	m.projects = slices.Delete(m.projects, i, i+1)
	return out, true
}

// moveProject applies a tab dropped over another and returns the full order
// to store, nil when nothing moved.
func (m *model) moveProject(id, over string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, len(m.projects))
	for i, p := range m.projects {
		ids[i] = p.ID
	}
	next := arrayMove(ids, id, over)
	if slices.Equal(next, ids) {
		return nil
	}
	slices.SortStableFunc(m.projects, func(a, b lichclient.Project) int {
		return slices.Index(next, a.ID) - slices.Index(next, b.ID)
	})
	return next
}

// notification is a row of the bell's queue (notifications.ts Notification).
type notification struct {
	pendingStatus
	projectID, projectName, sessionLabel string
}

// notifications joins the pending queue to the open projects, dropping a
// session that is gone (nowhere to route) and the focused one, whose own
// terminal already shows its state (notifications.ts notificationsFrom).
func (m *model) notifications() []notification {
	m.mu.Lock()
	defer m.mu.Unlock()
	focused := ""
	if p := m.shownProject(); p != nil {
		focused = m.active
	}
	var out []notification
	for _, item := range m.status.pending() {
		p, i := m.sessionAt(item.ID)
		if p == nil || item.ID == focused {
			continue
		}
		out = append(out, notification{pendingStatus: item, projectID: p.ID, projectName: p.Name, sessionLabel: p.Sessions[i].Label})
	}
	return out
}

// dismiss takes a session out of the bell's queue; dismissing a finished
// turn reads it.
func (m *model) dismiss(ctx context.Context, id string) {
	m.mu.Lock()
	read := m.status.dismiss(id)
	m.mu.Unlock()
	if read {
		m.markRead(ctx, id)
	}
}
