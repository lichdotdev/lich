package main

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// talkState is the Conversation tab's widgets, by thread id where a thread
// owns one.
type talkState struct {
	folds        map[string]*widget.Bool
	foldClicks   map[string]*widget.Clickable
	replying     map[string]bool
	replyFields  map[string]*replyField
	replyClicks  map[string]*widget.Clickable
	sendClicks   map[string]*widget.Clickable
	cancelClicks map[string]*widget.Clickable
	resolveClick map[string]*widget.Clickable
	showResolved widget.Bool
	resolvedTap  widget.Clickable
	comment      widget.Editor
	commentSize  ui.TextareaResize
	commentSend  widget.Clickable
	discards     map[int]*widget.Clickable
	edits        map[int]*widget.Clickable
	pendingEdit  pendingEdit
}

// pendingEdit is the box rewording one comment of the pending review, by its
// index there.
type pendingEdit struct {
	open         bool
	index        int
	editor       widget.Editor
	resize       ui.TextareaResize
	save, cancel widget.Clickable
}

func (s *talkState) init() {
	s.folds, s.foldClicks = map[string]*widget.Bool{}, map[string]*widget.Clickable{}
	s.replying, s.replyFields = map[string]bool{}, map[string]*replyField{}
	s.replyClicks, s.sendClicks = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	s.cancelClicks, s.resolveClick = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	s.discards, s.edits = map[int]*widget.Clickable{}, map[int]*widget.Clickable{}
}

// replyField is one thread's reply box: its text and the height it was
// dragged to.
type replyField struct {
	editor widget.Editor
	resize ui.TextareaResize
}

func (s *talkState) fieldOf(id string) *replyField {
	f, ok := s.replyFields[id]
	if !ok {
		f = new(replyField)
		s.replyFields[id] = f
	}
	return f
}

func (s *talkState) editorOf(id string) *widget.Editor { return &s.fieldOf(id).editor }

// talkClicks carries out the clicks that add or drop rows, before the rows
// are built.
func (v *pullView) talkClicks(gtx C, pending *pendingReview) {
	s := &v.talk
	if s.resolvedTap.Clicked(gtx) {
		s.showResolved.Value = !s.showResolved.Value
	}
	for i, c := range pending.snapshot() {
		if discard := s.discards[i]; discard != nil && discard.Clicked(gtx) {
			pending.remove(i)
			// Discarding shifts the indices the edit box was opened by.
			s.pendingEdit.open = false
		}
		if edit := s.edits[i]; edit != nil && edit.Clicked(gtx) {
			s.pendingEdit.open, s.pendingEdit.index = true, i
			s.pendingEdit.editor.SetText(c.Body)
			gtx.Execute(keyFocus(&s.pendingEdit.editor))
		}
	}
	s.pendingEdit.clicks(gtx, pending)
}

// clicks files the box's text on Save or Ctrl+Enter, or closes it on Cancel.
func (e *pendingEdit) clicks(gtx C, pending *pendingReview) {
	text := strings.TrimSpace(e.editor.Text())
	if (e.save.Clicked(gtx) || ctrlEnter(gtx, &e.editor)) && e.open && text != "" {
		pending.edit(e.index, text)
		e.open = false
	}
	if e.cancel.Clicked(gtx) {
		e.open = false
	}
}

// conversationRows is PullsConversation: the pending review, the timeline,
// the resolved threads behind their count, and the box for a comment on the
// pull request itself.
func (v *pullView) conversationRows(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) []layout.Widget {
	read := p.store.conversation(p.project.Path, d.Number)
	switch {
	case read.val == nil && read.err != "":
		return []layout.Widget{func(gtx C) D {
			return panelNotice(gtx, th, i18n.T("pulls.pullsConversation.readFailed", i18n.Args{"error": read.err}))
		}}
	case read.val == nil:
		return []layout.Widget{func(gtx C) D { return panelNotice(gtx, th, i18n.T("pulls.pullsConversation.loading")) }}
	}
	items, resolved := conversationTimeline(read.val)
	now := gtx.Now
	v.talkClicks(gtx, v.pendingOf(d.URL))
	rows := v.pendingRows(th, v.pendingOf(d.URL))
	if len(items) == 0 && len(resolved) == 0 {
		rows = append(rows, func(gtx C) D { return panelNotice(gtx, th, i18n.T("pulls.pullsConversation.nothingYet")) })
	}
	for _, it := range items {
		switch {
		case it.review != nil:
			rows = append(rows, v.talkEntry(th, "review:"+it.review.ID, it.review.Author, verdictLine(*it.review), it.review.Body, it.review.Date, now, act))
		case it.comment != nil:
			rows = append(rows, v.talkEntry(th, "comment:"+it.comment.Date+it.comment.Author, it.comment.Author, "", it.comment.Body, it.comment.Date, now, act))
		default:
			t := *it.thread
			rows = append(rows, func(gtx C) D { return v.threadCard(gtx, th, p, d, t, act, now) })
		}
	}
	if len(resolved) > 0 {
		rows = append(rows, func(gtx C) D { return v.resolvedToggle(gtx, th, len(resolved)) })
		if v.talk.showResolved.Value {
			for _, t := range resolved {
				rows = append(rows, func(gtx C) D { return v.threadCard(gtx, th, p, d, t, act, now) })
			}
		}
	}
	return append(rows, func(gtx C) D { return v.commentBox(gtx, th, act) })
}

