package main

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func submitVerdictTitle(verdict string) string {
	switch verdict {
	case "approve":
		return i18n.T("pulls.submitReviewDialog.verdict.approve.title")
	case "request_changes":
		return i18n.T("pulls.submitReviewDialog.verdict.request_changes.title")
	}
	return i18n.T("pulls.submitReviewDialog.verdict.comment.title")
}

// threadReply and threadResolve are a thread's writes, carried to the window.
type threadReply struct {
	commentID int64
	body      string
	thread    string
}

type threadResolve struct {
	id       string
	resolved bool
}

// submitState is the review dialog: the verdict it sends, open while set.
type submitState struct {
	menu    ui.MenuState
	click   widget.Clickable
	verdict string
	dialog  ui.DialogState
	summary widget.Editor
	fit     ui.TextareaResize
	send    widget.Clickable
	cancel  widget.Clickable
}

// reviewButton is Approve while nothing is pending, else the "Submit review
// N" menu of verdicts, which opens the dialog.
func (v *pullView) reviewButton(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, busy, blocked string, act *pullsAction) D {
	pending := len(v.pendingOf(d.URL).snapshot())
	if pending == 0 {
		label := i18n.T("pulls.pullRequestView.approve")
		switch {
		case busy == "approve":
			label = i18n.T("pulls.pullRequestView.approving")
		case d.ReviewDecision == "APPROVED":
			label = i18n.T("pulls.pullRequestView.approved")
		}
		b := th.Button(&v.approve, label)
		b.Icon, b.Variant, b.Size = icPullCheck, ui.VariantGhost, ui.SizeSM
		b.Disabled = busy != "" || blocked != "" || d.ReviewDecision == "APPROVED"
		return v.withTip(gtx, th, p, "approve", blocked, b.Layout)
	}
	s := &v.submit
	discard := i18n.T("pulls.pullRequestView.discardPending", i18n.Args{"count": pending})
	url := d.URL
	entries := []ui.MenuEntry{
		{Label: submitVerdictTitle("comment"), OnClick: func() { s.verdict = "comment" }},
		{Label: submitVerdictTitle("approve"), OnClick: func() { s.verdict = "approve" }},
		{Label: submitVerdictTitle("request_changes"), OnClick: func() { s.verdict = "request_changes" }},
		{Kind: ui.MenuSeparator},
		{Label: discard, OnClick: func() { v.pendingOf(url).clear() }},
	}
	b := th.Button(&s.click, i18n.T("pulls.pullsSubmit.review", i18n.Args{"count": pending}))
	b.Icon, b.Variant, b.Size, b.Disabled = icPullTalk, ui.VariantGhost, ui.SizeSM, blocked != ""
	if b.Disabled {
		return v.withTip(gtx, th, p, "approve", blocked, b.Layout)
	}
	return th.DropdownMenu(p.root, &s.menu, entries).Layout(gtx, b.Layout)
}

