package main

import (
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n
// (pulls.pullRequestView, pulls.submitReviewDialog).
const (
	submitReview      = "Submit review %d"
	submitComment     = "Comment"
	submitApprove     = "Approve"
	submitChanges     = "Request changes"
	submitDiscardOne  = "Discard 1 pending comment"
	submitDiscardMany = "Discard %d pending comments"
	submitTitle       = "Submit review"
	submitWithOne     = "With 1 line comment."
	submitWithMany    = "With %d line comments."
	submitAlone       = "No line comments — this sends the summary alone."
	submitSummaryHint = "Summary — optional"
	submitSend        = "Submit"
	submitSending     = "Submitting…"
	submitCancel      = "Cancel"
	submitSentToast   = "Review sent on #%d"
	commentFailed     = "Comment failed: %s"
	replyFailed       = "Reply failed: %s"
	resolveFailed     = "Resolve failed: %s"
	reopenFailed      = "Reopen failed: %s"
)

var submitVerdicts = map[string]string{"comment": submitComment, "approve": submitApprove, "request_changes": submitChanges}

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
func (v *pullView) reviewButton(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, busy, blocked string, act *pullsAction) D {
	pending := len(v.pendingOf(d.URL).snapshot())
	if pending == 0 {
		label := pullsApprove
		switch {
		case busy == "approve":
			label = pullsApproving
		case d.ReviewDecision == "APPROVED":
			label = pullsApproved
		}
		b := th.Button(&v.approve, label)
		b.Icon, b.Variant, b.Size = icPullCheck, ui.VariantGhost, ui.SizeSM
		b.Disabled = busy != "" || blocked != "" || d.ReviewDecision == "APPROVED"
		return v.withTip(gtx, th, p, "approve", blocked, b.Layout)
	}
	s := &v.submit
	discard := submitDiscardOne
	if pending != 1 {
		discard = fmt.Sprintf(submitDiscardMany, pending)
	}
	url := d.URL
	entries := []ui.MenuEntry{
		{Label: submitComment, OnClick: func() { s.verdict = "comment" }},
		{Label: submitApprove, OnClick: func() { s.verdict = "approve" }},
		{Label: submitChanges, OnClick: func() { s.verdict = "request_changes" }},
		{Kind: ui.MenuSeparator},
		{Label: discard, OnClick: func() { v.pendingOf(url).clear() }},
	}
	b := th.Button(&s.click, fmt.Sprintf(submitReview, pending))
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
	open := s.verdict != "" && p.open && p.selected != 0
	var d *lichclient.PRDetail
	if open {
		d = p.store.detail(p.project.Path, p.selected).val
		open = d != nil
	}
	if s.cancel.Clicked(gtx) {
		s.verdict = ""
	}
	busy := v.busyNow() == "submit"
	if (s.send.Clicked(gtx) || ctrlEnter(gtx, &s.summary)) && open && !busy {
		review := v.pendingOf(d.URL)
		go u.submitPR(p.project.Path, *d, s.verdict, strings.TrimSpace(s.summary.Text()), review)
	}
	description := submitAlone
	if open {
		switch n := len(v.pendingOf(d.URL).snapshot()); {
		case n == 1:
			description = submitWithOne
		case n > 1:
			description = fmt.Sprintf(submitWithMany, n)
		}
	}
	on := th.On(th.Popover)
	cancel := on.Button(&s.cancel, submitCancel)
	cancel.Variant = ui.VariantOutline
	label := submitSend
	if busy {
		label = submitSending
	}
	send := on.Button(&s.send, label)
	send.Disabled = busy
	dlg := th.Dialog(&s.dialog, open, submitVerdicts[s.verdict], description)
	summary := on.Textarea(&s.summary, submitSummaryHint)
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

func (u *window) submitPR(path string, d lichclient.PRDetail, verdict, summary string, review *pendingReview) {
	v := &u.pulls.view
	v.setBusy("submit")
	defer v.setBusy("")
	head := ""
	if conv := u.pulls.store.conversation(path, d.Number).val; conv != nil {
		head = conv.HeadRefOID
	}
	if err := u.client.SubmitReview(u.ctx, path, d.Number, verdict, summary, head, review.snapshot()); err != nil {
		u.toaster.Show(failure(pullsReviewFailed, err))
		return
	}
	review.clear()
	v.onFrame(func() {
		v.submit.verdict = ""
		v.submit.summary.SetText("")
	})
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: fmt.Sprintf(submitSentToast, d.Number)})
	u.model.invalidate()
}

// applyTalk carries out the Conversation tab's writes off the frame; the
// re-read comes back as PRChangedEvent.
func (u *window) applyTalk(path string, number int, act pullsAction) {
	v := &u.pulls.view
	if act.comment != "" {
		body := act.comment
		go u.prWrite("comment", commentFailed, func() error {
			if err := u.client.CommentOnPullRequest(u.ctx, path, number, body); err != nil {
				return err
			}
			v.onFrame(func() { v.talk.comment.SetText("") })
			return nil
		})
	}
	if r := act.reply; r != nil {
		reply := *r
		go u.prWrite("reply:"+reply.thread, replyFailed, func() error {
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
	if r := act.resolve; r != nil {
		resolve, failed := *r, resolveFailed
		if !resolve.resolved {
			failed = reopenFailed
		}
		go u.prWrite("resolve:"+resolve.id, failed, func() error {
			return u.client.ResolveReviewThread(u.ctx, path, resolve.id, resolve.resolved)
		})
	}
}

// prWrite runs one write with its button saying so, and toasts gh's refusal.
func (u *window) prWrite(busy, failedFormat string, write func() error) {
	u.pulls.view.setBusy(busy)
	defer u.pulls.view.setBusy("")
	if err := write(); err != nil {
		u.toaster.Show(failure(failedFormat, err))
	}
	u.model.invalidate()
}