func verdictLine(r lichclient.PRReview) string {
	if v, ok := reviewVerdict(r.State); ok {
		return v
	}
	return strings.ToLower(r.State)
}

// talkEntry is a review or a comment: who, what they decided, when, and what
// they wrote.
func (v *pullView) talkEntry(th *ui.Theme, key, author, verdict, body, date string, now time.Time, act *pullsAction) layout.Widget {
	body = strings.TrimSpace(body)
	return func(gtx C) D {
		return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(3), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return byline(gtx, th, author, verdict, date, now) }),
				layout.Rigid(func(gtx C) D {
					if body == "" {
						return D{}
					}
					return layout.Inset{Top: ui.Space(1)}.Layout(gtx, func(gtx C) D { return v.md.layout(gtx, th, key, body, act.open) })
				}),
			)
		})
	}
}

// byline is Byline: the author, what they did, and how long ago.
func byline(gtx C, th *ui.Theme, author, verdict, date string, now time.Time) D {
	if author == "" {
		author = i18n.T("pulls.byline.someone")
	}
	children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, author).Weight(font.Medium).Layout)}
	if verdict != "" {
		children = append(children, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, verdict).Muted().Layout))
	}
	if at := parsedTime(date); !at.IsZero() {
		children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, agoLabel(at.Unix(), now)).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
}

// threadCard is ReviewThread: the anchor and its badges over GitHub's hunk,
// the comments, then Reply and Resolve; a resolved thread starts folded.
func (v *pullView) threadCard(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, t lichclient.PRReviewThread, act *pullsAction, now time.Time) D {
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		return v.threadBox(gtx, th, p, d, t, act, now)
	})
}

// threadBox is the thread's card itself, folded to its header or open; the
// Conversation tab and the diff share its fold.
func (v *pullView) threadBox(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, t lichclient.PRReviewThread, act *pullsAction, now time.Time) D {
	s := &v.talk
	fold := boolOf(s.folds, t.ID)
	if _, seen := s.foldClicks[t.ID]; !seen {
		fold.Value = !t.IsResolved
	}
	foldClick := clickableOf(s.foldClicks, t.ID)
	if foldClick.Clicked(gtx) {
		fold.Value = !fold.Value
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
		func(gtx C) D {
			on := th.On(th.Sidebar)
			return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return foldClick.Layout(gtx, func(gtx C) D { return threadHeader(gtx, on, t, fold.Value) })
					}),
					layout.Rigid(func(gtx C) D {
						if !fold.Value {
							return D{}
						}
						return v.threadBody(gtx, on, p, d, t, act, now)
					}),
				)
			})
		})
}

