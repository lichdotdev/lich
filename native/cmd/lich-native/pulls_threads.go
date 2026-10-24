package main

import (
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// ghostAuthor is the login GitHub shows for an account deleted since.
const ghostAuthor = "ghost"

// canSendThreads is whether the PR's threads may go to a session: it is open,
// and its branch is one lich can check out and push to.
func canSendThreads(d backend.PRDetail) bool {
	return d.State == "OPEN" && (!d.IsCrossRepository || d.MaintainerCanModify)
}

// threadWaiting is whether thread id waits for the push that resolves it.
func (p *pullsScreen) threadWaiting(id string) bool { return p.waiting != nil && p.waiting(id) }

// threadSendable is whether thread t can go to a session: open, not already
// waiting for its push, and with a comment for the replies to hang off.
func (p *pullsScreen) threadSendable(t backend.PRReviewThread) bool {
	return !t.IsResolved && len(t.Comments) > 0 && !p.threadWaiting(t.ID)
}

// sendableThreads is the threads Send open threads hands over.
func (p *pullsScreen) sendableThreads(c *backend.PRConversation) []backend.PRReviewThread {
	if c == nil {
		return nil
	}
	var out []backend.PRReviewThread
	for _, t := range c.Threads {
		if p.threadSendable(t) {
			out = append(out, t)
		}
	}
	return out
}

// threadsPrompt is what a session gets for threads: each one where it is and
// what was said in it, then the ask.
func threadsPrompt(d backend.PRDetail, threads []backend.PRReviewThread) string {
	parts := make([]string, 0, len(threads)+1)
	for _, t := range threads {
		parts = append(parts, threadBlock(d, t))
	}
	parts = append(parts, i18n.Prompt("prompts.reviewThread.ask", i18n.Args{"count": len(threads)}))
	return strings.Join(parts, "\n\n")
}

func threadBlock(d backend.PRDetail, t backend.PRReviewThread) string {
	args := i18n.Args{"number": d.Number, "branch": d.HeadRefName, "path": t.Path, "lines": threadLines(t)}
	head := i18n.Prompt("prompts.reviewThread.head", args)
	if t.IsOutdated {
		head = i18n.Prompt("prompts.reviewThread.headOutdated", args)
	}
	lines := []string{head}
	if d.URL != "" && len(t.Comments) > 0 {
		lines = append(lines, fmt.Sprintf("%s#discussion_r%d", d.URL, t.Comments[0].ID))
	}
	lines = append(lines, "")
	for _, c := range t.Comments {
		author := c.Author
		if author == "" {
			author = ghostAuthor
		}
		lines = append(lines, "- @"+author+": "+continuation(strings.TrimSpace(c.Body)))
	}
	return strings.Join(lines, "\n")
}

// threadLines is the line ("19") or the span ("12-30") a thread sits on.
func threadLines(t backend.PRReviewThread) string {
	if t.StartLine > 0 && t.StartLine != t.Line {
		return fmt.Sprintf("%d-%d", t.StartLine, t.Line)
	}
	return fmt.Sprint(t.Line)
}

// sendThreads hands threads to the PR's session, opening one when its checkout
// has none, and, once the session took the prompt, has each answered on
// GitHub and waiting for the push that resolves it.
func (u *window) sendThreads(project backend.Project, d backend.PRDetail, threads []backend.PRReviewThread) {
	u.pulls.view.setBusy("threads")
	defer u.pulls.view.setBusy("")
	id, err := u.prSession(project, d)
	if err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.openSessionFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	if !u.awaitPrompt(id) {
		return
	}
	if err := u.client.Submit(u.ctx, id, threadsPrompt(d, threads)); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.worktreeDialog.handoffLost", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	for _, t := range threads {
		opts := api.ThreadFixOptions{Path: project.Path, Number: d.Number, ThreadID: t.ID, CommentID: t.Comments[len(t.Comments)-1].ID}
		if err := u.client.AwaitThreadFix(u.ctx, opts); err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.threadSend.awaitFailed", i18n.Args{"error": backend.ErrorText(err)})})
			return
		}
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.threadSend.sent", i18n.Args{"count": len(threads), "name": u.model.sessionLabel(id)})})
}
