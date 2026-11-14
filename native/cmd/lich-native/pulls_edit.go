package main

import (
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icPullEdit = icons.Lucide("pencil")

// editBusy is the busy key of a save, the title and the description sent
// one after the other.
const editBusy = "edit"

// editState is the edit of a pull request's title and description together,
// open on one pull request at a time: the URL it was opened on, so switching
// PRs never carries a draft over.
type editState struct {
	url        string
	title      widget.Editor
	body       widget.Editor
	bodyBar    widget.Scrollbar
	previewing bool

	open    ui.IconActionState
	preview ui.IconActionState
	save    widget.Clickable
	cancel  widget.Clickable
	more    widget.Clickable
	menu    ui.MenuState

	// heldTitle and heldBody are what was sent, shown until a read caught up.
	heldTitle, heldBody heldEdit
}

// heldEdit is a value sent to GitHub, shown over what the screen read until a
// read reflects it: without it the old text is back on screen between the
// write landing and the re-read its PRChangedEvent sets off.
type heldEdit struct {
	url, value string
	since      time.Time
	on         bool
}

func (h *heldEdit) hold(url, value string, now time.Time) {
	*h = heldEdit{url: url, value: value, since: now, on: true}
}

func (h *heldEdit) release() { h.on = false }

// over is what the screen shows for current, a field of the pull request at
// url as read. The read catches up once it equals the value sent, or once one
// has come back from a fetch begun after a stale that followed the send: a
// value GitHub normalised never equals what was sent, and must not stay held.
func (h *heldEdit) over(url, current string, read pullsRead[*backend.PRDetail], staledAt time.Time) string {
	if !h.on || h.url != url {
		return current
	}
	fresh := staledAt.After(h.since) && read.at.After(staledAt) && !read.stale && !read.loading
	if current == h.value || fresh {
		h.release()
		return current
	}
	return h.value
}

// shown is d with the title and description being sent in place of the ones
// read.
func (s *editState) shown(d backend.PRDetail, read pullsRead[*backend.PRDetail], staledAt time.Time) backend.PRDetail {
	d.Title = s.heldTitle.over(d.URL, d.Title, read, staledAt)
	d.Body = s.heldBody.over(d.URL, d.Body, read, staledAt)
	return d
}

// editing reports whether the edit is open on d.
func (s *editState) editing(d backend.PRDetail) bool { return s.url != "" && s.url == d.URL }

// start opens the edit on d's title and description as they stand, the caret
// at the title's end.
func (s *editState) start(d backend.PRDetail) {
	s.url, s.previewing = d.URL, false
	s.title.SetText(d.Title)
	s.title.SetCaret(s.title.Len(), s.title.Len())
	s.body.SetText(d.Body)
}

func (s *editState) close() { s.url = "" }

// changes is what saving sends: the title when it became something else than
// blank, as the session card's rename keeps a blank one, and the description
// when it changed at all, emptied included.
func (s *editState) changes(d backend.PRDetail) (title string, body *string) {
	if t := strings.TrimSpace(s.title.Text()); t != "" && t != d.Title {
		title = t
	}
	if b := s.body.Text(); b != d.Body {
		body = &b
	}
	return title, body
}

// submit hands what changed to act, held on screen until a read has it; with
// nothing changed the edit just closes.
func (s *editState) submit(d backend.PRDetail, now time.Time, act *pullsAction) {
	title, body := s.changes(d)
	if title == "" && body == nil {
		s.close()
		return
	}
	if title != "" {
		act.title = title
		s.heldTitle.hold(d.URL, title, now)
	}
	if body != nil {
		act.body = body
		s.heldBody.hold(d.URL, *body, now)
	}
}

// editClicks carries out the pencil's, Cancel's and Save's clicks. The title
// row reads them before it lays out the actions, so the frame that took one
// already draws the header it leads to.
func (v *pullView) editClicks(gtx C, d backend.PRDetail, act *pullsAction) {
	s := &v.edit
	if s.open.Click.Clicked(gtx) {
		s.start(d)
		v.showTab(d, tabOverview)
		gtx.Execute(keyFocus(&s.title))
	}
	busy := v.busyNow() == editBusy
	if s.cancel.Clicked(gtx) && !busy {
		s.close()
	}
	if s.save.Clicked(gtx) && !busy {
		s.submit(d, gtx.Now, act)
	}
}

// pullTitle is the title with the pencil that opens the edit right after its
// last word, or the field editing it.
func (v *pullView) pullTitle(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	s := &v.edit
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			if s.editing(d) {
				return v.titleField(gtx, th, d, act)
			}
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Max.X = max(gtx.Constraints.Max.X-gtx.Dp(ui.Space(7)), 0)
					l := th.Text(pullsTitleSize, d.Title).Weight(font.SemiBold)
					l.MaxLines = 0
					return l.Layout(gtx)
				}),
				ui.Gap(1),
				layout.Rigid(func(gtx C) D {
					return th.IconAction(p.root, &s.open, icPullEdit, i18n.T("pulls.pullRequestView.edit"), false).Layout(gtx)
				}),
			)
		}),
	)
}

