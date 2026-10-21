package main

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
)

// gitPoll is how often each shown path's branch and changes are re-read. The
// backend pushes no git events.
const gitPoll = 3 * time.Second

// lastScreenKey is the global setting the last route is saved under, the one
// record of which project the user was on.
const lastScreenKey = "window.lastScreen"

var projectScreen = regexp.MustCompile(`^/projects/([^/]+)(?:/.*)?$`)

// model is the backend state the window draws, fed by LoadState, the /events
// stream and a git poll. Every method is safe from any goroutine.
type model struct {
	client     *lichclient.Client
	invalidate func()

	mu       sync.Mutex
	projects []lichclient.Project // open projects, in the user's order
	// pendingOrder is a tab drag the backend has not confirmed: the strip
	// shows it, the store's order does not hold it. projects-reordered
	// settles it, a refused reorder drops it.
	pendingOrder  []string
	status        *statusStore
	usage         *footerUsage
	quota         *planQuota
	cwd           map[string]sessionCwd             // session id -> last session-cwd report
	agent         map[string]string                 // session id -> provider CLI live in its PTY
	sandbox       map[string]sandboxEvent           // session id -> how its spawn was confined
	relay         map[string]sessionRelay           // session id -> request open with another
	git           map[string]lichclient.DiffStats   // shown path -> its branch and changes
	base          map[string]*lichclient.BaseStatus // shown path -> where it stands against its base
	prs           map[string]prLookup               // shown path -> its open pull request
	activeProject string                            // project id on screen
	home          bool                              // Home is on screen instead
	// homeID is the project rooted at the home directory, which Home shows:
	// pinned out of the strip and never closed.
	homeID    string
	active    string          // session id on screen
	lost      bool            // /events ended under an open window
	focused   bool            // the window has the user's focus
	restored  bool            // unread marks taken from the first LoadState
	launched  bool            // the first screen was chosen from the last one
	folds     map[string]bool // foldKey -> block folded, once read or written
	providers *providerBook   // detected providers, their flags and defaults
	// profiles is who is logged in to each Claude Code profile, and
	// codexProfiles who is logged in to each Codex one and ompProfiles each
	// oh-my-pi one.
	profiles, codexProfiles, ompProfiles *profileBook
	// xdgProfiles is each link-home provider's, by kind (xdgProfileProviders).
	xdgProfiles map[string]*profileBook
	// notify hands each session-status report to the desktop channel.
	notify func(statusReport)
	// seen hears a session the window just showed the user.
	seen func(id string)
	// loginProgress takes each step of a profile login, of the provider kind
	// names.
	loginProgress func(kind string, step lichclient.ProfileLogin)
	// diffTouched hears the checkout of a session that likely changed files,
	// for the Review tab's diff, which is read apart from the git store.
	diffTouched func(checkout string)
	// focusSession brings a session's card up, as `lich focus` asks.
	focusSession func(projectID, id string)
	// raise brings the window forward, as the tray asks.
	raise func()
	// leave closes the window.
	leave func()
	// projectClosed hears a project that left the strip, whoever closed it,
	// once its tab is already gone: the window stops the terminals and moves
	// the screen.
	projectClosed func(closedProject)
	// pluginProfiles hears a pass that put Personal's lich plugin into the
	// other profiles of a provider.
	pluginProfiles func(lichclient.PluginProfiles)
	// prChanged hears a pull request write land (PRChangedEvent).
	prChanged func()
	// themeSelection hears the stored theme selection change
	// (ThemeSelectionEvent).
	themeSelection func(lichclient.ThemeSelection)
	// exited hears a session's process end.
	exited func(id string, e sessionExit)
	// duplicate hears a session report a conversation another open card
	// runs (duplicateEventName).
	duplicate func(sessionDuplicate)
	// pluginIncompatible hears a session report from a lich plugin outside
	// the range this lich speaks (PluginIncompatibleEvent).
	pluginIncompatible func()
	// appUpdateProgress hears a step of an update being applied.
	appUpdateProgress func(lichclient.AppUpdateProgress)
	// dropped hears /events end under an open window; nil marks the model
	// lost there and then.
	dropped func()
	// closed is the closed projects, which the menu and the palette read.
	closed *closedProjects

	// layouts are the window's split layouts; shownLayout is the one on
	// screen, "" for a project's session alone.
	layouts     layoutTabs
	shownLayout string
}