// layoutSubmitDialog is SubmitReviewDialog, laid out by the window ahead of
// its content: the optional summary and the count of line comments riding
// on the verdict.
func (u *window) layoutSubmitDialog(gtx C, th *ui.Theme) {
	v := &u.pulls.view
	s := &v.submit
	p := u.pulls
	if s.verdict == "" && !s.dialog.Animating(gtx) {
		return
	}
	if s.cancel.Clicked(gtx) {
		s.verdict = ""
	}
	open := s.verdict != "" && p.open && p.selected != 0
	var d *backend.PRDetail
	if open {
		d = p.store.detail(p.project.Path, p.selected).val
		open = d != nil
	}
	busy := v.busyNow() == "submit"
	if (s.send.Clicked(gtx) || ctrlEnter(gtx, &s.summary)) && open && !busy {
		review := v.pendingOf(d.URL)
		go u.submitPR(p.project.Path, *d, s.verdict, strings.TrimSpace(s.summary.Text()), review)
	}
	description := i18n.T("pulls.submitReviewDialog.summaryAlone")
	if open {
		if n := len(v.pendingOf(d.URL).snapshot()); n > 0 {
			description = i18n.T("pulls.pullsSubmit.withComments", i18n.Args{"count": n})
		}
	}
	on := th.On(th.Popover)
	cancel := on.Button(&s.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	label := i18n.T("pulls.pullsSubmit.send")
	if busy {
		label = i18n.T("pulls.pullsSubmit.sending")
	}
	send := on.Button(&s.send, label)
	send.Disabled = busy
	dlg := th.Dialog(&s.dialog, open, submitVerdictTitle(s.verdict), description)
	summary := on.Textarea(&s.summary, i18n.T("pulls.submitReviewDialog.summaryPlaceholder"))
	summary.Resize = &s.fit
	dlg.Body = summary.Layout
	dlg.Actions = []layout.Widget{cancel.Layout, send.Layout}
	dlg.Root = &u.root
	if dlg.Layout(gtx) {
		s.verdict = ""
	}
}

// ctrlEnter reports whether Ctrl+Enter (Cmd+Enter on macOS) was pressed in
// ed: CommentBox's send, Enter alone being a new line.
func ctrlEnter(gtx C, ed *widget.Editor) bool {
	_, ok := gtx.Event(key.Filter{Focus: ed, Name: key.NameReturn, Required: key.ModShortcut})
	return ok
}

func keyFocus(ed *widget.Editor) key.FocusCmd { return key.FocusCmd{Tag: ed} }

func (u *window) submitPR(path string, d backend.PRDetail, verdict, summary string, review *pendingReview) {
	v := &u.pulls.view
	v.setBusy("submit")
	defer v.setBusy("")
	head := ""
	if conv := u.pulls.store.conversation(path, d.Number).val; conv != nil {
		head = conv.HeadRefOID
	}
	sent := review.snapshot()
	if err := u.client.SubmitReview(u.ctx, path, d.Number, verdict, summary, head, sent); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("pulls.pullRequestView.reviewFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	review.drop(sent)
	v.onFrame(func() {
		v.submit.verdict = ""
		v.submit.summary.SetText("")
	})
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("pulls.pullRequestView.reviewSent", i18n.Args{"number": d.Number})})
	u.model.invalidate()
}

// applyTalk carries out the Conversation tab's writes off the frame; the
// re-read comes back as PRChangedEvent.
func (u *window) applyTalk(path string, number int, act pullsAction) {
	v := &u.pulls.view
	if act.comment != "" {
		body := act.comment
		go u.prWrite("comment", func(err error) string {
			return i18n.T("pulls.pullsConversation.commentFailed", i18n.Args{"error": backend.ErrorText(err)})
		}, func() error {
			if err := u.client.CommentOnPullRequest(u.ctx, path, number, body); err != nil {
				return err
			}
			v.onFrame(func() { v.talk.comment.SetText("") })
			return nil
		})
	}
	if r := act.reply; r != nil {
		reply := *r
		go u.prWrite("reply:"+reply.thread, func(err error) string {
			return i18n.T("pulls.reviewThread.replyFailed", i18n.Args{"error": backend.ErrorText(err)})
		}, func() error {
			if err := u.client.ReplyToReviewThread(u.ctx, path, number, reply.commentID, reply.body); err != nil {
				return err
			}
			v.onFrame(func() {
				v.talk.editorOf(reply.thread).SetText("")
				v.talk.replying[reply.thread] = false
			})
			return nil
		})
	}
	if e := act.edit; e != nil {
		go u.editComment(path, number, *e)
	}
	if r := act.resolve; r != nil {
		resolve := *r
		go u.prWrite("resolve:"+resolve.id, func(err error) string {
			args := i18n.Args{"error": backend.ErrorText(err)}
			if resolve.resolved {
				return i18n.T("pulls.reviewThread.resolveFailed", args)
			}
			return i18n.T("pulls.reviewThread.reopenFailed", args)
		}, func() error {
			return u.client.ResolveReviewThread(u.ctx, path, resolve.id, resolve.resolved)
		})
	}
}

// prWrite runs one write with its button saying so, and toasts gh's refusal.
func (u *window) prWrite(busy string, failed func(error) string, write func() error) {
	u.pulls.view.setBusy(busy)
	defer u.pulls.view.setBusy("")
	if err := write(); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: failed(err)})
	}
	u.model.invalidate()
}
