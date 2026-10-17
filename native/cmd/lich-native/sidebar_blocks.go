package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
)

// sidebarFilter is the lens over the shown project's cards. Like the web's,
// it is never persisted: a window that boots showing two of nine sessions is
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
	folded    bool   // collapsed with a header drawn to undo it
	mark      string // markWait / markDone while collapsed, else markNone
}

// sidebarView is the shown project's sidebar under a filter.
type sidebarView struct {
	blocks []blockView
	// showHeader is whether blocks draw their headers: once more than one
	// block splits the list, while a filter makes the checkout the answer, or
	// when the lone block is a worktree, whose + is the only way to open
	// another session there.
	showHeader bool
	matched    bool
	notice     string // the empty filter's sentence, "" while something matched
	counts     map[sessionPhase]int
	folders    []string // every folder of the project, filter or not
}

// foldKey is a block's entry in m.folds: the project id is hex, so the
// separator cannot collide.
func foldKey(projectID, blockKey string) string { return projectID + ":" + blockKey }

// sidebarBlocks is SessionSidebar.tsx's derivation for the shown project:
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
		showHeader: len(blocks) > 1 || f.narrows() || isLoneWorktree(blocks),
		matched:    matched,
		counts:     phaseCounts(p.Sessions, f.query, p.Path, m.phaseOfSession),
		folders:    foldersOf(p.Sessions),
	}
	if !matched {
		v.notice = noMatchNotice(f.query, f.phases)
	}
	pending := m.status.pending()
	now := time.Now()
	for _, b := range blocks {
		v.blocks = append(v.blocks, m.dressBlock(p, b, v.showHeader, pending, now))
	}
	return v
}

func isLoneWorktree(blocks []sessionBlock) bool {
	return len(blocks) == 1 && blocks[0].kind == blockCheckout && blocks[0].path != ""
}

// dressBlock is SessionGroup.tsx's per-block reading. Callers hold m.mu.
func (m *model) dressBlock(p lichclient.Project, b sessionBlock, header bool, pending []pendingStatus, now time.Time) blockView {
	collapsed := m.folds[foldKey(p.ID, b.key)]
	v := blockView{key: b.key, kind: b.kind, folder: b.folder, path: b.path, collapsed: collapsed, folded: header && collapsed}
	ids := make([]string, 0, len(b.sessions))
	for _, s := range b.sessions {
		ids = append(ids, s.ID)
		v.sessions = append(v.sessions, m.viewOf(p, s, now))
	}
	if collapsed {
		v.mark = collapsedMark(pending, ids)
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

// phaseOfSession is useSessionPhases' lookup. Callers hold m.mu.
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
	for _, b := range sessionBlocks(shown.Sessions) {
		if _, ok := m.folds[foldKey(projectID, b.key)]; !ok {
			unread = append(unread, b.key)
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