// titleField is the title being edited. Enter and Tab go on to the
// description, Ctrl+Enter saves; Escape does nothing, so a stray one never
// throws a long description away.
func (v *pullView) titleField(gtx C, th *ui.Theme, d backend.PRDetail, act *pullsAction) D {
	s := &v.edit
	busy := v.busyNow() == editBusy
	if ctrlEnter(gtx, &s.title) && !busy {
		s.submit(d, gtx.Now, act)
	}
	// Editor.Layout drains its own events, so the Enter has to be read first.
	s.title.Submit = true
	next := false
	for {
		ev, ok := s.title.Update(gtx)
		if !ok {
			break
		}
		if _, submit := ev.(widget.SubmitEvent); submit {
			next = true
		}
	}
	if _, tab := gtx.Event(key.Filter{Focus: &s.title, Name: key.NameTab}); tab {
		next = true
	}
	if next {
		s.previewing = false
		v.showTab(d, tabOverview)
		gtx.Execute(keyFocus(&s.body))
	}
	in := th.Input(&s.title, "")
	in.TextSize, in.Disabled = pullsTitleSize, busy
	return in.Layout(gtx)
}

// editActions is Cancel and Save in the place of the header's actions while
// the edit is open: the header never scrolls, so Save stays on screen however
// long the description.
func (v *pullView) editActions(gtx C, th *ui.Theme) D {
	s := &v.edit
	busy := v.busyNow() == editBusy
	cancel := th.Button(&s.cancel, i18n.T("common.action.cancel"))
	cancel.Variant, cancel.Size, cancel.Disabled = ui.VariantOutline, ui.SizeSM, busy
	label := i18n.T("pulls.pullsOverview.save")
	if busy {
		label = i18n.T("pulls.pullsOverview.saving")
	}
	save := th.Button(&s.save, label)
	save.Size, save.Disabled = ui.SizeSM, busy
	return ui.Wrap{GapX: ui.Space(2), GapY: ui.Space(2), Justify: ui.JustifyEnd}.Layout(gtx,
		ui.WrapItem(cancel.Layout), ui.WrapItem(save.Layout))
}

