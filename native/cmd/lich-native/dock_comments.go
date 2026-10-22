package main

// The review comment batch: line comments written while reading a
// diff or a file, held per checkout until they go to the session as one
// prompt. Every surface reviewing a checkout shares its batch.
//
// API, all on the UI goroutine:
//
//	comments := new(commentStore)                 // one per window
//	comments.add(target, path, lines, text)       // target is the checkout path
//	comments.list(target)                         // what the strip shows
//
//	var c commentComposer                         // one per surface
//	c.open(path, lines)                           // "Comment for the session" chosen on lines "12-30"
//	c.isOpen() / c.at()                           // where to place it (the selection's last new-file line)
//	c.holdsKeys(gtx)                              // keep the terminal off it; ask before layout
//	c.layout(gtx, th, comments, target)           // the box; files into comments on Add, closes itself
//
//	var b commentBatch                            // the strip at a panel's foot
//	b.layout(gtx, th, comments, target, submit, notify)
//	                                              // nothing at all while the batch is empty
//
// submit sends text as the active session's prompt and reports whether a
// session can take it; done reports, from any goroutine, whether it did. The
// sent comments leave the batch only on a nil done. notify raises a toast.

import (
	"fmt"
	"image"
	"path"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icBatchRemove = icons.Lucide("x")
	icBatchSend   = icons.Lucide("send")
)

// reviewComment is a place plus what should change there. path and lines are
// "" for a note about the change as a whole.
type reviewComment struct {
	id               int
	path, lines, txt string
}

// commentStore holds every checkout's batch, in memory only: a comment
// anchors to line numbers of the diff as it was drawn, and nothing tracks the
// file moving underneath, so it must not outlive the run.
type commentStore struct {
	byTarget map[string][]reviewComment
	next     int
}

// add appends a comment; blank text is not a comment and is dropped.
func (s *commentStore) add(target, path, lines, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if s.byTarget == nil {
		s.byTarget = map[string][]reviewComment{}
	}
	s.next++
	s.byTarget[target] = append(s.byTarget[target], reviewComment{id: s.next, path: path, lines: lines, txt: text})
}

// addNote is a remark about the change as a whole, joining the same batch.
func (s *commentStore) addNote(target, text string) { s.add(target, "", "", text) }

func (s *commentStore) remove(target string, id int) {
	s.byTarget[target] = slices.DeleteFunc(s.byTarget[target], func(c reviewComment) bool { return c.id == id })
}

func (s *commentStore) clear(target string) { delete(s.byTarget, target) }

// list is target's batch in the order it was written. The slice is the
// store's: read it, don't keep it across a change.
func (s *commentStore) list(target string) []reviewComment { return s.byTarget[target] }

// composeReviewComments is the batch as one prompt: a markdown list, each item
// anchored the way an inject writes a line reference.
func composeReviewComments(list []reviewComment, note string) string {
	items := make([]string, 0, len(list)+1)
	for _, c := range list {
		if c.path == "" {
			items = append(items, "- "+continuation(c.txt))
			continue
		}
		items = append(items, fmt.Sprintf("- %s:%s — %s", c.path, c.lines, continuation(c.txt)))
	}
	if note = strings.TrimSpace(note); note != "" {
		items = append(items, "- "+continuation(note))
	}
	return i18n.Prompt("prompts.reviewComments.header") + "\n\n" + strings.Join(items, "\n")
}

// continuation keeps a multi-line comment one list item.
func continuation(text string) string { return strings.ReplaceAll(text, "\n", "\n  ") }

// commentComposer is the box a "Comment for the session" opens under the
// selection: Enter adds it to the batch, Shift+Enter is a new line, Escape
// backs out. Opened for the pull request it files into the pending review
// instead, and sends on Ctrl+Enter, Enter being a new line.
type commentComposer struct {
	shown       bool
	review      bool
	path, lines string
	editor      widget.Editor
	resize      ui.TextareaResize
	add, cancel widget.Clickable
	focus       bool
	// toReview files the comment into a pull request's pending review; nil
	// outside the Pulls screen.
	toReview func(path, lines, body string)
}

// open starts a comment on lines ("19", "12-30") of path, dropping one being
// written.
func (c *commentComposer) open(path, lines string) {
	c.shown, c.review, c.path, c.lines, c.focus = true, false, path, lines, true
	c.editor.SetText("")
}

// openReview starts a comment on lines of path for the pull request's
// pending review.
func (c *commentComposer) openReview(path, lines string) {
	c.open(path, lines)
	c.review = true
}

func (c *commentComposer) close() { c.shown = false }

func (c *commentComposer) isOpen() bool { return c.shown }

// holdsKeys is whether the box has the keyboard, or asked for it this frame:
// a window that hands focus to its terminal every frame must not take it
// back before the request lands.
func (c *commentComposer) holdsKeys(gtx C) bool {
	return c.shown && (c.focus || gtx.Focused(&c.editor))
}

