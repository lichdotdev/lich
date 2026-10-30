package main

import (
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icPullEdit = icons.Lucide("pencil")

// pullsDescriptionMaxHeight is max-h-80: a long description scrolls inside
// the field and keeps Save on screen.
var pullsDescriptionMaxHeight = ui.Space(80)

// editState is the title and description edits, each open on one pull request
// at a time: the URL it was opened on, so switching PRs never carries a draft
// over.
type editState struct {
	titleURL  string
	title     ui.InlineEdit
	titleOpen iconActionState

	bodyURL    string
	body       widget.Editor
	bodySize   ui.TextareaResize
	bodyBar    widget.Scrollbar
	bodyMore   widget.Clickable
	bodyMenu   ui.MenuState
	bodySave   widget.Clickable
	bodyCancel widget.Clickable

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

// titleEditing reports whether the title field is open on d.
func (s *editState) titleEditing(d backend.PRDetail) bool {
	return s.title.Active() && s.titleURL == d.URL
}

// pullTitle is the number and the title, or the field renaming it, with the
// pencil that opens the field right after the title's last word. Enter or
// leaving the field saves, Escape cancels, as a session card's rename.
func (v *pullView) pullTitle(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	s := &v.edit
	if s.titleOpen.click.Clicked(gtx) {
		s.titleURL = d.URL
		s.title.Start(d.Title)
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			if s.titleEditing(d) {
				dims, res := th.InlineEdit(&s.title, pullsTitleSize).Weight(font.SemiBold).Layout(gtx)
				if res.Outcome == ui.InlineEditCommitted {
					act.title = res.Value
					s.heldTitle.hold(d.URL, res.Value, gtx.Now)
				}
				return dims
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
					return iconAction(gtx, th, p.root, &s.titleOpen, icPullEdit, i18n.T("pulls.pullRequestView.editTitle"), false)
				}),
			)
		}),
	)
}

// descriptionHeading is the Description heading, with the ⋯ that copies the
// description or turns it into a field while it is not one already.
func (v *pullView) descriptionHeading(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	s := &v.edit
	// Cancel is the field's, drawn under the heading: taken here, the
	// heading and the field both read it.
	if s.bodyCancel.Clicked(gtx) && v.busyNow() != "body" {
		s.bodyURL = ""
	}
	heading := sectionHeading(th, i18n.T("pulls.pullsOverview.description"))
	if s.bodyURL == d.URL {
		return heading(gtx)
	}
	var copied string
	entries := []ui.MenuEntry{
		{Label: i18n.T("pulls.pullsOverview.copyLink"), OnClick: func() { copied = d.URL }},
		{Label: i18n.T("pulls.pullsOverview.copyMarkdown"), Disabled: strings.TrimSpace(d.Body) == "", OnClick: func() { copied = d.Body }},
		{Kind: ui.MenuSeparator},
		{Label: i18n.T("pulls.pullsOverview.editDescription"), OnClick: func() {
			s.bodyURL = d.URL
			s.body.SetText(d.Body)
		}},
	}
	b := th.Button(&s.bodyMore, "")
	b.Icon, b.Variant, b.Size = icPullMore, ui.VariantGhost, ui.SizeIconXS
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, heading),
		layout.Rigid(func(gtx C) D {
			return v.withTip(gtx, th, p, "description", i18n.T("pulls.reviewThread.more"), func(gtx C) D {
				return th.DropdownMenu(p.root, &s.bodyMenu, entries).Layout(gtx, b.Layout)
			})
		}),
	)
	if copied != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(copied))})
	}
	if s.bodyURL == d.URL {
		gtx.Execute(keyFocus(&s.body))
	}
	return dims
}

// descriptionField is the description being edited, raw Markdown, saved with
// Save or Ctrl+Enter. It stays open, saying so, until GitHub took it.
func (v *pullView) descriptionField(gtx C, th *ui.Theme, d backend.PRDetail, act *pullsAction) D {
	s := &v.edit
	busy := v.busyNow() == "body"
	if (s.bodySave.Clicked(gtx) || ctrlEnter(gtx, &s.body)) && !busy {
		body := s.body.Text()
		act.body = &body
		s.heldBody.hold(d.URL, body, gtx.Now)
	}
	cancel := th.Button(&s.bodyCancel, i18n.T("common.action.cancel"))
	cancel.Variant, cancel.Size, cancel.Disabled = ui.VariantOutline, ui.SizeSM, busy
	label := i18n.T("pulls.pullsOverview.save")
	if busy {
		label = i18n.T("pulls.pullsOverview.saving")
	}
	save := th.Button(&s.bodySave, label)
	save.Size, save.Disabled = ui.SizeSM, busy
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(&s.body, i18n.T("pulls.pullsOverview.descriptionPlaceholder"))
			ta.Resize, ta.Disabled = &s.bodySize, busy
			ta.MaxHeight, ta.Scrollbar = pullsDescriptionMaxHeight, &s.bodyBar
			return ta.Layout(gtx)
		}),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx, layout.Rigid(cancel.Layout), ui.Gap(2), layout.Rigid(save.Layout))
		}),
	)
}

// applyEdit carries the title and description edits to GitHub off the frame;
// the re-read comes back as PRChangedEvent.
func (u *window) applyEdit(path string, number int, act pullsAction) {
	v := &u.pulls.view
	if act.title != "" {
		title := act.title
		go u.prWrite("title", func(err error) string {
			return i18n.T("pulls.pullRequestView.renameFailed", i18n.Args{"error": backend.ErrorText(err)})
		}, func() error {
			if err := u.client.EditPullRequestTitle(u.ctx, path, number, title); err != nil {
				v.onFrame(v.edit.heldTitle.release)
				return err
			}
			return nil
		})
	}
	if act.body != nil {
		body := *act.body
		go u.prWrite("body", func(err error) string {
			return i18n.T("pulls.pullsOverview.editFailed", i18n.Args{"error": backend.ErrorText(err)})
		}, func() error {
			if err := u.client.EditPullRequestBody(u.ctx, path, number, body); err != nil {
				v.onFrame(v.edit.heldBody.release)
				return err
			}
			v.onFrame(func() { v.edit.bodyURL = "" })
			return nil
		})
	}
}

// openForShot opens the edit -pulls-edit names (title, description) on d.
func (s *editState) openForShot(what string, d backend.PRDetail) {
	switch what {
	case "title":
		s.titleURL = d.URL
		s.title.Start(d.Title)
	case "description":
		s.bodyURL = d.URL
		s.body.SetText(d.Body)
	}
}
