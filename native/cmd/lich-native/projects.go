package main

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"

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
	if p := m.shownProject(); p != nil && !m.home {
		active = p.ID
	}
	for _, p := range m.shownOrder() {
		if p.ID == m.homeID {
			continue
		}
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
	if p := m.projectByID(m.homeID); p != nil {
		m.active = p.ActiveSessionID
	}
}

// onHome reports whether Home is on screen: chosen, or no project is open.
func (m *model) onHome() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.home || m.shownProject() == nil
}

// projectByID is the open project with id, nil when none is. Callers hold m.mu.
func (m *model) projectByID(id string) *lichclient.Project {
	if i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == id }); i >= 0 {
		return &m.projects[i]
	}
	return nil
}

// homeSessionLabel and homeNextSeq seed the home project's first shell the
// way newSession numbers one.
const (
	homeSessionLabel = "Shell 1"
	homeNextSeq      = 1
)

// ensureHome opens the home directory's project with a shell in it when the
// workspace does not hold it, as the web window does on every launch, and
// returns the workspace as it stands after.
func (m *model) ensureHome(ctx context.Context, ps []lichclient.Project) ([]lichclient.Project, error) {
	home, added, err := m.addHome(ctx, ps)
	if err != nil || !added {
		return ps, err
	}
	if err := m.client.AddSession(ctx, home.ID, strings.ToLower(rand.Text()), homeSessionLabel, "shell", "", homeNextSeq); err != nil {
		return nil, err
	}
	return m.client.LoadState(ctx)
}

// addHome opens the home directory's project, with no session, when ps does
// not hold it, and reports whether it did.
func (m *model) addHome(ctx context.Context, ps []lichclient.Project) (lichclient.Project, bool, error) {
	home, err := m.client.HomeProject(ctx)
	if err != nil {
		return home, false, err
	}
	m.mu.Lock()
	m.homeID = home.ID
	m.mu.Unlock()
	if slices.ContainsFunc(ps, func(p lichclient.Project) bool { return p.ID == home.ID }) {
		return home, false, nil
	}
	return home, true, m.client.AddProject(ctx, home.ID, home.Name, home.Path)
}

// openHome opens the home directory's project when the workspace lost it and
// puts Home on screen, as the web's ensureHomeProject does for a command with
// no project to run in. No session is seeded: the caller opens the one it
// needs.
func (m *model) openHome(ctx context.Context) error {
	ps, err := m.client.LoadState(ctx)
	if err != nil {
		return err
	}
	if _, _, err := m.addHome(ctx, ps); err != nil {
		return err
	}
	if err := m.reload(ctx); err != nil {
		return err
	}
	m.showHome()
	return nil
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

// shownOrder is the open projects as the strip draws them: the drag the
// backend has not confirmed yet, else the stored order. Callers hold m.mu.
func (m *model) shownOrder() []lichclient.Project {
	if m.pendingOrder == nil {
		return m.projects
	}
	return orderedBy(m.projects, m.pendingOrder)
}

// orderedBy is ps in the order of ids, a project ids does not name last.
func orderedBy(ps []lichclient.Project, ids []string) []lichclient.Project {
	out := slices.Clone(ps)
	rank := func(p lichclient.Project) int {
		if i := slices.Index(ids, p.ID); i >= 0 {
			return i
		}
		return len(ids)
	}
	slices.SortStableFunc(out, func(a, b lichclient.Project) int { return rank(a) - rank(b) })
	return out
}

// moveProject shows a tab dropped over another at once and returns the full
// order to store, nil when nothing moved. What it shows is a drag in flight,
// not the store's order: that changes when projects-reordered arrives.
func (m *model) moveProject(id, over string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	shown := m.shownOrder()
	ids := make([]string, len(shown))
	for i, p := range shown {
		ids[i] = p.ID
	}
	next := arrayMove(ids, id, over)
	if slices.Equal(next, ids) {
		return nil
	}
	m.pendingOrder = next
	return next
}

// storeOrder asks the backend to keep the order a drag showed. A refusal takes
// the drag back, so the strip returns to what is stored.
func (m *model) storeOrder(ctx context.Context, ids []string) error {
	err := m.client.ReorderProjects(ctx, ids)
	if err != nil {
		m.mu.Lock()
		if slices.Equal(m.pendingOrder, ids) {
			m.pendingOrder = nil
		}
		m.mu.Unlock()
	}
	return err
}

// applyOrder is projects-reordered: the order the backend stored, from this
// window's drag or from another client. It settles the drag it confirms.
func (m *model) applyOrder(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects = orderedBy(m.projects, ids)
	if slices.Equal(m.pendingOrder, ids) {
		m.pendingOrder = nil
	}
}

// errHomeIsPermanent refuses closing the home project: Home always has one.
var errHomeIsPermanent = errors.New("the home project cannot be closed")

// askClose has the backend close a project and answers the sessions it held,
// whose terminals the caller stops once the close went through. No tab moves
// here: it leaves the strip when project-closed comes back.
func (m *model) askClose(ctx context.Context, id string) ([]string, error) {
	m.mu.Lock()
	if id == m.homeID {
		m.mu.Unlock()
		return nil, errHomeIsPermanent
	}
	i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == id })
	var sessions []string
	if i >= 0 {
		sessions = sessionIDs(m.projects[i])
	}
	m.mu.Unlock()
	if err := m.client.CloseProject(ctx, id); err != nil {
		return nil, err
	}
	return sessions, nil
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