func threadHeader(gtx C, th *ui.Theme, t lichclient.PRReviewThread, open bool) D {
	chevron := icPullRight
	if open {
		chevron = icPullDown
	}
	start := t.StartLine
	if start == t.Line {
		start = 0
	}
	anchor := lineAnchor(start, t.Line)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
		ui.Gap(1.5),
		layout.Flexed(1, func(gtx C) D {
			return truncated(gtx, th.Text(ui.TextXS, t.Path+" · "+anchor).Mono())
		}),
	}
	if t.IsOutdated {
		children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, i18n.T("pulls.pullsConversation.outdated")).In(th.ToneWait).Layout))
	}
	if t.IsResolved {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D {
			return metaGlyph(gtx, th, icPullCheck, i18n.T("pulls.reviewThread.resolved"), th.TonePass)
		}))
	}
	children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, fmt.Sprint(len(t.Comments))).Muted().Layout))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (v *pullView) threadBody(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, t lichclient.PRReviewThread, act *pullsAction, now time.Time) D {
	var rows []layout.FlexChild
	if len(t.Comments) > 0 && t.Comments[0].DiffHunk != "" {
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(2)}.Layout(gtx, func(gtx C) D { return hunkTail(gtx, th, t.Comments[0].DiffHunk) })
		}))
	}
	for _, c := range t.Comments {
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return byline(gtx, th, c.Author, "", c.Date, now) }),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return wrapped(gtx, th.Text(ui.TextSM, c.Body)) })
					}),
				)
			})
		}))
	}
	rows = append(rows, layout.Rigid(func(gtx C) D {
		return layout.Inset{Top: ui.Space(2.5)}.Layout(gtx, func(gtx C) D { return v.threadActions(gtx, th, d, t, act) })
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// lineAnchor names the line a comment sits on, or the lines it spans from a
// positive start.
func lineAnchor(start, line int) string {
	if start > 0 {
		return i18n.T("pulls.pullsConversation.lines", i18n.Args{"start": start, "end": line})
	}
	return i18n.T("pulls.pullsConversation.line", i18n.Args{"line": line})
}

// hunkLines is ThreadHunk's tail: the last lines of GitHub's hunk, the ones
// the thread is about.
const hunkLines = 4

func hunkTail(gtx C, th *ui.Theme, hunk string) D {
	lines := strings.Split(strings.TrimRight(hunk, "\n"), "\n")
	lines = lines[max(0, len(lines)-hunkLines):]
	var children []layout.FlexChild
	for _, line := range lines {
		fg := th.MutedForeground
		switch {
		case strings.HasPrefix(line, "+"):
			fg = th.TonePass
		case strings.HasPrefix(line, "-"):
			fg = th.Destructive
		}
		children = append(children, layout.Rigid(func(gtx C) D { return truncated(gtx, th.Text(ui.TextXS, line).Mono().In(fg)) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// threadActions is Reply and Resolve, or the reply box while one is written.
func (v *pullView) threadActions(gtx C, th *ui.Theme, d lichclient.PRDetail, t lichclient.PRReviewThread, act *pullsAction) D {
	s := &v.talk
	reply, resolve := clickableOf(s.replyClicks, t.ID), clickableOf(s.resolveClick, t.ID)
	if reply.Clicked(gtx) {
		s.replying[t.ID] = true
		gtx.Execute(keyFocus(s.editorOf(t.ID)))
	}
	if resolve.Clicked(gtx) {
		act.resolve = &threadResolve{id: t.ID, resolved: !t.IsResolved}
	}
	if clickableOf(s.cancelClicks, t.ID).Clicked(gtx) {
		s.replying[t.ID] = false
	}
	if s.replying[t.ID] {
		return v.replyBox(gtx, th, d, t, act)
	}
	busy := v.busyNow()
	label := i18n.T("pulls.reviewThread.resolve")
	if t.IsResolved {
		label = i18n.T("pulls.reviewThread.reopen")
	}
	replyB := th.Button(reply, i18n.T("pulls.reviewThread.reply"))
	replyB.Variant, replyB.Size = ui.VariantOutline, ui.SizeXS
	resolveB := th.Button(resolve, label)
	resolveB.Variant, resolveB.Size, resolveB.Disabled = ui.VariantGhost, ui.SizeXS, busy == "resolve:"+t.ID
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(replyB.Layout), ui.Gap(2), layout.Rigid(resolveB.Layout))
}

func (v *pullView) replyBox(gtx C, th *ui.Theme, d lichclient.PRDetail, t lichclient.PRReviewThread, act *pullsAction) D {
	s := &v.talk
	ed := s.editorOf(t.ID)
	send, cancel := clickableOf(s.sendClicks, t.ID), clickableOf(s.cancelClicks, t.ID)
	submit := send.Clicked(gtx) || ctrlEnter(gtx, ed)
	body := strings.TrimSpace(ed.Text())
	if submit && body != "" && len(t.Comments) > 0 {
		act.reply = &threadReply{commentID: t.Comments[len(t.Comments)-1].ID, body: body, thread: t.ID}
	}
	busy := v.busyNow() == "reply:"+t.ID
	label := i18n.T("pulls.reviewThread.reply")
	if busy {
		label = i18n.T("pulls.commentBox.sending")
	}
	sendB := th.Button(send, label)
	sendB.Size, sendB.Disabled = ui.SizeXS, body == "" || busy
	cancelB := th.Button(cancel, i18n.T("common.action.cancel"))
	cancelB.Size, cancelB.Variant = ui.SizeXS, ui.VariantGhost
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(ed, i18n.T("pulls.reviewThread.replyPlaceholder"))
			ta.Resize = &s.fieldOf(t.ID).resize
			return ta.Layout(gtx)
		}),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(sendB.Layout), ui.Gap(2), layout.Rigid(cancelB.Layout))
		}),
	)
}

func (v *pullView) resolvedToggle(gtx C, th *ui.Theme, n int) D {
	s := &v.talk
	label := i18n.T("pulls.pullsConversation.resolved", i18n.Args{"count": n})
	chevron := icPullRight
	if s.showResolved.Value {
		chevron = icPullDown
	}
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(4)}.Layout(gtx, func(gtx C) D {
		return s.resolvedTap.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
				ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextXS, label).Muted().Layout),
			)
		})
	})
}

