package main

import (
	"crypto/rand"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// forkSource reads the open session id as a fork's source, with a copy of
// its project.
func (m *model) forkSource(id string) (lichclient.Project, forkSource, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil {
		return lichclient.Project{}, forkSource{}, false
	}
	return *p, forkSourceOf(p.Sessions[i]), true
}

// forkHere is Fork ▸ Here: a new card in the session's own checkout, under
// it, branching its conversation. Run off the frame.
func (u *window) forkHere(id string) {
	p, src, ok := u.model.forkSource(id)
	if !ok {
		return
	}
	if err := u.conversationLeft(p, src); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: err.Error()})
		return
	}
	if _, err := u.addForkSession(p, src, src.path); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.fork.failed", i18n.Args{"name": src.label, "error": lichclient.ErrorText(err)})})
	}
}

// forkPruned is a fork refused because the provider no longer has the
// conversation; its text is the sentence the window shows.
type forkPruned struct{ text string }

func (e forkPruned) Error() string { return e.text }

// conversationLeft asks the provider for src's conversation before a fork
// is made of it: one it pruned would die in the new card with the
// provider's own error, after the card and any worktree already exist.
func (u *window) conversationLeft(p lichclient.Project, src forkSource) error {
	cwd := src.path
	if cwd == "" {
		cwd = p.Path
	}
	ok, err := u.client.ResumeAvailable(u.ctx, src.kind, src.conversation, cwd)
	if err != nil {
		return err
	}
	if !ok {
		return forkPruned{i18n.T("sidebar.fork.pruned", i18n.Args{"name": src.label, "provider": src.kind})}
	}
	return nil
}

// addForkSession files a fork of src at path (stored as checkoutPath
// stores it), named after src, right under it in the drag order, and puts it
// on screen; its first start branches src's conversation.
func (u *window) addForkSession(p lichclient.Project, src forkSource, path string) (string, error) {
	id := strings.ToLower(rand.Text())
	label := forkLabel(src.label, labelsOf(p))
	if err := u.client.AddSessionFrom(u.ctx, p.ID, id, label, src.kind, checkoutPath(path, p.Path), p.NextSeq+1, src.id, src.label, src.folder); err != nil {
		return "", err
	}
	if err := u.client.MarkForked(u.ctx, id); err != nil {
		return "", err
	}
	u.terms.queueFork(id, src.conversation)
	u.logErr("reorder sessions", u.client.ReorderSessions(u.ctx, p.ID, placedAfter(sessionIDs(p), id, src.id)))
	return id, u.showNewSession(p.ID, id)
}

// showForkDialog is Fork ▸ Worktree…: the worktree dialog in its fork
// mode, on the session's own checkout and branch.
func (u *window) showForkDialog(id string) {
	p, src, ok := u.model.forkSource(id)
	if !ok {
		return
	}
	u.settings.hide()
	dir := src.path
	if dir == "" {
		dir = p.Path
	}
	checkout, _ := u.model.gitStatus(dir)
	u.worktree.show(u.ctx, p, src.kind, dir, checkout, &src)
	go u.model.refreshPath(u.ctx, dir)
}