// descriptionHeading is the Description heading, with the ⋯ that copies the
// description, or while it is edited the toggle that previews it.
func (v *pullView) descriptionHeading(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	s := &v.edit
	heading := th.SectionHeading(i18n.T("pulls.pullsOverview.description")).Layout
	if s.editing(d) {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, heading),
			layout.Rigid(func(gtx C) D { return v.previewToggle(gtx, th, p) }),
		)
	}
	var copied string
	entries := []ui.MenuEntry{
		{Label: i18n.T("pulls.pullsOverview.copyLink"), OnClick: func() { copied = d.URL }},
		{Label: i18n.T("pulls.pullsOverview.copyMarkdown"), Disabled: strings.TrimSpace(d.Body) == "", OnClick: func() { copied = d.Body }},
	}
	b := th.Button(&s.more, "")
	b.Icon, b.Variant, b.Size = icPullMore, ui.VariantGhost, ui.SizeIconXS
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, heading),
		layout.Rigid(func(gtx C) D {
			return v.withTip(gtx, th, p, "description", i18n.T("pulls.reviewThread.more"), func(gtx C) D {
				return th.DropdownMenu(p.root, &s.menu, entries).Layout(gtx, b.Layout)
			})
		}),
	)
	if copied != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(copied))})
	}
	return dims
}

// previewToggle swaps the description's editor for its Markdown rendered and
// back, as a Markdown file's card does in the Review.
func (v *pullView) previewToggle(gtx C, th *ui.Theme, p *pullsScreen) D {
	s := &v.edit
	if s.preview.Click.Clicked(gtx) {
		s.previewing = !s.previewing
		if !s.previewing {
			gtx.Execute(keyFocus(&s.body))
		}
	}
	label := i18n.T("common.action.previewMarkdown")
	if s.previewing {
		label = i18n.T("pulls.pullRequestView.edit")
	}
	return th.IconAction(p.root, &s.preview, icMarkdownPreview, label, s.previewing).Layout(gtx)
}

// descriptionEditor is the description being edited, raw Markdown, height
// tall: it scrolls inside rather than growing the page, so the caret never
// leaves the screen. Ctrl+Enter saves.
func (v *pullView) descriptionEditor(gtx C, th *ui.Theme, d backend.PRDetail, act *pullsAction, height int) D {
	s := &v.edit
	busy := v.busyNow() == editBusy
	if ctrlEnter(gtx, &s.body) && !busy {
		s.submit(d, gtx.Now, act)
	}
	gtx.Constraints.Max.Y = height
	h := gtx.Metric.PxToDp(height)
	ta := th.Textarea(&s.body, i18n.T("pulls.pullsOverview.descriptionPlaceholder"))
	ta.Mono, ta.Disabled, ta.Scrollbar = true, busy, &s.bodyBar
	ta.MinHeight, ta.MaxHeight = h, h
	return ta.Layout(gtx)
}

// applyEdit carries the edit to GitHub off the frame, the title first: the
// first refusal stops it and leaves the edit open on what is still to send,
// and the re-read comes back as PRChangedEvent.
func (u *window) applyEdit(path string, number int, act pullsAction) {
	if act.title == "" && act.body == nil {
		return
	}
	v := &u.pulls.view
	// Set before the write starts, so the next frame already shows Save busy.
	v.setBusy(editBusy)
	title, body := act.title, act.body
	failedBody := false
	failed := func(err error) string {
		if failedBody {
			return i18n.T("pulls.pullsOverview.editFailed", i18n.Args{"error": backend.ErrorText(err)})
		}
		return i18n.T("pulls.pullRequestView.renameFailed", i18n.Args{"error": backend.ErrorText(err)})
	}
	go u.prWrite(editBusy, failed, func() error {
		if title != "" {
			if err := u.client.EditPullRequestTitle(u.ctx, path, number, title); err != nil {
				v.onFrame(v.edit.heldTitle.release)
				if body != nil {
					v.onFrame(v.edit.heldBody.release)
				}
				return err
			}
		}
		if body != nil {
			if err := u.client.EditPullRequestBody(u.ctx, path, number, *body); err != nil {
				failedBody = true
				v.onFrame(v.edit.heldBody.release)
				return err
			}
		}
		v.onFrame(v.edit.close)
		return nil
	})
}

// openForShot opens the edit on d for -pulls-edit, previewing the
// description when it says preview.
func (s *editState) openForShot(what string, d backend.PRDetail) {
	s.start(d)
	s.previewing = what == "preview"
}