// at is the new-file line the box sits under: the selection's last.
func (c *commentComposer) at() int {
	last := c.lines[strings.LastIndexByte(c.lines, '-')+1:]
	n := 0
	fmt.Sscan(last, &n)
	return n
}

// filed reads this frame's input: whether the comment was sent or the box
// dismissed, filing a sent one where the box was opened for.
func (c *commentComposer) filed(gtx C, store *commentStore, target string) {
	send := c.add.Clicked(gtx)
	for {
		ev, ok := c.editor.Update(gtx)
		if !ok {
			break
		}
		if _, is := ev.(widget.SubmitEvent); is {
			send = true
		}
	}
	if c.review && ctrlEnter(gtx, &c.editor) {
		send = true
	}
	if _, ok := gtx.Event(key.Filter{Focus: &c.editor, Name: key.NameEscape}); ok || c.cancel.Clicked(gtx) {
		c.close()
		return
	}
	if send {
		c.commit(store, target)
	}
}

// commit files the comment, unless it is blank: into the pending review for
// a box opened for the pull request, into target's batch otherwise.
func (c *commentComposer) commit(store *commentStore, target string) {
	text := c.editor.Text()
	if strings.TrimSpace(text) == "" {
		return
	}
	if c.review {
		c.toReview(c.path, c.lines, text)
	} else {
		store.add(target, c.path, c.lines, text)
	}
	c.close()
}

// layout draws the box while open; it is zero-sized when closed.
func (c *commentComposer) layout(gtx C, th *ui.Theme, store *commentStore, target string) D {
	if !c.shown {
		return D{}
	}
	c.editor.Submit = !c.review
	c.filed(gtx, store, target)
	if !c.shown {
		return D{}
	}
	if c.focus {
		gtx.Execute(key.FocusCmd{Tag: &c.editor})
		c.focus = false
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	on := th.On(th.Sidebar)
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(on.Text(ui.TextXS, c.heading()).Mono().Muted().Layout),
					ui.VGap(1.5),
					layout.Rigid(c.box(on)),
				)
			})
		},
	)
}

// heading is ReviewSlots' line over the box, which says where the comment is
// going: the two boxes look alike and only one reaches GitHub.
func (c *commentComposer) heading() string {
	if c.review {
		return i18n.T("diff.reviewSlots.onPullRequest", i18n.Args{"lines": c.lines})
	}
	return i18n.T("diff.reviewSlots.forSession", i18n.Args{"lines": c.lines})
}

// box is CommentBox: the field (max-h-64), then Add and Cancel.
func (c *commentComposer) box(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		label, prompt := i18n.T("diff.reviewSlots.addToBatch"), i18n.T("diff.reviewSlots.sessionPlaceholder")
		if c.review {
			label, prompt = i18n.T("diff.reviewSlots.addToReview"), i18n.T("diff.reviewSlots.reviewPlaceholder")
		}
		add := th.Button(&c.add, label)
		add.Size, add.Disabled = ui.SizeSM, strings.TrimSpace(c.editor.Text()) == ""
		cancel := th.Button(&c.cancel, i18n.T("common.action.cancel"))
		cancel.Size, cancel.Variant = ui.SizeSM, ui.VariantGhost
		buttons := []layout.FlexChild{layout.Rigid(add.Layout), ui.Gap(2), layout.Rigid(cancel.Layout)}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(ui.Space(64)))
				ta := th.Textarea(&c.editor, prompt)
				ta.Resize = &c.resize
				return ta.Layout(gtx)
			}),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...) }),
		)
	}
}

// commentBatch is the strip at the foot of a review panel: the comments so
// far, a note about the whole change, and the one action that hands them all
// to the session as one prompt.
type commentBatch struct {
	list        widget.List
	removes     map[int]*widget.Clickable
	note        widget.Editor
	clear, send widget.Clickable
	// sending is the batch on its way to the session, still in the store:
	// the session can refuse it after the send left.
	sending *batchSend
}

// batchSend is one send in flight: what to drop once the session took it.
type batchSend struct {
	target string
	ids    []int
	note   string
	result chan error
}

// batchListMax is the list's max-h-40.
const batchListMax = unit.Dp(160)

// layout draws the strip for target's batch, or nothing while it is empty.
// submit sends the batch as the session's prompt and reports whether one took
// it; the comments are the one thing here the user typed, so they stay until
// the session has taken them.
func (b *commentBatch) layout(gtx C, th *ui.Theme, store *commentStore, target string, submit func(string, func(error)) bool, notify func(ui.Toast)) D {
	b.update(gtx, store, target, submit, notify)
	comments := store.list(target)
	if len(comments) == 0 {
		return D{}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.Y = gtx.Dp(batchListMax)
			return layout.UniformInset(ui.Space(1)).Layout(gtx, func(gtx C) D { return b.rows(gtx, th, comments) })
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1)}.Layout(gtx, b.noteField(th))
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(2)}.Layout(gtx, b.footer(th, len(comments)))
		}),
	)
}

