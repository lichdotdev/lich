package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// sidebarFilter is the lens over the shown project's cards. It is never
// persisted: a window that boots showing two of nine sessions is
// a bug report, not a restored setting.
type sidebarFilter struct {
	query  string
	phases map[sessionPhase]bool
}

func (f sidebarFilter) narrows() bool {
	return strings.TrimSpace(f.query) != "" || len(f.phases) > 0
}

// blockView is one sidebar block as the UI draws it.
type blockView struct {
	key    string
	kind   blockKind
	title  string // folder name, "Pinned", or the checkout's label
	folder string
	path   string // checkout, "" for the project root and gathered blocks
	color  string // a folder's shared card color, "" for none
	// checkouts is where a folder's + opens a session, titled like their
	// blocks; nil for every other kind.
	checkouts []launchCheckout
	sessions  []sessionView
	collapsed bool   // the stored fold, what the chevron shows
	mark      string // markWait / markDone while collapsed, else markNone
	// under is the checkout a checkout block draws as the row its sessions
	// hang from, "" for a block that is no checkout's.
	under string
	// byCheckout is a folder or the pinned block: it hangs its sessions from
	// the checkouts they run in (blockRows).
	byCheckout bool
	// motherFolds are the mothers of the block folded over their children.
	motherFolds map[string]bool
	// ofLayout is a project's sessions in a layout's own sidebar: its rows
	// fold and file nothing and open no session, those being the project's.
	ofLayout bool
	// carried is the session whose card is in the air, "" for none.
	carried string
	// checkouts' facts, keyed by path: the header's and every checkout a
	// child runs in apart from its mother.
	facts map[string]checkoutFacts
	// move is a checkout block or a folder coming in or leaving by the tree's
	// motion, and head its header's share of it this frame; nil at rest.
	move *blockMove
	head *rowMotion
}

// checkoutFacts is what a checkout row shows and its tooltip spells out.
type checkoutFacts struct {
	label string
	git   backend.DiffStats
	base  *backend.BaseStatus
	pr    *backend.PullRequest
}

// sidebarView is the shown project's sidebar under a filter.
type sidebarView struct {
	project string // the shown project's id
	blocks  []blockView
	matched bool
	notice  string // the empty filter's sentence, "" while something matched
	counts  map[sessionPhase]int
	folders []string // every folder of the project, filter or not
}

// foldKey is a block's entry in m.folds: the project id is hex, so the
// separator cannot collide.
func foldKey(projectID, blockKey string) string { return projectID + ":" + blockKey }

// sidebarBlocks derives the shown project's blocks:
// filter the stored list, group the survivors, and dress each block.
func (m *model) sidebarBlocks(f sidebarFilter) sidebarView {
	m.mu.Lock()
	defer m.mu.Unlock()
	shown := m.shownProject()
	if shown == nil {
		return sidebarView{}
	}
	p := *shown
	visible, matched := filterSessions(p.Sessions, f.query, p.Path, m.active, f.phases, m.phaseOfSession)
	blocks := sessionBlocks(visible)
	v := sidebarView{
		project: p.ID,
		matched: matched,
		counts:  phaseCounts(p.Sessions, f.query, p.Path, m.phaseOfSession),
		folders: foldersOf(p.Sessions),
	}
	if !matched {
		v.notice = noMatchNotice(f.query, f.phases)
	}
	pending := m.status.pending()
	now := time.Now()
	for _, b := range blocks {
		v.blocks = append(v.blocks, m.dressBlock(p, b, pending, now))
	}
	return v
}

