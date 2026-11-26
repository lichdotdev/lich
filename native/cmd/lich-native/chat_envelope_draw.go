package main

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

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
				stamp := func(gtx C) D {
					return u.chatStampRow(gtx, th, c, readable.Text, func(gtx C) D { return u.chatDatedStamp(gtx, th, p, readable) }, over, true)
				}
				if envs[0].kind == "task" {
					return layout.Inset{Left: chatTaskIndent}.Layout(gtx, stamp)
				}
				return layout.E.Layout(gtx, stamp)
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
		return u.chatTask(gtx, th, p, key, env)
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

// chatTaskIndent is how far a task's balloon stands from the column's left
// edge: past its sender's mark and the gap after it.
var chatTaskIndent = ui.Space(chatTaskMark + chatTaskGap)

const chatTaskMark, chatTaskGap = 3.5, 2.5

// chatTask is a task another session handed this one, at the column's left
// as the other side of a conversation: the sender's provider mark, its name
// and an arrow that opens it while it is open, then the prompt in a Card
// balloon whose corner by the mark is the tight one. The right side stays the
// person's own.
func (u *window) chatTask(gtx C, th *ui.Theme, p *chatPane, key string, env envelope) D {
	open := p.note(key)
	if open.Clicked(gtx) {
		if sv, ok := u.model.sessionViewOf(env.session); ok {
			u.openSession(sv.projectID, sv.ID)
		}
	}
	sv, alive := u.model.sessionViewOf(env.session)
	maxW := int(float32(gtx.Constraints.Max.X) * chatUserShare)
	on := th.On(th.Card)
	balloon := func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = maxW
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				r, tip := gtx.Dp(ui.RadiusXL), gtx.Dp(ui.RadiusSM)
				box := clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Min}, NW: r, NE: r, SE: r, SW: tip}
				paint.FillShape(gtx.Ops, th.Card, box.Op(gtx.Ops))
				paint.FillShape(gtx.Ops, th.Border, clip.Stroke{Path: box.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
				return D{Size: gtx.Constraints.Min}
			},
			func(gtx C) D {
				return layout.Inset{Top: ui.Space(3), Bottom: ui.Space(3), Left: ui.Space(4), Right: ui.Space(4)}.Layout(gtx, func(gtx C) D {
					return u.userMarkdown(gtx, on, p, key, env.text["prompt"])
				})
			})
	}
	head := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, env.from).Weight(font.Medium).Layout)}
	if alive {
		head = append(head, ui.Gap(1.5), layout.Rigid(func(gtx C) D {
			return open.Layout(gtx, func(gtx C) D {
				fg := th.MutedForeground
				if open.Hovered() {
					fg = th.Foreground
				}
				return icOpenCard.Layout(gtx, ui.Space(3), fg)
			})
		}))
	}
	column := []layout.FlexChild{}
	if env.from != "" {
		column = append(column, layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, head...) }), ui.VGap(1))
	}
	column = append(column, layout.Rigid(balloon))
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, sv.Kind, ui.Space(chatTaskMark), markPixel) }),
		ui.Gap(chatTaskGap),
		layout.Rigid(func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, column...) }),
	)
}

// chatWorkerNote is a worker's outcome as one line, shut: the worker's
// provider mark, a glyph of how it ended, what happened, muted how, the first
// line of its text, a chevron that unfolds the text, and an arrow that opens
// the worker's card while it is still open. The text, a result as Markdown,
// is the agent's to read; the person seldom needs it.
func (u *window) chatWorkerNote(gtx C, th *ui.Theme, p *chatPane, key string, env envelope) D {
	open := p.note(key)
	if open.Clicked(gtx) {
		if sv, ok := u.model.sessionViewOf(env.session); ok {
			u.openSession(sv.projectID, sv.ID)
		}
	}
	f := p.fold(key)
	f.update(gtx)
	sv, alive := u.model.sessionViewOf(env.session)
	title, meta, body := workerNoteText(env)
	result := env.text["result"]
	text := result
	if text == "" {
		text = body
	}
	tone := th.Foreground
	switch env.kind {
	case "worker-waiting":
		tone = th.ToneWait
	case "worker-failed":
		tone = th.Destructive
	}
	glyph, ink := workerNoteGlyph(th, env.kind)

	line := func(gtx C) D {
		row := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, sv.Kind, ui.Space(3.5), markPixel) }),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return glyph.Layout(gtx, ui.Space(3.5), ink) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextSM, title).Weight(font.Medium).In(tone).Layout),
		}
		if meta != "" {
			row = append(row, ui.Gap(2), layout.Rigid(th.Text(ui.TextSM, meta).Muted().Layout))
		}
		preview := ""
		if !f.open {
			preview = markdownMarks.Replace(firstLine(strings.TrimSpace(text)))
		}
		row = append(row, ui.Gap(2), layout.Flexed(1, th.Text(ui.TextSM, preview).Muted().Truncated().Layout))
		if text != "" {
			ic := icChatClosed
			if f.open {
				ic = icChatOpen
			}
			fg := th.MutedForeground
			if f.click.Hovered() {
				fg = th.Foreground
			}
			row = append(row, ui.Gap(2), layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	}
	head := []layout.FlexChild{layout.Flexed(1, func(gtx C) D {
		if text == "" {
			return line(gtx)
		}
		return f.click.Layout(gtx, line)
	})}
	if alive {
		head = append(head, ui.Gap(2), layout.Rigid(func(gtx C) D {
			return open.Layout(gtx, func(gtx C) D {
				fg := th.MutedForeground
				if open.Hovered() {
					fg = th.Foreground
				}
				return icOpenCard.Layout(gtx, ui.Space(3.5), fg)
			})
		}))
	}
	headRow := layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, head...) })
	if text == "" {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, headRow)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, headRow,
		layout.Rigid(func(gtx C) D {
			if f.shut(gtx.Now) {
				return f.body.Layout(gtx, false, chatNothing)
			}
			return f.body.Layout(gtx, f.open, func(gtx C) D {
				return layout.Inset{Top: ui.Space(1.5), Left: ui.Space(5.5)}.Layout(gtx, func(gtx C) D {
					if result == "" {
						return th.Text(ui.TextSM, body).Muted().Wrapped().Layout(gtx)
					}
					notify := func(t ui.Toast) { u.toaster.Show(t) }
					return p.md.LayoutLinks(gtx, th, key, result, markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths})
				})
			})
		}),
	)
}

// markdownMarks are the marks a shut note's preview drops: it is a line of
// plain text, and code spans and bold read as their marks there.
var markdownMarks = strings.NewReplacer("`", "", "**", "")

// workerNoteGlyph is the glyph of how a worker's task ended, in the tone the
// turn's closing line gives the same end.
func workerNoteGlyph(th *ui.Theme, kind string) (*icons.Icon, color.NRGBA) {
	switch kind {
	case "worker-finished":
		return icCheck, th.TonePass
	case "worker-failed":
		return icToolsFailed, th.Destructive
	case "worker-stopped":
		return icChatStop, th.MutedForeground
	case "worker-waiting":
		return icQueued, th.ToneWait
	}
	return icQueued, th.MutedForeground
}

// workerNoteText is a worker note's title, the muted words beside it and the
// muted text under it when there is no result.
func workerNoteText(env envelope) (title, meta, body string) {
	who := i18n.Args{"label": env.from}
	switch env.kind {
	case "worker-finished":
		return i18n.T("chat.notification.finished", who), "", env.text["summary"]
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
