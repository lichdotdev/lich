package main

import (
	"context"
	"errors"
	"slices"

	"github.com/omartelo/lich/native/lichclient"
)

// errNoProject is an action on the sidebar with no project on screen.
var errNoProject = errors.New("no project open")

// The sidebar's writes follow projects.tsx: change the model at once, then
// tell the store. A failed write is returned, not rolled back, as on the web;
// the sessions-filed event a folder write emits reloads the truth anyway.

// reorderSessions applies a full id order to the shown project and stores
// it. An order that no longer names the project's exact session set (a close
// raced the drag) is dropped whole, which is not an error.
func (m *model) reorderSessions(ctx context.Context, ids []string) error {
	m.mu.Lock()
	p := m.shownProject()
	if p == nil {
		m.mu.Unlock()
		return errNoProject
	}
	next, ok := applyOrder(p.Sessions, ids)
	if !ok {
		m.mu.Unlock()
		return nil
	}
	p.Sessions = next
	projectID := p.ID
	m.mu.Unlock()
	m.invalidate()
	return m.client.ReorderSessions(ctx, projectID, ids)
}

// commitDragOrder stores the blocks dragged into keys order. keys names
// every block but the pinned one, which never moves.
func (m *model) commitDragOrder(ctx context.Context, keys []string) error {
	stored, ok := m.storedSessions()
	if !ok {
		return errNoProject
	}
	return m.reorderSessions(ctx, dragOrder(stored, sessionBlocks(stored), keys))
}

// commitBlockOrder stores a drag inside one block: ids, the block's cards in
// their new order, fill the slots that block drew.
func (m *model) commitBlockOrder(ctx context.Context, blockKey string, ids []string) error {
	stored, ok := m.storedSessions()
	if !ok {
		return errNoProject
	}
	blocks := sessionBlocks(stored)
	i := slices.IndexFunc(blocks, func(b sessionBlock) bool { return b.key == blockKey })
	if i < 0 {
		return nil
	}
	return m.reorderSessions(ctx, blockOrder(stored, blocks[i], ids))
}

// fileSessions files sessions under folder, "" to take them out. Every id is
// written, as the web does: one the project does not hold fails nothing.
func (m *model) fileSessions(ctx context.Context, ids []string, folder string) error {
	if !m.refileShown(func(s []lichclient.Session) ([]lichclient.Session, bool) { return fileSessions(s, ids, folder) }) {
		return nil
	}
	for _, id := range ids {
		if err := m.client.SetSessionFolder(ctx, id, folder); err != nil {
			return err
		}
	}
	return nil
}

// renameFolder renames a folder of the shown project, its fold moving with
// it, and ungroups it for an empty to.
func (m *model) renameFolder(ctx context.Context, from, to string) error {
	p, ok := m.project()
	if !ok {
		return errNoProject
	}
	if !m.refileShown(func(s []lichclient.Session) ([]lichclient.Session, bool) { return renameFolder(s, from, to) }) {
		return nil
	}
	if err := m.moveFold(ctx, p.ID, folderKey(from), to); err != nil {
		return err
	}
	_, err := m.client.RenameFolder(ctx, p.ID, from, to)
	return err
}

// ungroupFolder takes every card out of a folder, which ends it.
func (m *model) ungroupFolder(ctx context.Context, name string) error {
	return m.renameFolder(ctx, name, "")
}

func (m *model) moveFold(ctx context.Context, projectID, fromKey, to string) error {
	toKey := ""
	if to != "" {
		toKey = folderKey(to)
	}
	collapsed, err := moveCollapsed(ctx, m.client, projectID, fromKey, toKey)
	m.mu.Lock()
	delete(m.folds, foldKey(projectID, fromKey))
	if toKey != "" && err == nil {
		m.folds[foldKey(projectID, toKey)] = collapsed
	}
	m.mu.Unlock()
	return err
}

// refileShown applies a folder reducer to the shown project's sessions,
// false when it changed nothing.
func (m *model) refileShown(refile func([]lichclient.Session) ([]lichclient.Session, bool)) bool {
	m.mu.Lock()
	p := m.shownProject()
	changed := false
	if p != nil {
		p.Sessions, changed = refile(p.Sessions)
	}
	m.mu.Unlock()
	if changed {
		m.invalidate()
	}
	return changed
}

func (m *model) storedSessions() ([]lichclient.Session, bool) {
	p, ok := m.project()
	return slices.Clone(p.Sessions), ok
}

// launchProviders are the providers a launch menu lists, in detection order.
func (m *model) launchProviders() []launchProvider { return m.providers.launchProviders() }

// newSessionKind is what an implicit new session spawns in the shown
// project (providers-store.ts projectNewSessionKind).
func (m *model) newSessionKind() string {
	m.mu.Lock()
	projectID := ""
	if p := m.shownProject(); p != nil {
		projectID = p.ID
	}
	m.mu.Unlock()
	return m.providers.implicitKind(projectID)
}