// sessionCwd is a session-cwd payload: where the session's shell is, or the
// host (tmux, ssh, ...) it went into that the backend cannot see through.
type sessionCwd struct {
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
	Host string `json:"host"`
}

func newModel(client *lichclient.Client, invalidate func()) *model {
	quota := newPlanQuota(quotaReader(client), invalidate)
	return &model{client: client, invalidate: invalidate, status: newStatusStore(), usage: newFooterUsage(), quota: quota, cwd: map[string]sessionCwd{},
		agent: map[string]string{}, sandbox: map[string]sandboxEvent{}, relay: map[string]sessionRelay{}, git: map[string]lichclient.DiffStats{},
		base: map[string]*lichclient.BaseStatus{}, prs: map[string]prLookup{}, folds: map[string]bool{},
		profiles:      newProfileBook(claudeProfileProvider, client, invalidate, quota),
		codexProfiles: newProfileBook(codexProfileProvider, client, invalidate, quota),
		ompProfiles:   newProfileBook(ompProfileProvider, client, invalidate, quota),
		xdgProfiles:   xdgProfileBooks(client, invalidate, quota),
		providers:     newProviderBook(client, invalidate),
		closed:        newClosedProjects(client, invalidate)}
}

func (m *model) reload(ctx context.Context) error {
	ps, err := m.client.LoadState(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	first := !m.launched
	m.mu.Unlock()
	if first {
		if ps, err = m.ensureHome(ctx, ps); err != nil {
			return err
		}
	}
	choose := first && len(ps) > 0
	launch := ""
	if choose {
		if launch, err = m.launchProject(ctx, ps); err != nil {
			return err
		}
	}
	normalizeCheckouts(ps)
	m.mu.Lock()
	m.projects = ps
	fresh := m.providers.noteProjects(ps)
	if !m.restored {
		m.status.restoreUnread(unreadSessions(ps))
		m.restored = true
	}
	if choose {
		m.launched = true
		m.activeProject, m.home = launch, launch == "" || launch == m.homeID
	}
	if p := m.shownProject(); p != nil && !slices.ContainsFunc(p.Sessions, func(s lichclient.Session) bool { return s.ID == m.active }) {
		m.active = p.ActiveSessionID
	}
	pruned := m.layouts.prune(m.liveLocked)
	if pruned {
		m.refocusLayoutLocked()
	}
	m.mu.Unlock()
	if pruned {
		m.saveLayouts()
	}
	if len(fresh) > 0 {
		go m.providers.readProfiles(ctx, fresh)
	}
	m.invalidate()
	return nil
}

// launchProject is the project the window opens on: the one the last screen
// was on while it is still open, else "" for Home, where anything else lands
// too, Home and a closed project included.
func (m *model) launchProject(ctx context.Context, ps []lichclient.Project) (string, error) {
	saved, err := m.client.GetSetting(ctx, lastScreenKey, "")
	if err != nil {
		return "", err
	}
	if match := projectScreen.FindStringSubmatch(saved); match != nil {
		if slices.ContainsFunc(ps, func(p lichclient.Project) bool { return p.ID == match[1] }) {
			return match[1], nil
		}
	}
	return "", nil
}

// shownProject is the active project, else the first when the active one is
// not open; nil on Home. Callers hold m.mu.
func (m *model) shownProject() *lichclient.Project {
	if m.home {
		return m.projectByID(m.homeID)
	}
	if i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == m.activeProject }); i >= 0 {
		return &m.projects[i]
	}
	if len(m.projects) == 0 {
		return nil
	}
	return &m.projects[0]
}

