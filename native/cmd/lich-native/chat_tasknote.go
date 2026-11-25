package main

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// A <task-notification> is Claude Code's own note to its model that a
// background task (a subagent, a shell command, a monitor) moved, written into
// the conversation as a user message. Like lich's envelope its text goes in
// unescaped, so it is read by its tags, never as XML.

const (
	taskNoteOpen  = "<task-notification>"
	taskNoteClose = "</task-notification>"
	// taskNotePreamble is the line Claude Code puts over some of them,
	// telling its model the message is not the person's.
	taskNotePreamble = "[SYSTEM NOTIFICATION - NOT USER INPUT]"
	taskNoteFailed   = "failed"
)

// taskNote is what a row shows of one, in Claude Code's words: a subagent
// carries its result, a monitor its event, and a command neither, its output
// staying in its file.
type taskNote struct {
	status, summary, result, event string
}

// parseTaskNotes reads a message that is one <task-notification> or several,
// and is false for any other message.
func parseTaskNotes(text string) ([]taskNote, bool) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, taskNotePreamble) {
		_, rest, ok := strings.Cut(text, "\n\n"+taskNoteOpen)
		if !ok {
			return nil, false
		}
		text = taskNoteOpen + rest
	}
	if !strings.HasPrefix(text, taskNoteOpen) || !strings.HasSuffix(text, taskNoteClose) {
		return nil, false
	}
	var notes []taskNote
	for chunk := range strings.SplitSeq(strings.TrimSuffix(text, taskNoteClose), taskNoteClose) {
		chunk = strings.TrimSpace(chunk)
		if !strings.HasPrefix(chunk, taskNoteOpen) {
			return nil, false
		}
		n := taskNote{
			status:  taskNoteTag(chunk, "status"),
			summary: taskNoteTag(chunk, "summary"),
			result:  taskNoteLongTag(chunk, "result"),
			event:   taskNoteLongTag(chunk, "event"),
		}
		if n.summary == "" {
			return nil, false
		}
		notes = append(notes, n)
	}
	return notes, true
}

// taskNoteTag is the text of chunk's tag on a line of its own, to the tag's
// first close.
func taskNoteTag(chunk, tag string) string {
	_, rest, ok := strings.Cut(chunk, "\n<"+tag+">")
	text, _, closed := strings.Cut(rest, "</"+tag+">")
	if !ok || !closed {
		return ""
	}
	return strings.TrimSpace(text)
}

// taskNoteLongTag is taskNoteTag for a long text, a result, which may quote any
// tag, its own included: it runs to the tag's last close.
func taskNoteLongTag(chunk, tag string) string {
	_, rest, ok := strings.Cut(chunk, "\n<"+tag+">")
	stop := strings.LastIndex(rest, "</"+tag+">")
	if !ok || stop < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:stop])
}

// chatTaskNotes draws each note as a row under the next: the provider's mark,
// the summary, a hairline and, when it carries text, a chevron that unfolds it.
// They open shut: the agent reads them, the person seldom needs to.
func (u *window) chatTaskNotes(gtx C, th *ui.Theme, p *chatPane, kind string, it backend.ChatItem, notes []taskNote) D {
	children := make([]layout.FlexChild, 0, 2*len(notes))
	for i, n := range notes {
		if i > 0 {
			children = append(children, ui.VGap(2))
		}
		key := it.ID + "#" + strconv.Itoa(i)
		children = append(children, layout.Rigid(func(gtx C) D { return u.chatTaskNote(gtx, th, p, kind, key, n) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (u *window) chatTaskNote(gtx C, th *ui.Theme, p *chatPane, kind, key string, n taskNote) D {
	f := p.fold(key)
	f.update(gtx)
	tone := th.Foreground
	if n.status == taskNoteFailed {
		tone = th.Destructive
	}
	folds := n.result != "" || n.event != ""
	line := func(gtx C) D {
		row := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, kind, ui.Space(3.5), markPixel) }),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextSM, n.summary).Weight(font.Medium).In(tone).Layout),
					layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
				)
			}),
		}
		if folds {
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
	if !folds {
		return line(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return f.click.Layout(gtx, line) }),
		layout.Rigid(func(gtx C) D {
			if f.shut(gtx.Now) {
				return f.body.Layout(gtx, false, chatNothing)
			}
			return f.body.Layout(gtx, f.open, func(gtx C) D {
				return layout.Inset{Top: ui.Space(1.5), Left: ui.Space(5.5)}.Layout(gtx, func(gtx C) D {
					if n.result == "" {
						return th.Text(ui.TextSM, expandTabs(n.event)).Muted().Wrapped().Layout(gtx)
					}
					notify := func(t ui.Toast) { u.toaster.Show(t) }
					return p.md.LayoutLinks(gtx, th, key, n.result, markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths})
				})
			})
		}),
	)
}
