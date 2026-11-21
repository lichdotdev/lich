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
	// motherFolds are the mothers of the block folded over their children.
	motherFolds map[string]bool
	// ofLayout is a project's sessions in a layout's own sidebar: its rows
	// fold and file nothing and open no session, those being the project's.
	ofLayout bool
	// carried is the session whose card is in the air, "" for none.
	carried string
	// editors are the editor nodes the block hangs under its checkouts, by
	// checkout (hangEditors).
	editors map[string]editorNode
	// move is a checkout block or a folder coming in or leaving by the tree's
	// motion, and head its header's share of it this frame; nil at rest.
	move *blockMove
	head *rowMotion
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
	m.hangEditors(v.blocks)
	return v
}

// hangEditors hangs each checkout's editor node under one session of that
// checkout: the active one when it runs there, else the first drawn. Callers
// hold m.mu.
// shortcut: the editor still belongs to its checkout, so it follows whichever
// session runs there; give it the session that opened the file once the
// editor store keeps that.
func (m *model) hangEditors(blocks []blockView) {
	owner := map[string]string{}
	for _, b := range blocks {
		for _, sv := range b.sessions {
			if _, ok := owner[sv.path]; !ok || sv.ID == m.active {
				owner[sv.path] = sv.ID
			}
		}
	}
	for i := range blocks {
		b := &blocks[i]
		for _, sv := range b.sessions {
			n, ok := m.editorOf(sv.path)
			if !ok || owner[sv.path] != sv.ID {
				continue
			}
			if b.editors == nil {
				b.editors = map[string]editorNode{}
			}
			b.editors[sv.ID] = n
		}
	}
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
	slices.SortStableFunc(v.sessions, func(a, b sessionView) int {
		return phaseRank(phaseOf(a.status, a.unread)) - phaseRank(phaseOf(b.status, b.unread))
	})
	v.motherFolds = map[string]bool{}
	for _, sv := range v.sessions {
		if m.folds[foldKey(p.ID, motherKey(sv.ID))] {
			v.motherFolds[sv.ID] = true
		}
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
