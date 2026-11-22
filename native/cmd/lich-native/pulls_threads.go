package main

import (
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// canSendThreads is whether the PR's threads may go to a session: it is open,
// and its branch is one lich can check out and push to.
func canSendThreads(d backend.PRDetail) bool {
	return d.State == "OPEN" && (!d.IsCrossRepository || d.MaintainerCanModify)
}

func (p *pullsScreen) threadSent(id string) (sent, fixed bool) {
	if p.threadState == nil {
		return false, false
	}
	return p.threadState(id)
}

// threadSendable is whether thread t can go to a session: open, not marked
// fixed and waiting for its push, and with a comment for the reply to hang
// off. One sent and never marked, a question the agent brought back, can go
// again once it is discussed.
func (p *pullsScreen) threadSendable(t backend.PRReviewThread) bool {
	_, fixed := p.threadSent(t.ID)
	return !t.IsResolved && len(t.Comments) > 0 && !fixed
}

// sendableThreads is the threads Send open threads hands over: none already
// sent, which the agent may still be working on.
func (p *pullsScreen) sendableThreads(c *backend.PRConversation) []backend.PRReviewThread {
	if c == nil {
		return nil
	}
	var out []backend.PRReviewThread
	for _, t := range c.Threads {
		if sent, _ := p.threadSent(t.ID); p.threadSendable(t) && !sent {
			out = append(out, t)
		}
	}
	return out
}

// threadsRequest is what a session is handed for threads: each one where it
// is, its link and what was said in it; the backend words the ask.
func threadsRequest(d backend.PRDetail, threads []backend.PRReviewThread) api.RequestOptions {
	req := api.RequestOptions{Topic: api.RequestReviewThread, PR: d.Number, Branch: d.HeadRefName}
	for _, t := range threads {
		item := api.RequestItem{Path: t.Path, Lines: threadLines(t), ID: t.ID, Outdated: t.IsOutdated}
		if d.URL != "" && len(t.Comments) > 0 {
			item.URL = fmt.Sprintf("%s#discussion_r%d", d.URL, t.Comments[0].ID)
		}
		for _, c := range t.Comments {
			item.Comments = append(item.Comments, api.RequestComment{Author: c.Author, Body: c.Body})
		}
		req.Items = append(req.Items, item)
	}
	return req
}

// threadLines is the line ("19") or the span ("12-30") a thread sits on.
func threadLines(t backend.PRReviewThread) string {
	if t.StartLine > 0 && t.StartLine != t.Line {
		return fmt.Sprintf("%d-%d", t.StartLine, t.Line)
	}
	return fmt.Sprint(t.Line)
}

// sendThreads hands threads to the PR's session, opening one off screen when
// its checkout has none, as the PR handoffs do, and, once the session took the
// prompt, has the backend keep each for its agent to mark fixed.
func (u *window) sendThreads(project backend.Project, d backend.PRDetail, threads []backend.PRReviewThread) {
	u.pulls.view.setBusy("threads")
	defer u.pulls.view.setBusy("")
	id, err := u.prSession(project, d)
	if err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pulls.openSessionFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	u.model.invalidate()
	if !u.handOffPR(project.ID, id, threadsRequest(d, threads)) {
		return
	}
	for _, t := range threads {
		opts := api.ThreadFixOptions{Path: project.Path, Number: d.Number, ThreadID: t.ID, CommentID: t.Comments[len(t.Comments)-1].ID}
		if err := u.client.AwaitThreadFix(u.ctx, opts); err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.threadSend.awaitFailed", i18n.Args{"error": backend.ErrorText(err)})})
			return
		}
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.threadSend.sent", i18n.Args{"count": len(threads), "name": u.model.sessionLabel(id)}),
		Action: u.openPRButton(project.ID, id)})
}

// threadSentLabel is what a thread's header says of its send: nothing for one
// never sent.
func (p *pullsScreen) threadSentLabel(id string) string {
	switch sent, fixed := p.threadSent(id); {
	case fixed:
		return i18n.T("pulls.threadSend.waiting")
	case sent:
		return i18n.T("pulls.threadSend.handed")
	}
	return ""
}
