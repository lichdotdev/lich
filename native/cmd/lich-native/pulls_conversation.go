package main

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (pulls.pullsConversation,
// pulls.reviewThread).
const (
	talkLoading       = "Reading the conversation…"
	talkFailed        = "Couldn’t read the conversation: %s"
	talkEmpty         = "Nothing said yet."
	talkResolvedOne   = "1 resolved thread"
	talkResolvedMany  = "%d resolved threads"
	talkCommentOn     = "Comment on the pull request"
	talkCommentHint   = "Something about the pull request as a whole"
	talkComment       = "Comment"
	talkSending       = "Sending…"
	talkLine          = "line %d"
	talkLines         = "lines %d-%d"
	talkOutdated      = "outdated"
	talkResolved      = "Resolved"
	talkResolve       = "Resolve"
	talkReopen        = "Reopen"
	talkReply         = "Reply"
	talkReplyHint     = "Reply to this thread"
	talkCancel        = "Cancel"
	talkPendingHead   = "Pending review"
	talkPendingNote   = "Sent when you submit the review."
	talkDiscard       = "Discard"
	talkSomeone       = "someone"
	talkVerdictFormat = "%s %s"
)

var talkVerdicts = map[string]string{
	"APPROVED": "approved", "CHANGES_REQUESTED": "requested changes", "COMMENTED": "commented", "DISMISSED": "review dismissed",
}

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
}

func (s *talkState) init() {
	s.folds, s.foldClicks = map[string]*widget.Bool{}, map[string]*widget.Clickable{}
	s.replying, s.replyFields = map[string]bool{}, map[string]*replyField{}
	s.replyClicks, s.sendClicks = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	s.cancelClicks, s.resolveClick = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	s.discards = map[int]*widget.Clickable{}
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

// conversationRows is PullsConversation: the pending review, the timeline,
// the resolved threads behind their count, and the box for a comment on the
// pull request itself.
func (v *pullView) conversationRows(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) []layout.Widget {
	read := p.store.conversation(p.project.Path, d.Number)
	switch {
	case read.val == nil && read.err != "":
		return []layout.Widget{func(gtx C) D { return panelNotice(gtx, th, fmt.Sprintf(talkFailed, read.err)) }}
	case read.val == nil:
		return []layout.Widget{func(gtx C) D { return panelNotice(gtx, th, talkLoading) }}
	}
	items, resolved := conversationTimeline(read.val)
	now := gtx.Now
	rows := v.pendingRows(th, v.pendingOf(d.URL))
	if len(items) == 0 && len(resolved) == 0 {
		rows = append(rows, func(gtx C) D { return panelNotice(gtx, th, talkEmpty) })
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
	if v, ok := talkVerdicts[r.State]; ok {
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
		author = talkSomeone
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
	anchor := fmt.Sprintf(talkLine, t.Line)
	if t.StartLine > 0 && t.StartLine != t.Line {
		anchor = fmt.Sprintf(talkLines, t.StartLine, t.Line)
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
		ui.Gap(1.5),
		layout.Flexed(1, func(gtx C) D {
			return truncated(gtx, th.Text(ui.TextXS, t.Path+" · "+anchor).Mono())
		}),
	}
	if t.IsOutdated {
		children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, talkOutdated).In(th.ToneWait).Layout))
	}
	if t.IsResolved {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return metaGlyph(gtx, th, icPullCheck, talkResolved, th.TonePass) }))
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
	if s.replying[t.ID] {
		return v.replyBox(gtx, th, d, t, act)
	}
	busy := v.busyNow()
	label := talkResolve
	if t.IsResolved {
		label = talkReopen
	}
	replyB := th.Button(reply, talkReply)
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
	if cancel.Clicked(gtx) {
		s.replying[t.ID] = false
	}
	body := strings.TrimSpace(ed.Text())
	if submit && body != "" && len(t.Comments) > 0 {
		act.reply = &threadReply{commentID: t.Comments[len(t.Comments)-1].ID, body: body, thread: t.ID}
	}
	busy := v.busyNow() == "reply:"+t.ID
	label := talkReply
	if busy {
		label = talkSending
	}
	sendB := th.Button(send, label)
	sendB.Size, sendB.Disabled = ui.SizeXS, body == "" || busy
	cancelB := th.Button(cancel, talkCancel)
	cancelB.Size, cancelB.Variant = ui.SizeXS, ui.VariantGhost
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(ed, talkReplyHint)
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
	if s.resolvedTap.Clicked(gtx) {
		s.showResolved.Value = !s.showResolved.Value
	}
	label := talkResolvedOne
	if n != 1 {
		label = fmt.Sprintf(talkResolvedMany, n)
	}
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
	label := talkComment
	if busy {
		label = talkSending
	}
	send := th.Button(&s.commentSend, label)
	send.Size, send.Disabled = ui.SizeSM, body == "" || busy
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(5), Bottom: ui.Space(5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sectionHeading(th, talkCommentOn)),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D {
				ta := th.Textarea(&s.comment, talkCommentHint)
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
				layout.Rigid(sectionHeading(th, talkPendingHead)),
				ui.Gap(2),
				layout.Rigid(th.Text(ui.TextXS, talkPendingNote).Muted().Layout),
			)
		})
	}}
	for i, c := range comments {
		discard := v.talk.discards[i]
		if discard == nil {
			discard = new(widget.Clickable)
			v.talk.discards[i] = discard
		}
		rows = append(rows, func(gtx C) D {
			if discard.Clicked(gtx) {
				r.remove(i)
			}
			anchor := fmt.Sprintf(talkLine, c.Line)
			if c.StartLine > 0 {
				anchor = fmt.Sprintf(talkLines, c.StartLine, c.Line)
			}
			b := th.Button(discard, talkDiscard)
			b.Size, b.Variant = ui.SizeXS, ui.VariantGhost
			return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(2)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(th.Text(ui.TextXS, c.Path+" · "+anchor).Mono().Muted().Layout),
							layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextSM, c.Body)) }),
						)
					}),
					layout.Rigid(b.Layout),
				)
			})
		})
	}
	return rows
}

func (v *pullView) pendingOf(url string) *pendingReview {
	r, ok := v.pending[url]
	if !ok {
		r = new(pendingReview)
		v.pending[url] = r
	}
	return r
}
