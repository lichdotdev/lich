package main

import (
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// chatNoteIndent puts a worker's result under its title, past the avatar.
var chatNoteIndent = ui.Space(7)

// chatEnvelopes draws a message lich handed the agent: a task as the person's
// box naming who sent it, a worker's outcome as a row, anything else as a
// muted line.
//
// A task or a request stands as the person's message does, with its time and
// copy button under it, copying the box's words rather than the envelope's
// tags. Note rows
// carry no such line: one under each would space them unevenly against the
// rows of the same message.
func (u *window) chatEnvelopes(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem, envs []envelope) D {
	rows := func(gtx C) D {
		children := make([]layout.FlexChild, 0, 2*len(envs))
		for i, env := range envs {
			if i > 0 {
				children = append(children, ui.VGap(3.5))
			}
			key := it.ID + "#" + strconv.Itoa(i)
			children = append(children, layout.Rigid(func(gtx C) D { return u.chatEnvelope(gtx, th, p, key, env) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	if len(envs) != 1 || (envs[0].kind != "task" && envs[0].kind != "request") {
		return rows(gtx)
	}
	readable := it
	readable.Text = envelopesText(envs)
	c := p.copyOf(it.ID)
	return c.hover(gtx, func(gtx C, over bool) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(rows),
			ui.VGap(1),
			layout.Rigid(func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D { return u.chatStampRow(gtx, th, p, c, readable, over, true) })
			}),
		)
	})
}

// envelopesText is what envelopes say, in the words their rows draw: what
// copying them puts on the clipboard.
func envelopesText(envs []envelope) string {
	parts := make([]string, 0, len(envs))
	for _, env := range envs {
		var lines []string
		switch env.kind {
		case "request":
			lines = append(lines, env.text["prompt"])
		case "task":
			if env.from != "" {
				lines = append(lines, i18n.T("chat.notification.from", i18n.Args{"label": env.from}))
			}
			lines = append(lines, env.text["prompt"])
		case "worker-finished", "worker-failed", "worker-stopped", "worker-waiting", "expired":
			title, meta, body := workerNoteText(env)
			if meta != "" {
				title += " · " + meta
			}
			lines = append(lines, title)
			if result := env.text["result"]; result != "" {
				body = result
			}
			if body != "" {
				lines = append(lines, body)
			}
		default:
			lines = append(lines, env.text["summary"])
			if env.text["prompt"] != "" {
				lines = append(lines, env.text["prompt"])
			}
		}
		parts = append(parts, strings.Join(lines, "\n\n"))
	}
	return strings.Join(parts, "\n\n")
}

func (u *window) chatEnvelope(gtx C, th *ui.Theme, p *chatPane, key string, env envelope) D {
	switch env.kind {
	case "task":
		from := ""
		if env.from != "" {
			from = i18n.T("chat.notification.from", i18n.Args{"label": env.from})
		}
		return u.chatUserFrom(gtx, th, p, key, from, env.text["prompt"])
	case "worker-finished", "worker-failed", "worker-stopped", "worker-waiting", "expired":
		return u.chatWorkerNote(gtx, th, p, key, env)
	case "request":
		return u.chatRequest(gtx, th, p, key, env)
	}
	line := layout.Rigid(th.Text(ui.TextSM, env.text["summary"]).Muted().Wrapped().Layout)
	if env.text["prompt"] == "" {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, line)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, line, ui.VGap(2),
		layout.Rigid(func(gtx C) D { return u.chatUser(gtx, th, p, key, env.text["prompt"]) }))
}

// chatWorkerNote is a worker's outcome: its agent's mark, what happened and
// muted how, a hairline to the column's edge, and the result as Markdown under
// it. A click on the row opens the worker's card while it is still open; the
// result keeps its own links.
func (u *window) chatWorkerNote(gtx C, th *ui.Theme, p *chatPane, key string, env envelope) D {
	click := p.note(key)
	if click.Clicked(gtx) {
		if sv, ok := u.model.sessionViewOf(env.session); ok {
			u.openSession(sv.projectID, sv.ID)
		}
	}
	title, meta, body := workerNoteText(env)
	tone := th.Foreground
	switch env.kind {
	case "worker-waiting":
		tone = th.ToneWait
	case "worker-failed":
		tone = th.Destructive
	}
	tint, mark, _ := u.agentLook(env.session)
	row := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return chatAvatar(gtx, th, tint, mark) }), ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, title).Weight(font.Medium).In(tone).Layout),
	}
	if meta != "" {
		row = append(row, ui.Gap(2), layout.Rigid(th.Text(ui.TextSM, meta).Muted().Layout))
	}
	row = append(row, ui.Gap(3), layout.Flexed(1, func(gtx C) D { return chatHairline(gtx, th) }))
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return click.Layout(gtx, func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...) })
	})}
	if result := env.text["result"]; result != "" {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: chatNoteIndent}.Layout(gtx, func(gtx C) D {
				notify := func(t ui.Toast) { u.toaster.Show(t) }
				return p.md.LayoutLinks(gtx, th, key, result, markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths})
			})
		}))
	} else if body != "" {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: chatNoteIndent}.Layout(gtx, th.Text(ui.TextSM, body).Muted().Wrapped().Layout)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// workerNoteText is a worker note's title, the muted words beside it and the
// muted text under it when there is no result.
func workerNoteText(env envelope) (title, meta, body string) {
	who := i18n.Args{"label": env.from}
	switch env.kind {
	case "worker-finished":
		return i18n.T("chat.notification.finished", who), i18n.T("chat.notification.answered"), env.text["summary"]
	case "worker-failed":
		return i18n.T("chat.notification.failed", who), "", env.text["error"]
	case "worker-stopped":
		by := i18n.T("chat.notification.stoppedByYou")
		if env.text["by"] != "user" {
			by = i18n.T("chat.notification.stoppedBy", i18n.Args{"label": env.text["by"]})
		}
		return i18n.T("chat.notification.stopped", who), by, ""
	case "worker-waiting":
		return i18n.T("chat.notification.waiting", who), "", env.text["request"]
	}
	return i18n.T("chat.notification.expired", who), "", ""
}

// note is the click on one notification row, made on first use.
func (v *chatView) note(key string) *ui.Clickable {
	c, ok := v.notes[key]
	if !ok {
		if v.notes == nil {
			v.notes = map[string]*ui.Clickable{}
		}
		c = new(ui.Clickable)
		v.notes[key] = c
	}
	return c
}
