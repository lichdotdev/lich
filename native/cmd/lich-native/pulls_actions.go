package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// cleanupToastFor is how long the offer to remove a merged branch's worktree
// stays.
const cleanupToastFor = 10 * time.Second

var (
	flagPulls     = flag.Int("pulls", -1, "open the Pulls screen at start on this pull request (0 for the checkout's), for -shot")
	flagPullsTab  = flag.Int("pulls-tab", 0, "the tab -pulls opens on (0 Overview, 1 Conversation, 2 Commits, 3 Files, 4 Checks)")
	flagMergeEdit = flag.String("pulls-merge-edit", "", "open the merge message dialog for this method (squash, merge) once -pulls loads, for -shot")
	flagPullsEdit = flag.String("pulls-edit", "", "open the title or description edit (title, description) once -pulls loads, for -shot")
)

// pullsShot opens the screen at start as -pulls asks.
func (u *window) pullsShot() {
	if *flagPulls >= 0 {
		u.showPulls(*flagPulls)
		u.pulls.view.tabs.Selected = *flagPullsTab
		u.pulls.view.shotMergeEdit = *flagMergeEdit
		u.pulls.view.shotEdit = *flagPullsEdit
	}
}

// showPulls opens the Pulls screen on the active project, at the PR of the
// active session's checkout when it has one.
func (u *window) showPulls(number int) {
	p, ok := u.model.project()
	if !ok {
		return
	}
	u.settings.hide()
	branch, prNumber := u.checkoutPR(p)
	if number != 0 {
		prNumber = number
	}
	u.pulls.show(p, branch, prNumber)
	u.model.invalidate()
}

// checkoutPR is the active session's branch and its open PR, the project's
// own checkout's without a session.
func (u *window) checkoutPR(p backend.Project) (string, int) {
	if sv, ok := u.model.activeSession(); ok {
		if sv.pr != nil {
			return sv.git.Branch, sv.pr.Number
		}
		return sv.git.Branch, 0
	}
	g, _ := u.model.gitStatus(p.Path)
	return g.Branch, 0
}

func (u *window) togglePulls() {
	if u.pulls.open {
		u.pulls.hide()
		u.model.invalidate()
		return
	}
	u.showPulls(0)
}

// applyPulls carries out what the screen asked for, off the frame.
func (u *window) applyPulls(act pullsAction) {
	p := u.pulls
	path := p.project.Path
	if act.refresh {
		u.prChanged()
	}
	if act.external != "" {
		url := act.external
		go func() { u.logErr("open pull request", u.client.OpenExternal(u.ctx, url)) }()
	}
	if act.create {
		go func() {
			if err := u.client.CreatePullRequest(u.ctx, u.checkoutPath(path)); err != nil {
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pullsEmptyState.openFailed", i18n.Args{"error": backend.ErrorText(err)})})
			}
		}()
	}
	if act.session != nil {
		d, prompt := *act.session, act.handoff
		go u.openPRSession(p.project, d, prompt)
	}
	if p.selected != 0 {
		u.applyTalk(path, p.selected, act)
		u.applyEdit(path, p.selected, act)
	}
	if act.approve && p.selected != 0 {
		go u.approvePR(path, p.selected)
	}
	if act.merge != "" && p.selected != 0 {
		if d := p.store.detail(path, p.selected).val; d != nil {
			go u.mergePR(p.project, *d, act.merge, act.admin, "", "")
		}
	}
}

// checkoutPath is the active session's checkout, the project's own without
// one: where gh finds the branch a new PR is for.
func (u *window) checkoutPath(projectPath string) string {
	if sv, ok := u.model.activeSession(); ok {
		return sv.path
	}
	return projectPath
}

// prChanged is PRChangedEvent, and the Refresh button: every pull request read
// is owed a newer answer, the cards' badges too.
func (u *window) prChanged() {
	u.pulls.store.stale()
	u.model.mu.Lock()
	u.model.stalePullRequests()
	u.model.mu.Unlock()
	u.model.invalidate()
}

func (u *window) approvePR(path string, number int) {
	u.pulls.view.setBusy("approve")
	defer u.pulls.view.setBusy("")
	if err := u.client.SubmitReview(u.ctx, path, number, "approve", "", "", nil); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pullRequestView.reviewFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.pullRequestView.approvedToast", i18n.Args{"number": number})})
}