// commentBox is the box for a comment on the pull request as a whole.
func (v *pullView) commentBox(gtx C, th *ui.Theme, act *pullsAction) D {
	s := &v.talk
	body := strings.TrimSpace(s.comment.Text())
	if (s.commentSend.Clicked(gtx) || ctrlEnter(gtx, &s.comment)) && body != "" {
		act.comment = body
	}
	busy := v.busyNow() == "comment"
	label := i18n.T("pulls.pullsConversation.comment")
	if busy {
		label = i18n.T("pulls.commentBox.sending")
	}
	send := th.Button(&s.commentSend, label)
	send.Size, send.Disabled = ui.SizeSM, body == "" || busy
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(5), Bottom: ui.Space(5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sectionHeading(th, i18n.T("pulls.pullsConversation.commentOn"))),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D {
				ta := th.Textarea(&s.comment, i18n.T("pulls.pullsConversation.commentPlaceholder"))
				ta.Resize = &s.commentSize
				return ta.Layout(gtx)
			}),
			ui.VGap(2),
			layout.Rigid(send.Layout),
		)
	})
}

// pendingRows is the review being written: each line comment, discardable,
// until the review is submitted.
func (v *pullView) pendingRows(th *ui.Theme, r *pendingReview) []layout.Widget {
	comments := r.snapshot()
	if len(comments) == 0 {
		return nil
	}
	rows := []layout.Widget{func(gtx C) D {
		return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(sectionHeading(th, i18n.T("pulls.pullsConversation.pendingHead"))),
				ui.Gap(2),
				layout.Rigid(th.Text(ui.TextXS, i18n.T("pulls.pullsConversation.pendingNote")).Muted().Layout),
			)
		})
	}}
	for i, c := range comments {
		rows = append(rows, func(gtx C) D { return v.pendingRow(gtx, th, i, c) })
	}
	return rows
}

// pendingRow is one line comment of the review being written, with Edit and
// Discard, or the box rewording it.
func (v *pullView) pendingRow(gtx C, th *ui.Theme, i int, c lichclient.ReviewComment) D {
	s := &v.talk
	discard, edit := clickableAt(s.discards, i), clickableAt(s.edits, i)
	editing := s.pendingEdit.open && s.pendingEdit.index == i
	body := func(gtx C) D { return wrapped(gtx, th.Text(ui.TextSM, c.Body)) }
	if editing {
		body = func(gtx C) D { return v.pendingEditBox(gtx, th) }
	}
	editB := th.Button(edit, i18n.T("pulls.pullsConversation.edit"))
	editB.Size, editB.Variant = ui.SizeXS, ui.VariantGhost
	discardB := th.Button(discard, i18n.T("pulls.pullsConversation.discard"))
	discardB.Size, discardB.Variant = ui.SizeXS, ui.VariantGhost
	anchor := lineAnchor(c.StartLine, c.Line)
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextXS, c.Path+" · "+anchor).Mono().Muted().Layout),
					layout.Rigid(body),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if editing {
					return D{}
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(editB.Layout), ui.Gap(1), layout.Rigid(discardB.Layout))
			}),
		)
	})
}

// pendingEditBox is the reply box's shape over the comment being reworded;
// pendingEdit.clicks carries out its buttons.
func (v *pullView) pendingEditBox(gtx C, th *ui.Theme) D {
	e := &v.talk.pendingEdit
	saveB := th.Button(&e.save, i18n.T("pulls.pullsConversation.save"))
	saveB.Size, saveB.Disabled = ui.SizeXS, strings.TrimSpace(e.editor.Text()) == ""
	cancelB := th.Button(&e.cancel, i18n.T("common.action.cancel"))
	cancelB.Size, cancelB.Variant = ui.SizeXS, ui.VariantGhost
	return layout.Inset{Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				ta := th.Textarea(&e.editor, "")
				ta.Resize = &e.resize
				return ta.Layout(gtx)
			}),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(saveB.Layout), ui.Gap(2), layout.Rigid(cancelB.Layout))
			}),
		)
	})
}

func (v *pullView) pendingOf(url string) *pendingReview {
	r, ok := v.pending[url]
	if !ok {
		r = new(pendingReview)
		v.pending[url] = r
	}
	return r
}