// update runs the strip's clicks against the store before it is drawn.
func (b *commentBatch) update(gtx C, store *commentStore, target string, submit func(string, func(error)) bool, notify func(ui.Toast)) {
	b.note.SingleLine, b.note.Submit = true, true
	for {
		ev, ok := b.note.Update(gtx)
		if !ok {
			break
		}
		if _, is := ev.(widget.SubmitEvent); is {
			store.addNote(target, b.note.Text())
			b.note.SetText("")
		}
	}
	for id, c := range b.removes {
		if c.Clicked(gtx) {
			store.remove(target, id)
			delete(b.removes, id)
		}
	}
	if b.clear.Clicked(gtx) {
		b.drop(store, target)
	}
	b.settle(store)
	if b.send.Clicked(gtx) && b.sending == nil {
		b.sendBatch(store, target, submit, notify)
	}
}

// sendBatch hands target's batch to submit and keeps it in the store until
// settle learns the session took it.
func (b *commentBatch) sendBatch(store *commentStore, target string, submit func(string, func(error)) bool, notify func(ui.Toast)) {
	comments := store.list(target)
	send := &batchSend{target: target, note: b.note.Text(), result: make(chan error, 1)}
	for _, c := range comments {
		send.ids = append(send.ids, c.id)
	}
	if !submit(composeReviewComments(comments, send.note), func(err error) { send.result <- err }) {
		notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.commentBatch.openSession")})
		return
	}
	b.sending = send
}

// settle drops the sent comments once the session took them, and leaves
// them all in place when it refused.
func (b *commentBatch) settle(store *commentStore) {
	if b.sending == nil {
		return
	}
	var err error
	select {
	case err = <-b.sending.result:
	default:
		return
	}
	send := b.sending
	b.sending = nil
	if err != nil {
		return
	}
	for _, id := range send.ids {
		store.remove(send.target, id)
	}
	if b.note.Text() == send.note {
		b.note.SetText("")
	}
}

func (b *commentBatch) drop(store *commentStore, target string) {
	store.clear(target)
	b.note.SetText("")
}

func (b *commentBatch) remove(id int) *widget.Clickable {
	if b.removes == nil {
		b.removes = map[int]*widget.Clickable{}
	}
	c, ok := b.removes[id]
	if !ok {
		c = new(widget.Clickable)
		b.removes[id] = c
	}
	return c
}

func (b *commentBatch) rows(gtx C, th *ui.Theme, comments []reviewComment) D {
	b.list.Axis = layout.Vertical
	return th.ScrollArea(&b.list).Layout(gtx, len(comments), func(gtx C, i int) D {
		return b.row(gtx, th, comments[i])
	})
}

// row is one comment: where (mono, muted), what (truncated), and its ×.
func (b *commentBatch) row(gtx C, th *ui.Theme, c reviewComment) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	remove := th.Button(b.remove(c.id), "")
	remove.Icon, remove.Variant, remove.Size = icBatchRemove, ui.VariantGhost, ui.SizeIconXS
	return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if c.path == "" {
					return D{}
				}
				return layout.Inset{Right: ui.Space(2)}.Layout(gtx,
					th.Text(ui.TextXS, path.Base(c.path)+":"+c.lines).Mono().Muted().Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				l := th.Text(ui.TextXS, strings.ReplaceAll(c.txt, "\n", " "))
				l.MaxLines = 1
				return l.Layout(gtx)
			}),
			ui.Gap(2),
			layout.Rigid(remove.Layout),
		)
	})
}

// noteField is the note input; Enter adds the note to the batch.
func (b *commentBatch) noteField(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		in := th.Input(&b.note, i18n.T("diff.commentBatch.notePlaceholder"))
		in.Height, in.TextSize = ui.Space(7), ui.TextXS
		return in.Layout(gtx)
	}
}

// footer is the count, then Clear and Send.
func (b *commentBatch) footer(th *ui.Theme, count int) layout.Widget {
	return func(gtx C) D {
		clear := th.Button(&b.clear, i18n.T("diff.commentBatch.clear"))
		clear.Variant, clear.Size = ui.VariantGhost, ui.SizeSM
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, i18n.T("diff.commentBatch.count", i18n.Args{"count": count})).Muted().Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(clear.Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return b.sendButton(gtx, th) }),
		)
	}
}

// sendButtonFill is the accent's alpha on the Send button at rest.
const sendButtonFill = 0.55

func (b *commentBatch) sendButton(gtx C, th *ui.Theme) D {
	send := th.Button(&b.send, i18n.T("diff.commentBatch.send"))
	send.Icon, send.Variant, send.Size = icBatchSend, ui.VariantGhost, ui.SizeSM
	if b.send.Hovered() {
		return send.Layout(gtx)
	}
	fill := th.Over(ui.Alpha(th.Accent, sendButtonFill))
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
		send.Layout,
	)
}
