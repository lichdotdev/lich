package main

import (
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icChatSend   = icons.Lucide("arrow-up")
	icChatStop   = icons.Lucide("square")
	icChatClosed = icons.Lucide("chevron-right")
	icChatOpen   = icons.Lucide("chevron-down")
	// chatToolRow is mockup A's h-7 tool line.
	chatToolRow = ui.Space(7)
)

// chatClock is the short time under a message, the 24-hour layout the window's
// other clocks (the Git graph, a stopped console) use.
const chatClock = "15:04"

func (u *window) chatRow(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	switch r.kind {
	case chatRowWork:
		return chatWork(gtx, th, p.fold("work:"+strconv.Itoa(r.turn)), r)
	case chatRowLive:
		return chatLive(gtx, th)
	case chatRowDay:
		return layout.Center.Layout(gtx, th.Text(ui.TextXS, dayLabel(time.UnixMilli(r.day).In(gtx.Now.Location()), gtx.Now)).Muted().Layout)
	}
	it := r.item
	switch it.Kind {
	case api.ItemUser:
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return chatUser(gtx, th, it.Text) }),
			ui.VGap(1),
			layout.Rigid(func(gtx C) D { return layout.E.Layout(gtx, func(gtx C) D { return u.chatStamp(gtx, th, p, it) }) }),
		)
	case api.ItemAgent:
		notify := func(t ui.Toast) { u.toaster.Show(t) }
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return p.md.Layout(gtx, th, it.ID, it.Text, func(url string) { openLink(u.client, notify, url) })
			}),
			ui.VGap(1),
			layout.Rigid(func(gtx C) D { return layout.W.Layout(gtx, func(gtx C) D { return u.chatStamp(gtx, th, p, it) }) }),
		)
	case api.ItemThought:
		return chatThought(gtx, th, p.fold(it.ID), it.Text)
	case api.ItemPermission:
		if it.Permission == nil {
			return D{}
		}
		return chatPermission(gtx, th, p, it)
	}
	return th.Text(ui.TextSM, expandTabs(it.Text)).Muted().Wrapped().Layout(gtx)
}

// chatStamp is the time an item began, in the window's short clock, muted,
// with the whole date in a tooltip. An item with no time draws nothing.
func (u *window) chatStamp(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem) D {
	if it.At <= 0 {
		return D{}
	}
	t := time.UnixMilli(it.At).In(gtx.Now.Location())
	return th.Tooltip(&u.root, p.stamp(it.ID), clockText(t)).Layout(gtx, th.Text(ui.TextXS, t.Format(chatClock)).Muted().Layout)
}

// dayLabel names a day against now: Today, Yesterday, else its date, with the
// year when it is not this one.
func dayLabel(t, now time.Time) string {
	switch {
	case sameDay(t, now):
		return i18n.T("chat.day.today")
	case sameDay(t, now.AddDate(0, 0, -1)):
		return i18n.T("chat.day.yesterday")
	case t.Year() == now.Year():
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

// update takes the fold's click before what it opens is laid out.
func (f *chatFold) update(gtx C) {
	if f.click.Clicked(gtx) {
		f.open = !f.open
	}
}

// chatFoldLine is a fold's line: its chevron and label, muted, lit on hover.
func chatFoldLine(gtx C, th *ui.Theme, f *chatFold, label string) D {
	ic := icChatClosed
	if f.open {
		ic = icChatOpen
	}
	return f.click.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		if f.click.Hovered() {
			fg = th.Foreground
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextSM, label).In(fg).Layout),
		)
	})
}

func chatThought(gtx C, th *ui.Theme, f *chatFold, text string) D {
	f.update(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return chatFoldLine(gtx, th, f, i18n.T("chat.view.thought")) }),
		layout.Rigid(func(gtx C) D {
			if !f.open {
				return D{}
			}
			return layout.Inset{Top: ui.Space(1.5), Left: ui.Space(5)}.Layout(gtx, th.Text(ui.TextSM, expandTabs(text)).Muted().Wrapped().Layout)
		}),
	)
}

