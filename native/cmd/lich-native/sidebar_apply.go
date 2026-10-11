package main

import (
	"context"
	"errors"
)

// writeSidebar carries out a frame's block-level writes, off the frame. Each
// is attempted whatever the others did; their failures come back joined.
func writeSidebar(ctx context.Context, m *model, w sidebarWrites, l sidebarLayout) error {
	var errs []error
	if w.fold != "" {
		errs = append(errs, m.toggleFold(ctx, w.fold))
	}
	if w.file.ids != nil {
		errs = append(errs, m.fileSessions(ctx, w.file.ids, w.file.folder))
	}
	if w.paint.ids != nil {
		errs = append(errs, m.paintSessions(ctx, w.paint.ids, w.paint.color))
	}
	if w.folder.from != "" {
		errs = append(errs, m.renameFolder(ctx, w.folder.from, w.folder.to))
	}
	if w.blockOrder != nil {
		errs = append(errs, m.commitDragOrder(ctx, w.blockOrder))
	}
	if w.cardOrder.key != "" {
		errs = append(errs, m.commitBlockOrder(ctx, w.cardOrder.key, w.cardOrder.ids))
	}
	if w.saveWidth {
		errs = append(errs, saveSidebarWidth(ctx, m.client, l.widthRem))
	}
	if w.saveOpen {
		errs = append(errs, saveSidebarOpen(ctx, m.client, l.open))
	}
	m.invalidate()
	return errors.Join(errs...)
}

// paintSessions tints every session of ids at once, a folder's Color.
func (m *model) paintSessions(ctx context.Context, ids []string, tint string) error {
	for _, id := range ids {
		m.setColor(id, tint)
	}
	m.invalidate()
	for _, id := range ids {
		if err := m.client.SetSessionColor(ctx, id, tint); err != nil {
			return err
		}
	}
	return nil
}

// projectFolders is every folder of the shown project, filter or not.
func (m *model) projectFolders() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p := m.shownProject(); p != nil {
		return foldersOf(p.Sessions)
	}
	return nil
}