// dressBlock reads one block into its view. Callers hold m.mu.
func (m *model) dressBlock(p backend.Project, b sessionBlock, pending []pendingStatus, now time.Time) blockView {
	collapsed := m.folds[foldKey(p.ID, b.key)]
	v := blockView{key: b.key, kind: b.kind, folder: b.folder, path: b.path, collapsed: collapsed}
	ids := make([]string, 0, len(b.sessions))
	for _, s := range b.sessions {
		ids = append(ids, s.ID)
		v.sessions = append(v.sessions, m.viewOf(p, s, now))
	}
	if collapsed {
		v.mark = collapsedMark(pending, ids)
	}
	v.motherFolds = map[string]bool{}
	v.facts = map[string]checkoutFacts{}
	for _, sv := range v.sessions {
		if m.folds[foldKey(p.ID, motherKey(sv.ID))] {
			v.motherFolds[sv.ID] = true
		}
		v.facts[sv.path] = m.checkoutFactsOf(p, sv.path)
	}
	v.byCheckout = b.kind != blockCheckout
	if b.kind == blockCheckout {
		v.under = sessionPath(p, backend.Session{Path: b.path})
		v.facts[v.under] = m.checkoutFactsOf(p, v.under)
	}
	switch b.kind {
	case blockPinned:
		v.title = i18n.T("sidebar.sessionGroup.pinned")
	case blockFolder:
		v.title, v.color = b.folder, sharedColor(b.sessions)
		for _, path := range checkoutsOf(b.sessions) {
			v.checkouts = append(v.checkouts, launchCheckout{path: path, label: checkoutLabel(path, p.Path, p.ID)})
		}
	default:
		v.title = checkoutLabel(b.path, p.Path, p.ID)
	}
	return v
}

// checkoutFactsOf reads what the checkout at path shows. Callers hold m.mu.
func (m *model) checkoutFactsOf(p backend.Project, path string) checkoutFacts {
	f := checkoutFacts{label: checkoutLabel(path, p.Path, p.ID), git: m.git[path], base: m.base[path], pr: m.prs[path].pr}
	if path == p.Path {
		f.label = checkoutLabel("", p.Path, p.ID)
	}
	if f.git.Branch != "" {
		f.label = f.git.Branch
	}
	return f
}

// phaseOfSession looks up a session's phase. Callers hold m.mu.
func (m *model) phaseOfSession(id string) sessionPhase {
	return phaseOf(m.status.status(id), m.status.unread(id))
}

// loadFolds reads the stored fold of every block of the shown project not
// read yet. The UI calls it off the frame, after a reload or a project switch.
func (m *model) loadFolds(ctx context.Context) error {
	m.mu.Lock()
	shown := m.shownProject()
	if shown == nil {
		m.mu.Unlock()
		return nil
	}
	projectID := shown.ID
	var unread []string
	keys := make([]string, 0, len(shown.Sessions))
	for _, b := range sessionBlocks(shown.Sessions) {
		keys = append(keys, b.key)
	}
	for _, s := range shown.Sessions {
		if s.OriginSessionID != "" {
			keys = append(keys, motherKey(s.OriginSessionID))
		}
	}
	for _, key := range keys {
		if _, ok := m.folds[foldKey(projectID, key)]; !ok && !slices.Contains(unread, key) {
			unread = append(unread, key)
		}
	}
	m.mu.Unlock()
	for _, key := range unread {
		collapsed, err := loadCollapsed(ctx, m.client, projectID, key)
		if err != nil {
			return err
		}
		m.mu.Lock()
		m.folds[foldKey(projectID, key)] = collapsed
		m.mu.Unlock()
	}
	return nil
}

// toggleFold folds or unfolds a block of the shown project and stores it.
func (m *model) toggleFold(ctx context.Context, blockKey string) error {
	m.mu.Lock()
	shown := m.shownProject()
	if shown == nil {
		m.mu.Unlock()
		return errors.New("toggle fold: no project open")
	}
	projectID := shown.ID
	key := foldKey(projectID, blockKey)
	m.folds[key] = !m.folds[key]
	collapsed := m.folds[key]
	m.mu.Unlock()
	return saveCollapsed(ctx, m.client, projectID, blockKey, collapsed)
}