// setActiveProject puts a project on screen with the session it last showed,
// and reports false when no open project has that id.
func (m *model) setActiveProject(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == id })
	if i < 0 {
		return false
	}
	m.activeProject, m.home = id, id == m.homeID
	m.active = m.projects[i].ActiveSessionID
	m.shownLayout = ""
	return true
}

// hydrateTurnStates seeds the status, usage and cwd stores from the backend's
// snapshot of what each live session last reported. /events replays nothing,
// so without it a card is blank until its next hook. It runs after /events
// attached and before follow starts: whatever the stream delivered meanwhile
// is queued behind it and lands on top, and a store that already heard a
// session ignores the snapshot for it.
func (m *model) hydrateTurnStates(ctx context.Context) error {
	states, err := m.client.TurnStates(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.status.hydrate(states)
	for _, ts := range states {
		if ts.Cwd != "" || ts.Host != "" {
			m.cwd[ts.ID] = sessionCwd{ID: ts.ID, Cwd: ts.Cwd, Host: ts.Host}
		}
	}
	m.mu.Unlock()
	for _, ts := range states {
		if len(ts.Usage) == 0 {
			continue
		}
		if err := m.usage.seed(ts.Usage); err != nil {
			log.Printf("terminal.TurnStates: usage of %s: %v", ts.ID, err)
		}
	}
	m.invalidate()
	return nil
}

// follow applies /events until ctx ends. The backend replays nothing beyond
// what hydrateTurnStates read. When the stream ends first, the backend dropped this window: /events
// serves one client and a newer one replaces it, or lich quit or restarted.
// dropped tells those apart.
func (m *model) follow(ctx context.Context, events <-chan lichclient.Event) {
	for ev := range events {
		if id, exited := strings.CutPrefix(ev.Name, exitEventPrefix); exited {
			m.mu.Lock()
			m.status.exited(id)
			m.mu.Unlock()
			if m.exited != nil {
				m.exited(id, readSessionExit(ev.Data))
			}
			m.invalidate()
			continue
		}
		// The usage store also clears on session-agent and session-status,
		// which the cases below still take.
		if _, err := m.usage.apply(ev); err != nil {
			log.Printf("%s: %v", ev.Name, err)
		}
		m.quota.apply(ev)
		switch ev.Name {
		case "session-usage":
			// Taken by the usage store above; only the redraw is left.
		case statusEventName:
			if err := m.applyStatus(ctx, ev); err != nil {
				log.Printf("%s: %v", ev.Name, err)
			}
		case "session-cwd":
			var c sessionCwd
			if json.Unmarshal(ev.Data, &c) == nil {
				m.mu.Lock()
				m.cwd[c.ID] = c
				m.mu.Unlock()
			}
		case "session-title":
			var s struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			}
			if json.Unmarshal(ev.Data, &s) == nil {
				m.rename(s.ID, s.Label)
			}
		case lichclient.SessionConversationEvent:
			var c lichclient.ConversationReported
			if json.Unmarshal(ev.Data, &c) == nil {
				m.setConversation(c.ID, c.ProviderSessionID)
			}
		case lichclient.SessionProfileEvent:
			var moved lichclient.ProfileMoved
			if json.Unmarshal(ev.Data, &moved) == nil {
				m.setProfile(moved.ID, moved.Profile)
			}
		case lichclient.ClaudeProfileLoginEvent, lichclient.CodexProfileLoginEvent:
			var step lichclient.ProfileLogin
			if json.Unmarshal(ev.Data, &step) == nil {
				m.loginStep(ctx, loginEventKind(ev.Name), step)
			}
		case lichclient.PluginProfilesEvent:
			var pass lichclient.PluginProfiles
			if json.Unmarshal(ev.Data, &pass) == nil && m.pluginProfiles != nil {
				m.pluginProfiles(pass)
			}
		case lichclient.ThemeSelectionEvent:
			var sel lichclient.ThemeSelection
			if json.Unmarshal(ev.Data, &sel) == nil && m.themeSelection != nil {
				m.themeSelection(sel)
			}
		case lichclient.PRChangedEvent:
			if m.prChanged != nil {
				m.prChanged()
			}
		case lichclient.AppUpdateProgressEvent:
			var p lichclient.AppUpdateProgress
			if json.Unmarshal(ev.Data, &p) == nil && m.appUpdateProgress != nil {
				m.appUpdateProgress(p)
			}
		case duplicateEventName:
			var d sessionDuplicate
			if json.Unmarshal(ev.Data, &d) == nil && m.duplicate != nil {
				m.duplicate(d)
			}
		case lichclient.PluginIncompatibleEvent:
			if m.pluginIncompatible != nil {
				m.pluginIncompatible()
			}
		case touchedEventName:
			m.sessionTouched(ctx, ev.Data)
		case focusEventName:
			m.sessionFocus(ev.Data)
		case raiseEventName:
			if m.raise != nil {
				m.raise()
			}
		case closeEventName:
			log.Print("lich asked this window to close")
			if m.leave != nil {
				m.leave()
			}
		case "project-closed":
			var gone struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(ev.Data, &gone) == nil {
				m.closed.stale()
				if out, ok := m.closeProject(gone.ID); ok && m.projectClosed != nil {
					m.projectClosed(out)
				}
			}
		case "projects-reordered":
			var order struct {
				IDs []string `json:"ids"`
			}
			if json.Unmarshal(ev.Data, &order) == nil {
				m.applyOrder(order.IDs)
			}
		case "project-added", "project-opened":
			m.closed.stale()
			if err := m.reload(ctx); err != nil {
				log.Printf("reload after %s: %v", ev.Name, err)
			} else if err := m.loadFolds(ctx); err != nil {
				log.Printf("load folds after %s: %v", ev.Name, err)
			}
		case "session-opened", "session-closed", "sessions-filed", "sessions-colored":
			if err := m.reload(ctx); err != nil {
				log.Printf("reload after %s: %v", ev.Name, err)
			} else if err := m.loadFolds(ctx); err != nil {
				log.Printf("load folds after %s: %v", ev.Name, err)
			}
		default:
			live, err := m.applyLive(ev)
			if err != nil {
				log.Printf("%s: %v", ev.Name, err)
			}
			if !live {
				continue
			}
		}
		m.invalidate()
	}
	if ctx.Err() != nil {
		return
	}
	if m.dropped != nil {
		m.dropped()
		return
	}
	m.markLost()
}