// openPRSession opens the PR in a session, then sends prompt (when any) as
// the session's prompt: a checkout already holding the branch is reused, its live
// session brought up or a new one opened; else the PR is checked out into a
// new worktree. The checkouts are read fresh, never from what drew the button.
func (u *window) openPRSession(project backend.Project, d backend.PRDetail, prompt string) {
	u.pulls.view.setBusy("session")
	defer u.pulls.view.setBusy("")
	id, err := u.prSession(project, d)
	if err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.openSessionFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	u.pulls.hide()
	u.model.invalidate()
	if prompt != "" {
		u.submitAtPrompt(id, prompt)
	}
}

func (u *window) prSession(project backend.Project, d backend.PRDetail) (string, error) {
	checkouts, err := u.pulls.store.freshCheckouts(project.Path)
	if err != nil {
		return "", err
	}
	i := slices.IndexFunc(checkouts, func(c backend.Worktree) bool { return c.Name == d.HeadRefName })
	if i < 0 {
		wt, err := u.client.CreateWorktreeFromPR(u.ctx, project.Path, project.ID, d.Number)
		if err != nil {
			return "", err
		}
		return u.addWorktreeSession(worktreeRequest{project: project, kind: u.model.newSessionKind()}, wt, "")
	}
	existing := checkouts[i]
	if id, ok := u.model.sessionToOpenAt(project.ID, existing.Path); ok {
		u.openSession(project.ID, id)
		return id, nil
	}
	if existing.Path == project.Path {
		return u.newSessionAt(project, existing.Path)
	}
	restored, err := u.client.ReopenWorktreeSession(u.ctx, project.ID, existing.Path, strings.ToLower(rand.Text()))
	if err != nil {
		return "", err
	}
	if restored == nil {
		return u.addWorktreeSession(worktreeRequest{project: project, kind: u.model.newSessionKind()},
			&backend.Worktree{Name: filepath.Base(existing.Path), Path: existing.Path}, "")
	}
	return restored.ID, u.showNewSession(project.ID, restored.ID)
}

// newSessionAt opens a session of the kind a new one spawns in the project's
// own checkout and puts it on screen.
func (u *window) newSessionAt(project backend.Project, path string) (string, error) {
	kind := u.model.newSessionKind()
	id := strings.ToLower(rand.Text())
	label := fmt.Sprintf("%s %d", strings.ToUpper(kind[:1])+kind[1:], project.NextSeq+1)
	if err := u.client.AddSessionInFolder(u.ctx, project.ID, id, label, kind, path, project.NextSeq+1, ""); err != nil {
		return "", err
	}
	return id, u.showNewSession(project.ID, id)
}

// mergePR merges with subject and body as the commit message (gh's own for an
// empty subject), tells the agents in the merged branch's checkout, and
// offers to remove that checkout when it is a worktree. It reports whether
// the merge went through.
func (u *window) mergePR(project backend.Project, d backend.PRDetail, method string, admin bool, subject, body string) bool {
	u.pulls.view.setBusy("merge")
	defer u.pulls.view.setBusy("")
	if err := u.client.MergePullRequest(u.ctx, project.Path, d.Number, method, subject, body, admin); err != nil {
		text := backend.ErrorText(err)
		if note := mergeRuleNote(d, u.pulls.store.branchRules(project.Path, d.BaseRefName)); note != "" {
			text += " " + note
		}
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pullRequestView.mergeFailed", i18n.Args{"error": text})})
		return false
	}
	merged := ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.pulls.merged", i18n.Args{"number": d.Number, "base": d.BaseRefName})}
	checkouts, err := u.pulls.store.freshCheckouts(project.Path)
	u.logErr("list checkouts", err)
	i := slices.IndexFunc(checkouts, func(c backend.Worktree) bool { return c.Name == d.HeadRefName })
	if i < 0 {
		u.toaster.Show(merged)
		return true
	}
	wt := checkouts[i]
	if err := u.client.AnnounceMerge(u.ctx, wt.Path, d.Number, d.Title, d.HeadRefName, d.BaseRefName); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.announceFailed", i18n.Args{"name": filepath.Base(wt.Path), "error": backend.ErrorText(err)})})
	}
	if wt.Path != project.Path {
		merged.Duration = cleanupToastFor
		merged.Action = &ui.ToastButton{Label: i18n.T("pulls.pulls.removeWorktree"), OnClick: func() { go u.removePRWorktree(project, wt.Path) }}
	}
	u.toaster.Show(merged)
	return true
}