// chatWork is a turn's tool calls folded into one line, how long the turn
// ran and how many it made, opening on a line per call.
func chatWork(gtx C, th *ui.Theme, f *chatFold, r chatRow) D {
	f.update(gtx)
	label := i18n.T("chat.view.worked", i18n.Args{"duration": formatAge(r.took), "count": len(r.tools)})
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return chatFoldLine(gtx, th, f, label) })}
	if f.open {
		children = append(children, ui.VGap(1))
		for _, it := range r.tools {
			children = append(children, layout.Rigid(func(gtx C) D { return chatTool(gtx, th, it.Tool) }))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// chatTool is one call: its name, what it acted on, and how it ended, with
// the diff it made under it.
func chatTool(gtx C, th *ui.Theme, t *backend.ChatToolCall) D {
	if t == nil {
		return D{}
	}
	line := func(gtx C) D {
		h := gtx.Dp(chatToolRow)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, t.Name).Weight(font.Medium).Layout),
				ui.Gap(2.5),
				layout.Flexed(1, th.Text(ui.TextXS, cleanLine(t.Title)).Mono().Muted().Layout),
				ui.Gap(2.5),
				layout.Rigid(func(gtx C) D { return chatToolOutcome(gtx, th, t) }),
			)
		})
	}
	if t.Diff == "" {
		return line(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(line),
		layout.Rigid(func(gtx C) D { return chatDiff(gtx, th, t.Diff) }),
	)
}

func chatToolOutcome(gtx C, th *ui.Theme, t *backend.ChatToolCall) D {
	out := cleanLine(t.Output)
	switch t.State {
	case api.ToolPending, api.ToolRunning:
		return th.Spinner(gtx, ui.Space(3), th.MutedForeground)
	case api.ToolFailed:
		if out == "" {
			out = i18n.T("chat.view.toolFailed")
		}
		return th.Text(ui.TextXS, out).Mono().In(th.Destructive).Layout(gtx)
	}
	return th.Text(ui.TextXS, out).Mono().Muted().Layout(gtx)
}

// cleanLine is the first line of s, tabs expanded: a row has room for one.
func cleanLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return expandTabs(s)
}

// expandTabs is s with its tabs as spaces: the fonts have no glyph for one.
func expandTabs(s string) string { return strings.ReplaceAll(s, "\t", hunkTab) }

// chatDiff is a tool's whole diff on the window's background, each added or
// removed line in its tone, as a thread's hunk is (hunkTail).
func chatDiff(gtx C, th *ui.Theme, diff string) D {
	var children []layout.FlexChild
	for line := range strings.SplitSeq(strings.TrimRight(diff, "\n"), "\n") {
		children = append(children, layout.Rigid(func(gtx C) D { return hunkLine(gtx, th, line) }))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusSM) },
		func(gtx C) D {
			return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
}

// chatPermission is the agent asking before a tool runs: a box with the
// tool, its diff and a button per answer the provider offers, the first
// allow filled. Answered, it is a muted line with the answer.
func chatPermission(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem) D {
	perm := it.Permission
	tool := strings.TrimSpace(perm.Tool.Name + " " + cleanLine(perm.Tool.Title))
	if perm.Chosen != "" {
		choice := perm.Chosen
		for _, o := range perm.Options {
			if o.ID == perm.Chosen {
				choice = o.Label
			}
		}
		return th.Text(ui.TextXS, i18n.T("chat.view.answered", i18n.Args{"tool": tool, "choice": choice})).Muted().Layout(gtx)
	}
	box := th.On(th.Sidebar)
	primary := permitOption(perm, api.PermitAllowOnce, api.PermitAllowAlways)
	var buttons []layout.FlexChild
	for i, o := range perm.Options {
		if i > 0 {
			buttons = append(buttons, ui.Gap(1.5))
		}
		b := box.Button(p.answer(it.ID, o.ID), o.Label)
		b.Size, b.Variant = ui.SizeSM, ui.VariantOutline
		if o.ID == primary {
			b.Variant = ui.VariantDefault
		}
		buttons = append(buttons, layout.Rigid(b.Layout))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
		func(gtx C) D {
			return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
							layout.Rigid(box.Text(ui.TextSM, perm.Tool.Name).Weight(font.Medium).Layout),
							ui.Gap(2),
							layout.Flexed(1, box.Text(ui.TextXS, cleanLine(perm.Tool.Title)).Mono().Muted().Layout),
						)
					}),
					layout.Rigid(func(gtx C) D {
						if perm.Tool.Diff == "" {
							return D{}
						}
						return layout.Inset{Top: ui.Space(2)}.Layout(gtx, func(gtx C) D { return chatDiff(gtx, box, perm.Tool.Diff) })
					}),
					ui.VGap(2.5),
					layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...) }),
				)
			})
		})
}

// chatLive is the turn in flight.
func chatLive(gtx C, th *ui.Theme) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return th.Spinner(gtx, ui.Space(3.5), th.MutedForeground) }),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, i18n.T("chat.view.working")).Muted().Layout),
	)
}
