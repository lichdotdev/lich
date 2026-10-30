package main

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icPullMore = icons.Lucide("ellipsis")

// commentKind is which of GitHub's three addresses a conversation entry's
// writes go to.
type commentKind uint8

const (
	threadComment commentKind = iota
	prComment
	reviewSummary
)

// commentTarget is what an entry's ⋯ acts on, with what GitHub lets the viewer
// do to it. lastInThread marks the thread comment whose delete takes the
// thread with it.
type commentTarget struct {
	kind         commentKind
	id           int64
	body         string
	canEdit      bool
	canDelete    bool
	lastInThread bool
}

// key addresses the target's widgets in the maps a thread's live in; a thread
// id never takes this shape.
func (t commentTarget) key() string { return fmt.Sprintf("%d:%d", t.kind, t.id) }

// commentEdit is an edit's write, carried to the window.
type commentEdit struct {
	target commentTarget
	body   string
}

func threadTarget(t backend.PRReviewThread, c backend.PRThreadComment) commentTarget {
	return commentTarget{
		kind: threadComment, id: c.ID, body: c.Body,
		canEdit: c.CanEdit, canDelete: c.CanDelete, lastInThread: len(t.Comments) == 1,
	}
}

// commentEntries is the ⋯ menu: Edit and Delete as far as GitHub allows them.
func (s *talkState) commentEntries(t commentTarget) []ui.MenuEntry {
	var entries []ui.MenuEntry
	if t.canEdit {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("pulls.reviewThread.edit"), OnClick: func() { s.startEdit(t) }})
	}
	if t.canDelete {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("pulls.reviewThread.delete"), Variant: ui.VariantDestructive, OnClick: func() {
			target := t
			s.deleting = &target
		}})
	}
	return entries
}

// isEditing takes the entry's Cancel and answers whether it is still being
// edited; the click is taken before anything reads the state, so the frame
// that took it draws the result.
func (s *talkState) isEditing(gtx C, t commentTarget) bool {
	if clickableOf(s.cancelClicks, t.key()).Clicked(gtx) {
		s.editing[t.key()] = false
	}
	return s.editing[t.key()]
}

func (s *talkState) startEdit(t commentTarget) {
	s.editing[t.key()] = true
	s.editorOf(t.key()).SetText(t.body)
	s.focusEdit = t.key()
}

// commentHead is an entry's byline with the ⋯ beside it, which is gone while
// the entry is being edited.
func (v *pullView) commentHead(gtx C, th *ui.Theme, p *pullsScreen, t commentTarget, byline layout.Widget) D {
	s := &v.talk
	entries := s.commentEntries(t)
	if len(entries) == 0 || s.editing[t.key()] {
		return byline(gtx)
	}
	more := func(gtx C) D {
		b := th.Button(clickableOf(s.moreClicks, t.key()), "")
		b.Icon, b.Variant, b.Size = icPullMore, ui.VariantGhost, ui.SizeIconXS
		return v.withTip(gtx, th, p, "more:"+t.key(), i18n.T("pulls.reviewThread.more"), b.Layout)
	}
	menu, ok := s.menus[t.key()]
	if !ok {
		menu = new(ui.MenuState)
		s.menus[t.key()] = menu
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, byline),
		layout.Rigid(func(gtx C) D { return th.DropdownMenu(p.root, menu, entries).Layout(gtx, more) }),
	)
}

// threadComment is one comment of a thread beside its author's avatar: its
// byline over its body, or over the box editing it.
func (v *pullView) threadComment(gtx C, th *ui.Theme, p *pullsScreen, t commentTarget, c backend.PRThreadComment, act *pullsAction, now time.Time) D {
	editing := v.talk.editing[t.key()]
	return p.withAvatar(gtx, th, author{c.Author, c.Avatar}, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return v.commentHead(gtx, th, p, t, func(gtx C) D { return byline(gtx, th, c.Author, "", c.Date, now) })
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
					if editing {
						return v.editBox(gtx, th, t, act)
					}
					return wrapped(gtx, th.Text(ui.TextSM, c.Body))
				})
			}),
		)
	})
}