// removePRWorktree removes a PR's worktree: the refusals first, the
// question about running sessions last, a click the refusals would make moot.
func (u *window) removePRWorktree(project backend.Project, path string) {
	name := filepath.Base(path)
	checkouts, err := u.pulls.store.freshCheckouts(project.Path)
	if err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.removeFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	if !slices.ContainsFunc(checkouts, func(c backend.Worktree) bool { return c.Path == path }) {
		u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.pulls.alreadyRemoved", i18n.Args{"name": name})})
		return
	}
	_, pinned, running := u.model.sessionsAt(project.ID, path)
	if pinned {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.pinnedSession")})
		return
	}
	if adopted, err := u.client.WorktreeAdopted(u.ctx, path); err != nil || adopted {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.notCreatedByLich")})
		return
	}
	if dirty, err := u.client.WorktreeDirty(u.ctx, path); err == nil && dirty {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.dirty")})
		return
	}
	if running > 0 {
		u.pulls.confirm.ask(project, path, running)
		u.model.invalidate()
		return
	}
	u.removeCheckout(project, path, false, false)
}

// removeCheckout discards the sessions rooted in path, so none survives to
// offer a resume in a directory that is gone, then removes the worktree.
// force and adoptedAck are RemoveWorktree's.
func (u *window) removeCheckout(project backend.Project, path string, force, adoptedAck bool) {
	ids, _, _ := u.model.sessionsAt(project.ID, path)
	for _, id := range ids {
		u.parkSession(id)
		u.terms.drop(id)
		u.logErr("close terminal", u.client.CloseTerminal(u.ctx, id))
	}
	u.logErr("purge worktree sessions", u.client.PurgeWorktreeSessions(u.ctx, project.ID, path))
	if err := u.client.RemoveWorktree(u.ctx, project.Path, path, force, adoptedAck); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.removeFailed", i18n.Args{"error": backend.ErrorText(err)})})
	} else {
		u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.pulls.removed", i18n.Args{"name": filepath.Base(path)})})
	}
	_, err := u.pulls.store.freshCheckouts(project.Path)
	u.logErr("list checkouts", err)
	u.model.invalidate()
}

// sessionsAt is the project's open sessions rooted in path, whether any is
// pinned, and how many hold a turn.
func (m *model) sessionsAt(projectID, path string) (ids []string, pinned bool, running int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.projects, func(p backend.Project) bool { return p.ID == projectID })
	if i < 0 {
		return nil, false, 0
	}
	p := m.projects[i]
	for _, s := range p.Sessions {
		if sessionPath(p, s) == path {
			ids = append(ids, s.ID)
			pinned = pinned || s.Pinned
		}
	}
	return ids, pinned, len(m.status.running(ids))
}

// sessionToOpenAt is the session the window brings up for path: the one on
// screen when it sits there, else the first; false when none does.
func (m *model) sessionToOpenAt(projectID, path string) (string, bool) {
	ids, _, _ := m.sessionsAt(projectID, path)
	if len(ids) == 0 {
		return "", false
	}
	m.mu.Lock()
	active := m.active
	m.mu.Unlock()
	if slices.Contains(ids, active) {
		return active, true
	}
	return ids[0], true
}

// removeConfirm asks before removing a worktree whose sessions are mid-turn,
// which the merge notice likely just woke.
type removeConfirm struct {
	dialog ui.DialogState
	// mu guards the question, which the removal asks off the frame.
	mu      sync.Mutex
	open    bool
	project backend.Project
	path    string
	running int
	cancel  widget.Clickable
	confirm widget.Clickable
}

func (c *removeConfirm) ask(project backend.Project, path string, running int) {
	c.mu.Lock()
	c.open, c.project, c.path, c.running = true, project, path, running
	c.mu.Unlock()
}

// layoutRemoveConfirm draws the question and removes on its yes.
func (u *window) layoutRemoveConfirm(gtx C, th *ui.Theme) {
	c := &u.pulls.confirm
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.open && !c.dialog.Animating(gtx) {
		return
	}
	if c.confirm.Clicked(gtx) && c.open {
		c.open = false
		project, path := c.project, c.path
		go u.removeCheckout(project, path, false, false)
	}
	if c.cancel.Clicked(gtx) {
		c.open = false
	}
	body := i18n.T("pulls.pullsActions.running", i18n.Args{"count": c.running, "name": filepath.Base(c.path)})
	on := th.On(th.Popover)
	cancel := on.Button(&c.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	anyway := on.Button(&c.confirm, i18n.T("pulls.pulls.runningRemoval.removeAnyway"))
	anyway.Variant = ui.VariantDestructive
	d := th.Dialog(&c.dialog, c.open, i18n.T("pulls.pullsActions.runningTitle"), body)
	d.Actions = []layout.Widget{cancel.Layout, anyway.Layout}
	if d.Layout(gtx) {
		c.open = false
	}
}