// markLost gives the window up: the backend serves another one, or is gone.
func (m *model) markLost() {
	m.mu.Lock()
	m.lost = true
	m.mu.Unlock()
	m.invalidate()
}

// connectionLost reports whether the backend dropped this window.
func (m *model) connectionLost() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lost
}

func (m *model) rename(id, label string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for pi := range m.projects {
		for si := range m.projects[pi].Sessions {
			if m.projects[pi].Sessions[si].ID == id {
				m.projects[pi].Sessions[si].Label = label
			}
		}
	}
}

func (m *model) pollGit(ctx context.Context) {
	tick := time.NewTicker(gitPoll)
	defer tick.Stop()
	for {
		m.refreshGit(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// refreshGit re-reads every shown path once. A path that is not a git
// checkout fails and keeps no entry, so its card shows no branch.
func (m *model) refreshGit(ctx context.Context) {
	for _, path := range m.shownPaths() {
		m.refreshPath(ctx, path)
	}
	m.forgetUntracked()
}

func (m *model) shownPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.shownProject()
	if p == nil {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for _, s := range p.Sessions {
		add(m.shownPath(*p, s))
	}
	// The footer reads the project's own checkout while no card is active.
	if !slices.ContainsFunc(p.Sessions, func(s lichclient.Session) bool { return s.ID == m.active }) {
		add(p.Path)
	}
	return paths
}

// shownPath is where the card says a session is: its live cwd, else its
// checkout. Callers hold m.mu.
func (m *model) shownPath(p lichclient.Project, s lichclient.Session) string {
	if c := m.cwd[s.ID]; c.Cwd != "" {
		return c.Cwd
	}
	return sessionPath(p, s)
}

func sessionPath(p lichclient.Project, s lichclient.Session) string {
	if s.Path != "" {
		return s.Path
	}
	return p.Path
}

// sessionView is a session with what the card shows about it. path is the
// checkout it was spawned in, which groups it; shown is where its shell is now,
// and git describes shown.
type sessionView struct {
	lichclient.Session
	projectID string
	path      string
	shown     string
	host      string
	status    string
	unread    bool          // a done turn nobody has read
	age       time.Duration // how long status has lasted, while aged
	aged      bool          // status has a clock (busy, compacting, waiting)
	agent     string        // provider CLI live in the PTY, "" for none
	sandbox   sandboxEvent
	relay     sessionRelay // Direction "" for none
	tool      sessionTool  // Name "" outside a tool call
	reason    string       // what a waiting session is blocked on, "" for unsaid
	git       lichclient.DiffStats
	base      *lichclient.BaseStatus  // nil without a base to stand against
	pr        *lichclient.PullRequest // open PR of the shown path's branch, nil for none
	// profileName is the profile the session runs in, "" for Personal;
	// continueIn are the other logins its conversation could move to.
	profileName string
	continueIn  []continueTarget
	// layout is the name of the layout the session sits in, "" for none.
	layout string
	// running is the model the provider's transcript reports, "" for a
	// provider that reports none.
	running string
}

// viewOf is what s's card shows at now. Callers hold m.mu.
func (m *model) viewOf(p lichclient.Project, s lichclient.Session, now time.Time) sessionView {
	shown := m.shownPath(p, s)
	age, aged := m.status.age(s.ID, now)
	tool, _ := m.status.tool(s.ID)
	usage, _ := m.usage.get(s.ID)
	return sessionView{
		Session: s, projectID: p.ID, path: sessionPath(p, s), shown: shown, host: m.cwd[s.ID].Host,
		status: m.status.status(s.ID), unread: m.status.unread(s.ID), age: age, aged: aged,
		agent: m.agent[s.ID], sandbox: m.sandbox[s.ID], git: m.git[shown],
		base: m.base[shown], pr: m.prs[shown].pr,
		relay: m.relay[s.ID], tool: tool, reason: m.status.reason(s.ID),
		profileName: m.profileNameOf(s.Kind, s.Profile), continueIn: m.continueTargets(s.Kind, s.Profile),
		layout: m.layoutNameOfLocked(s.ID), running: usage.Model,
	}
}

// gitStatus is path's last branch and changes, false while git has not
// answered for it or it is not a checkout.
func (m *model) gitStatus(path string) (lichclient.DiffStats, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.git[path]
	return g, ok
}

// setConversation records the provider conversation session id runs, which
// its exit offers back.
func (m *model) setConversation(id, providerSessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, i := m.sessionAt(id); p != nil {
		p.Sessions[i].ProviderSessionID = providerSessionID
	}
}

// sessionViewOf is what session id's card shows now, false once it left
// the open sessions.
func (m *model) sessionViewOf(id string) (sessionView, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil {
		return sessionView{}, false
	}
	return m.viewOf(*p, p.Sessions[i], time.Now()), true
}

func (m *model) activeSession() (sessionView, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.shownProject()
	if p == nil {
		return sessionView{}, false
	}
	i := slices.IndexFunc(p.Sessions, func(s lichclient.Session) bool { return s.ID == m.active })
	if i < 0 {
		return sessionView{}, false
	}
	return m.viewOf(*p, p.Sessions[i], time.Now()), true
}

// setActive puts a session of the active project on screen, and records it
// as the session that project shows when it is put back on screen.
//
// Leaving a card reads its turn, so one that finished while it was on screen
// does not stay news; arriving reads the new one while the window has focus.
func (m *model) setActive(ctx context.Context, id string) {
	m.mu.Lock()
	refocused := m.routeLocked(id)
	left := m.active
	m.active = id
	if p := m.shownProject(); p != nil {
		p.ActiveSessionID = id
	}
	readLeft := left != id && m.status.markSeen(left)
	readNew := m.focused && m.status.markSeen(id)
	m.mu.Unlock()
	if refocused {
		m.saveLayouts()
	}
	if readLeft {
		m.markRead(ctx, left)
	}
	if readNew {
		m.markRead(ctx, id)
	}
}

// project returns the active project.
func (m *model) project() (lichclient.Project, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.shownProject()
	if p == nil {
		return lichclient.Project{}, false
	}
	return *p, true
}

// closeSession drops a session from its project and, when it was on screen,
// puts its neighbor there: the session that fills its slot, else the one before
// it. It returns the project and the session now active, and false when no open
// session has that id.
func (m *model) closeSession(id string) (projectID, active string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil {
		return "", "", false
	}
	p.Sessions = slices.Delete(p.Sessions, i, i+1)
	if left, took := m.layouts.take(id); took {
		m.saveLayoutsLocked()
		if m.active == id && m.refocusLayoutLocked() {
			return p.ID, m.active, true
		}
		if lp, _ := m.sessionAt(left); m.active == id && lp != nil {
			m.active, m.activeProject = left, lp.ID
			return lp.ID, left, true
		}
	}
	if m.active == id {
		m.active = ""
		switch {
		case i < len(p.Sessions):
			m.active = p.Sessions[i].ID
		case i > 0:
			m.active = p.Sessions[i-1].ID
		}
		p.ActiveSessionID = m.active
	}
	return p.ID, m.active, true
}

// sessionAt finds the project holding a session and its slot there. Callers
// hold m.mu.
func (m *model) sessionAt(id string) (*lichclient.Project, int) {
	for pi := range m.projects {
		if i := slices.IndexFunc(m.projects[pi].Sessions, func(s lichclient.Session) bool { return s.ID == id }); i >= 0 {
			return &m.projects[pi], i
		}
	}
	return nil, -1
}

// checkoutLabel titles a group with the project folder for the root checkout
// (an empty path), else with the name the worktree was made with, read past
// <worktrees-root>/<projectID>/ because a branch name may hold a slash. A
// checkout outside that root keeps its last segment.
func checkoutLabel(path, projectPath, projectID string) string {
	if path == "" {
		return baseName(projectPath)
	}
	// git spells a Windows checkout with forward slashes, lich with backslashes.
	unix := strings.ReplaceAll(path, `\`, "/")
	root := "/" + projectID + "/"
	// The first match: a worktree may itself be named after the project id.
	at := strings.Index(unix, root)
	if at < 0 {
		return baseName(path)
	}
	return unix[at+len(root):]
}

// baseName is the last segment under either separator, ignoring a trailing
// one.
func baseName(path string) string {
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}

// unreadSessions is every session the workspace database says came back with
// a finished turn nobody read.
func unreadSessions(ps []lichclient.Project) []string {
	var ids []string
	for _, p := range ps {
		for _, s := range p.Sessions {
			if s.Unread {
				ids = append(ids, s.ID)
			}
		}
	}
	return ids
}

// checkoutPath is the path a session stores; one in the project's own
// directory stores none of its own.
func checkoutPath(dir, projectPath string) string {
	if dir == projectPath {
		return ""
	}
	return dir
}

// normalizeCheckouts stores a session in the project's own directory with no
// path, as checkoutPath does at creation. Native builds before that fix
// wrote the project's path instead, which split the root block in two.
func normalizeCheckouts(ps []lichclient.Project) {
	for pi := range ps {
		for si := range ps[pi].Sessions {
			s := &ps[pi].Sessions[si]
			s.Path = checkoutPath(s.Path, ps[pi].Path)
		}
	}
}