// editBox rewrites an entry in place; Save sends it as it stands. A review's
// verdict is not part of what GitHub lets an edit change, and the box says so.
func (v *pullView) editBox(gtx C, th *ui.Theme, t commentTarget, act *pullsAction) D {
	s := &v.talk
	key := t.key()
	ed := s.editorOf(key)
	if s.focusEdit == key {
		gtx.Execute(keyFocus(ed))
		s.focusEdit = ""
	}
	save, cancel := clickableOf(s.sendClicks, key), clickableOf(s.cancelClicks, key)
	body := strings.TrimSpace(ed.Text())
	if (save.Clicked(gtx) || ctrlEnter(gtx, ed)) && body != "" {
		act.edit = &commentEdit{target: t, body: body}
	}
	busy := v.busyNow() == "edit:"+key
	label := i18n.T("pulls.reviewThread.save")
	if busy {
		label = i18n.T("pulls.reviewThread.saving")
	}
	saveB := th.Button(save, label)
	saveB.Size, saveB.Disabled = ui.SizeXS, body == "" || busy
	cancelB := th.Button(cancel, i18n.T("common.action.cancel"))
	cancelB.Size, cancelB.Variant = ui.SizeXS, ui.VariantGhost
	buttons := []layout.FlexChild{layout.Rigid(saveB.Layout), ui.Gap(2), layout.Rigid(cancelB.Layout)}
	if t.kind == reviewSummary {
		buttons = append(buttons, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, i18n.T("pulls.reviewThread.verdictStays")).Muted().Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(ed, "")
			ta.Resize = &s.fieldOf(key).resize
			return ta.Layout(gtx)
		}),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...) }),
	)
}

// editComment sends an edit to the address its kind lives at; the re-read
// comes back as PRChangedEvent.
func (u *window) editComment(path string, number int, e commentEdit) {
	v := &u.pulls.view
	key := e.target.key()
	u.prWrite("edit:"+key, func(err error) string {
		return i18n.T("pulls.reviewThread.editFailed", i18n.Args{"error": backend.ErrorText(err)})
	}, func() error {
		var err error
		switch e.target.kind {
		case threadComment:
			err = u.client.EditReviewComment(u.ctx, path, number, e.target.id, e.body)
		case prComment:
			err = u.client.EditPullRequestComment(u.ctx, path, number, e.target.id, e.body)
		case reviewSummary:
			err = u.client.EditReviewBody(u.ctx, path, number, e.target.id, e.body)
		}
		if err != nil {
			return err
		}
		v.onFrame(func() { v.talk.editing[key] = false })
		return nil
	})
}

// deleteComment removes a thread comment or a pull request comment; a review
// has no delete.
func (u *window) deleteComment(path string, number int, t commentTarget) {
	u.prWrite("delete:"+t.key(), func(err error) string {
		return i18n.T("pulls.reviewThread.deleteFailed", i18n.Args{"error": backend.ErrorText(err)})
	}, func() error {
		if t.kind == prComment {
			return u.client.DeletePullRequestComment(u.ctx, path, number, t.id)
		}
		return u.client.DeleteReviewComment(u.ctx, path, number, t.id)
	})
}

// layoutDeleteCommentDialog is the delete confirmation, laid out by the window
// ahead of its content so it takes Escape first.
func (u *window) layoutDeleteCommentDialog(gtx C, th *ui.Theme) {
	p := u.pulls
	s := &p.view.talk
	if s.deleting == nil && !s.deleteDialog.Animating(gtx) {
		return
	}
	if s.deleting != nil {
		s.deleteShown = *s.deleting
	}
	t := s.deleteShown
	switch {
	case s.deleteYes.Clicked(gtx) && s.deleting != nil && p.selected != 0:
		s.deleting = nil
		go u.deleteComment(p.project.Path, p.selected, t)
	case s.deleteNo.Clicked(gtx):
		s.deleting = nil
	}
	description := i18n.T("pulls.reviewThread.deleteGone")
	if t.lastInThread {
		description = i18n.T("pulls.reviewThread.deleteLast")
	}
	pop := th.On(th.Popover)
	d := th.Dialog(&s.deleteDialog, s.deleting != nil, i18n.T("pulls.reviewThread.deleteTitle"), description)
	confirm := pop.Button(&s.deleteYes, i18n.T("pulls.reviewThread.delete"))
	confirm.Variant = ui.VariantDestructive
	back := pop.Button(&s.deleteNo, i18n.T("common.action.cancel"))
	back.Variant = ui.VariantOutline
	d.Actions = []layout.Widget{back.Layout, confirm.Layout}
	d.Root = &u.root
	if d.Layout(gtx) {
		s.deleting = nil
	}
}
