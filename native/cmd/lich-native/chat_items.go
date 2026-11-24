package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"path"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

var (
	icChatFail   = icons.Lucide("x")
	icChatSend   = icons.Lucide("arrow-up")
	icChatStop   = icons.Lucide("square")
	icChatClosed = icons.Lucide("chevron-right")
	icChatOpen   = icons.Lucide("chevron-down")
	// chatToolRow is mockup A's h-7 tool line.
	chatToolRow = ui.Space(7)
)

// chatShellLines is how much of a shell command's output shows before the
// rest opens on a click.
const chatShellLines = 20

var (
	// chatBranchGutter is the room a tool row gives its connector, and
	// chatBranchX where the connector runs: under the fold's chevron.
	chatBranchGutter = ui.Space(6)
	chatBranchX      = ui.Space(1.75)
)

// chatClock is the short time under a message, the 24-hour layout the window's
// other clocks (the Git graph, a stopped console) use.
const chatClock = "15:04"

func (u *window) chatRow(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	switch r.kind {
	case chatRowWork:
		return u.chatWork(gtx, th, p, p.fold("work:"+r.run), sv, r)
	case chatRowShell:
		return chatBranch(gtx, th, true, func(gtx C) D { return chatShellCall(gtx, th, p.fold("out:"+r.item.ID), r.item, r.live) })
	case chatRowEdit:
		return u.chatEdit(gtx, th, p, sv, r)
	case chatRowLive:
		if r.compacting {
			return chatCompacting(gtx, th, r)
		}
		return chatLive(gtx, th, sv.ID, r)
	case chatRowDone:
		if p.swap.key == chatRowKey(r) && p.swap.playing(gtx.Now) {
			return u.chatSwapLine(gtx, th, p, sv, r)
		}
		return u.chatDone(gtx, th, p, sv, r, 1, 1)
	case chatRowDay:
		return layout.Center.Layout(gtx, th.Text(ui.TextXS, dayLabel(time.UnixMilli(r.day).In(gtx.Now.Location()), gtx.Now)).Muted().Layout)
	}
	it := r.item
	switch it.Kind {
	case api.ItemUser:
		if envs, ok := parseEnvelopes(it.Text); ok {
			return u.chatEnvelopes(gtx, th, p, it, envs)
		}
		c := p.copyOf(it.ID)
		return c.hover(gtx, func(gtx C, over bool) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return u.sentCards(gtx, th, p, it.ID, it.Files) }),
				layout.Rigid(func(gtx C) D {
					if it.Text == "" {
						return D{}
					}
					return u.chatUser(gtx, th, p, it.ID, it.Text)
				}),
				ui.VGap(1),
				layout.Rigid(func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return u.chatStampRow(gtx, th, c, it.Text, func(gtx C) D { return u.chatDatedStamp(gtx, th, p, it) }, over, true)
					})
				}),
			)
		})
	case api.ItemAgent:
		return u.chatAgent(gtx, th, p, sv, r)
	case api.ItemCompaction:
		return u.chatCompaction(gtx, th, p, it)
	case api.ItemThought:
		return chatThought(gtx, th, p.fold(it.ID), it, r.live)
	case api.ItemPermission:
		if it.Permission == nil {
			return D{}
		}
		return chatPermission(gtx, th, p, it)
	case api.ItemQuestion:
		if it.Question == nil {
			return D{}
		}
		if it.Question.Proposal != nil {
			return u.chatProposal(gtx, th, p, it)
		}
		return chatQuestion(gtx, th, it)
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
	return u.chatStampText(gtx, th, p, it, t, t.Format(chatClock))
}

// chatDatedStamp is chatStamp with the day in front when it is not today's:
// "Nov 18 · 14:32", the year too when it is not this one's.
func (u *window) chatDatedStamp(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem) D {
	if it.At <= 0 {
		return D{}
	}
	t := time.UnixMilli(it.At).In(gtx.Now.Location())
	return u.chatStampText(gtx, th, p, it, t, datedClock(t, gtx.Now))
}

func (u *window) chatStampText(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem, t time.Time, text string) D {
	return th.Tooltip(&u.root, p.stamp(it.ID), clockText(t)).Layout(gtx, th.Text(ui.TextXS, text).Muted().Layout)
}

// datedClock is t's short clock, led by its day unless that is now's.
func datedClock(t, now time.Time) string {
	switch {
	case sameDay(t, now):
		return t.Format(chatClock)
	case t.Year() == now.Year():
		return t.Format("Jan 2 · " + chatClock)
	}
	return t.Format("Jan 2, 2006 · " + chatClock)
}

// chatStampRow is the line under a message: the stamp, and while the pointer is
// over the message the copy button at its left. The person's message keeps its
// time at rest; an answer shows both only under the pointer. The line is as
// tall with the button as without it.
func (u *window) chatStampRow(gtx C, th *ui.Theme, c *chatCopy, text string, stamp func(C) D, over, atRest bool) D {
	gtx.Constraints.Min.Y = gtx.Dp(chatCopySlot)
	if !over && !atRest {
		return D{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
	}
	var row []layout.FlexChild
	if over {
		row = append(row, layout.Rigid(func(gtx C) D { return c.button(gtx, th, text) }), ui.Gap(1))
	}
	row = append(row, layout.Rigid(stamp))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}

// chatAnswerStamp is the line under an answer: the model and effort that wrote
// it, as the item says them (never the chat's options now), and its time, muted.
// The provider's name is the tooltip's, ahead of the full date.
func (u *window) chatAnswerStamp(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem, provider string) D {
	var parts []string
	for _, part := range []string{it.Model, it.Effort} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	tip := provider
	if it.At > 0 {
		t := time.UnixMilli(it.At).In(gtx.Now.Location())
		parts, tip = append(parts, t.Format(chatClock)), tip+" · "+clockText(t)
	}
	if len(parts) == 0 {
		return D{}
	}
	return th.Tooltip(&u.root, p.stamp(it.ID), tip).Layout(gtx, th.Text(ui.TextXS, strings.Join(parts, " · ")).Muted().Layout)
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
		f.open, f.turned = !f.open, gtx.Now
	}
}

// chatFoldLine is a fold's line: its chevron, a glyph of the step when it has
// one and the label, muted, lit on hover.
func chatFoldLine(gtx C, th *ui.Theme, f *chatFold, lead icons.Glyph, label func(gtx C, fg color.NRGBA) D) D {
	ic := icChatClosed
	if f.open {
		ic = icChatOpen
	}
	return f.click.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		if f.click.Hovered() {
			fg = th.Foreground
		}
		row := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) }),
			ui.Gap(1.5),
		}
		if lead != nil {
			row = append(row, layout.Rigid(func(gtx C) D { return lead.Layout(gtx, ui.Space(3.5), fg) }), ui.Gap(1.5))
		}
		row = append(row, layout.Rigid(func(gtx C) D { return label(gtx, fg) }))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// chatLiveLabel is a live step's label: its words shimmering, and the time it
// has run beside them, counting each second.
func chatLiveLabel(gtx C, th *ui.Theme, words string, fg color.NRGBA, since time.Time) D {
	row := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, words).In(fg).Shimmer().Layout)}
	if !since.IsZero() {
		chatTick(gtx, since)
		row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, chatElapsed(gtx.Now.Sub(since))).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}

func chatThought(gtx C, th *ui.Theme, f *chatFold, it backend.ChatItem, live bool) D {
	f.update(gtx)
	label := func(gtx C, fg color.NRGBA) D {
		if !live {
			return th.Text(ui.TextSM, i18n.T("chat.view.thought")).In(fg).Layout(gtx)
		}
		var since time.Time
		if it.At > 0 {
			since = time.UnixMilli(it.At)
		}
		return chatLiveLabel(gtx, th, i18n.T("chat.view.thinking"), fg, since)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return chatFoldLine(gtx, th, f, icThink, label) }),
		layout.Rigid(func(gtx C) D {
			if f.shut(gtx.Now) {
				return f.body.Layout(gtx, false, chatNothing)
			}
			return f.body.Layout(gtx, f.open, func(gtx C) D {
				return layout.Inset{Top: ui.Space(1.5), Left: ui.Space(5)}.Layout(gtx, th.Text(ui.TextSM, expandTabs(it.Text)).Muted().Wrapped().Layout)
			})
		}),
	)
}

// chatWork is a run of tool calls between the agent's texts: one call alone
// is its own row; several fold into a line saying what they did, a clause per
// kind with the time they took and how many failed, opening on a row per call.
// While a call runs the line is that call, shimmering, beside the count.
func (u *window) chatWork(gtx C, th *ui.Theme, p *chatPane, f *chatFold, sv sessionView, r chatRow) D {
	outcome := u.vetoOutcome(p, sv)
	if len(r.tools) == 1 {
		return chatToolWith(gtx, th, r.tools[0], r.live, outcome(r.tools[0]), 0)
	}
	f.update(gtx)
	vetoed := func(it backend.ChatItem) bool { return outcome(it) != nil }
	label := func(gtx C, fg color.NRGBA) D { return chatWorkLabel(gtx, th, r, vetoed, fg) }
	earlier := p.fold("earlier:" + r.run)
	earlier.update(gtx)
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return chatFoldLine(gtx, th, f, nil, label) })}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, append(children, chatCalls(gtx, th, f, earlier, r, outcome)...)...)
}

// chatWorkLabel is a fold of calls' line: the call running, or what the run
// did once it is over.
func chatWorkLabel(gtx C, th *ui.Theme, r chatRow, vetoed func(backend.ChatItem) bool, fg color.NRGBA) D {
	if now, ok := runningCall(r.tools); r.live && ok {
		words := callVerb(now.Tool, true)
		if target := cleanLine(now.Tool.Title); target != "" {
			words += " " + chatShort(target)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return chatLiveLabel(gtx, th, words, fg, gtx.Now.Add(-r.took)) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextSM, "· "+i18n.T("chat.view.toolCalls", i18n.Args{"count": len(r.tools)})).Muted().Layout),
		)
	}
	label, failed := callSummary(r.tools, vetoed)
	row := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextSM, label).In(fg).Layout),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextSM, chatTook(r.took)).Muted().Layout),
	}
	if failed != "" {
		row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, "· "+failed).In(th.Destructive).Layout))
	}
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, row...)
}

// chatShortRunes is as much of a running call's target as the fold's line
// shows: a long command would push the count off the line.
const chatShortRunes = 60

// chatShort is s cut to chatShortRunes with an ellipsis.
func chatShort(s string) string {
	if r := []rune(s); len(r) > chatShortRunes {
		return string(r[:chatShortRunes]) + "…"
	}
	return s
}

// chatCallsShown is how many of a group's last calls show when it opens: a
// turn of fifty calls would run past the view; the rest are a click away.
const chatCallsShown = 5

// chatCalls are a group's calls, under the line and indented past its
// chevron, each in a fold of its own that opens and
// shuts with the group: its last chatCallsShown, under a line that shows the
// earlier ones until earlier is opened. A call that comes once the group is
// drawn enters as a row does.
func chatCalls(gtx C, th *ui.Theme, f, earlier *chatFold, r chatRow, outcome func(backend.ChatItem) *chatOpened) []layout.FlexChild {
	ids := make([]string, len(r.tools))
	for i, it := range r.tools {
		ids[i] = it.ID
	}
	f.enter.Update(gtx.Now, ids)
	hidden := max(len(r.tools)-chatCallsShown, 0)
	verbW := chatVerbWidth(gtx, th, r.tools, r.live)
	children := make([]layout.FlexChild, 0, len(r.tools)+1)
	if hidden > 0 {
		children = append(children, layout.Rigid(func(gtx C) D {
			fold := f.call("earlier")
			if f.shut(gtx.Now) {
				return fold.Layout(gtx, false, chatNothing)
			}
			return fold.Layout(gtx, f.open, func(gtx C) D {
				return layout.Inset{Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
					return layout.Inset{Left: chatBranchGutter}.Layout(gtx, func(gtx C) D { return chatEarlierCalls(gtx, th, earlier, hidden) })
				})
			})
		}))
	}
	for i, it := range r.tools {
		if i < hidden && !earlier.open {
			continue
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return f.enter.LayoutFlat(gtx, it.ID, func(gtx C) D {
				fold := f.call(it.ID)
				if f.shut(gtx.Now) {
					return fold.Layout(gtx, false, chatNothing)
				}
				return fold.Layout(gtx, f.open, func(gtx C) D {
					var top unit.Dp
					if i == 0 && hidden == 0 {
						top = ui.Space(1)
					}
					return layout.Inset{Top: top}.Layout(gtx, func(gtx C) D {
						return layout.Inset{Left: chatBranchGutter}.Layout(gtx, func(gtx C) D { return chatToolWith(gtx, th, it, r.live, outcome(it), verbW) })
					})
				})
			})
		}))
	}
	return children
}

// chatShellCall is a shell command's call with its output under it, as
// Claude Code prints it, on a branch from the person's command: the row keeps
// the mark, neither the command the message over it already shows nor the
// output's first line, and output past chatShellLines opens on a click of the
// line under it.
func chatShellCall(gtx C, th *ui.Theme, more *chatFold, it backend.ChatItem, live bool) D {
	tool := *it.Tool
	out := strings.Trim(tool.Output, "\n")
	tool.Output, tool.Title = "", ""
	row := it
	row.Tool = &tool
	if out == "" {
		return chatTool(gtx, th, row, live)
	}
	more.update(gtx)
	lines := strings.Split(out, "\n")
	hidden := len(lines) - chatShellLines
	if hidden > 0 && !more.open {
		lines = lines[:chatShellLines]
	}
	children := make([]layout.FlexChild, 0, len(lines)+1)
	for _, l := range lines {
		children = append(children, layout.Rigid(shellLine(th, l)))
	}
	if hidden > 0 {
		children = append(children, layout.Rigid(func(gtx C) D { return chatShellMore(gtx, th, more, hidden) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return chatTool(gtx, th, row, live) }),
		layout.Rigid(func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusSM) },
				func(gtx C) D {
					return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					})
				})
		}),
	)
}

// chatShellMore opens the lines of a shell command's output past the head,
// or shuts them again.
func chatShellMore(gtx C, th *ui.Theme, more *chatFold, hidden int) D {
	text := i18n.T("chat.view.shellMore", i18n.Args{"count": hidden})
	if more.open {
		text = i18n.T("chat.view.shellLess")
	}
	return layout.Inset{Top: ui.Space(1), Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return more.click.Layout(gtx, func(gtx C) D {
			fg := th.MutedForeground
			if more.click.Hovered() {
				fg = th.Foreground
			}
			return th.Text(ui.TextXS, text).In(fg).Layout(gtx)
		})
	})
}

// chatEarlierCalls shows a group's calls before its last chatCallsShown, or
// hides them again, a row on the group's branch.
func chatEarlierCalls(gtx C, th *ui.Theme, earlier *chatFold, hidden int) D {
	text := i18n.T("chat.view.earlierCalls", i18n.Args{"count": hidden})
	if earlier.open {
		text = i18n.T("chat.view.shellLess")
	}
	gtx.Constraints.Min.Y = gtx.Dp(chatToolRow)
	return earlier.click.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		if earlier.click.Hovered() {
			fg = th.Foreground
		}
		return layout.W.Layout(gtx, func(gtx C) D {
			return layout.Inset{Left: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, text).In(fg).Layout)
		})
	})
}

// chatHairline is a 1px rule in Border across the room it is given.
func chatHairline(gtx C, th *ui.Theme) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	return D{Size: size}
}

// chatBranch lays w to the right of a connector from the fold's line: a thin
// vertical line in Border under the fold's chevron, with a stub into the row,
// and ending there on the last row, as the sidebar's tree links its children.
func chatBranch(gtx C, th *ui.Theme, last bool, w layout.Widget) D {
	gutter := gtx.Dp(chatBranchGutter)
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Max.X = max(gtx.Constraints.Max.X-gutter, 0)
	gtx.Constraints.Min.X = min(gtx.Constraints.Min.X, gtx.Constraints.Max.X)
	d := w(gtx)
	call := macro.Stop()
	line := th.Over(th.Border)
	x, mid := gtx.Dp(chatBranchX), gtx.Dp(chatToolRow)/2
	bottom := d.Size.Y
	if last {
		bottom = mid + 1
	}
	paint.FillShape(gtx.Ops, line, clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(x+1, bottom)}.Op())
	paint.FillShape(gtx.Ops, line, clip.Rect{Min: image.Pt(x, mid), Max: image.Pt(gutter-gtx.Dp(ui.Space(1)), mid+1)}.Op())
	defer op.Offset(image.Pt(gutter, 0)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: image.Pt(d.Size.X+gutter, d.Size.Y)}
}

// chatNothing stands in for what a shut fold hides.
func chatNothing(C) D { return D{} }

// chatTool is one call: its glyph, its verb (chatVerb), what it acted on and
// the time it has run, and why it failed when it did; a call that went well
// says nothing more. The diff it made, if any, is under it. While it runs in a
// live turn its verb and target shimmer and its time counts; once it ends the
// time stands still, and a call whose end nobody reported shows none.
func chatTool(gtx C, th *ui.Theme, it backend.ChatItem, live bool) D {
	return chatToolWith(gtx, th, it, live, nil, 0)
}

// chatToolWith is chatTool drawn as the card it was opened as when opened is
// not nil, and its verb at least verbW wide, so a group's targets line up.
func chatToolWith(gtx C, th *ui.Theme, it backend.ChatItem, live bool, opened *chatOpened, verbW int) D {
	t := it.Tool
	if t == nil {
		return D{}
	}
	running := live && callRunning(it)
	line := func(gtx C) D {
		h := gtx.Dp(chatToolRow)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		return layout.Inset{Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			ink := th.MutedForeground
			if t.State == api.ToolFailed {
				ink = th.Destructive
			}
			verb := th.Text(ui.TextXS, callVerb(t, running)).In(ink).Truncated()
			target := th.Text(ui.TextXS, cleanLine(t.Title)).Mono()
			if running {
				// The band lights toward Foreground, so the target runs muted to show it.
				verb, target = verb.Shimmer(), target.Muted().Shimmer()
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if opened != nil && opened.mark != nil {
						return opened.mark(gtx)
					}
					return toolIcon(t.Name).Layout(gtx, ui.Space(3.5), ink)
				}),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D {
					if verbW > 0 {
						gtx.Constraints.Max.X = verbW
					}
					d := verb.Layout(gtx)
					d.Size.X = max(d.Size.X, verbW)
					return d
				}),
				ui.Gap(2),
				layout.Flexed(1, func(gtx C) D {
					// The text's own width, not the room's: the band sweeps the words.
					gtx.Constraints.Min.X = 0
					d := target.Layout(gtx)
					d.Size.X = gtx.Constraints.Max.X
					return d
				}),
				layout.Rigid(func(gtx C) D {
					if opened != nil {
						return opened.links(gtx)
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return chatToolTime(gtx, th, it, running) }),
						layout.Rigid(func(gtx C) D { return chatToolOutcome(gtx, th, t) }),
					)
				}),
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

// chatVerbMax caps a group's verb column: a tool's own name can run long.
var chatVerbMax = ui.Space(24)

// chatVerbWidth is the widest of tools' verbs, up to chatVerbMax.
func chatVerbWidth(gtx C, th *ui.Theme, tools []backend.ChatItem, live bool) int {
	w := 0
	gtx.Constraints.Min.X = 0
	for _, it := range tools {
		if it.Tool == nil {
			continue
		}
		m := op.Record(gtx.Ops)
		d := th.Text(ui.TextXS, callVerb(it.Tool, live && callRunning(it))).Layout(gtx)
		m.Stop()
		w = max(w, d.Size.X)
	}
	return min(w, gtx.Dp(chatVerbMax))
}

// chatToolTime is the time a call has run, counting while running, standing
// still once it ended, and nothing where its start or end is unknown.
func chatToolTime(gtx C, th *ui.Theme, it backend.ChatItem, running bool) D {
	if it.At <= 0 {
		return D{}
	}
	start := time.UnixMilli(it.At)
	var took time.Duration
	switch {
	case running:
		chatTick(gtx, start)
		took = gtx.Now.Sub(start)
	case it.Tool.Ended > 0:
		took = time.UnixMilli(it.Tool.Ended).Sub(start)
	default:
		return D{}
	}
	text := chatTook(took)
	if running {
		text = chatElapsed(took)
	}
	return layout.Inset{Left: ui.Space(2.5)}.Layout(gtx, th.Text(ui.TextXS, text).Mono().Muted().Layout)
}

// chatToolOutcome is why a call failed, the first line of its output in
// Destructive; a call that runs or went well shows nothing.
func chatToolOutcome(gtx C, th *ui.Theme, t *backend.ChatToolCall) D {
	if t.State != api.ToolFailed {
		return D{}
	}
	out := cleanLine(t.Output)
	if out == "" {
		out = i18n.T("chat.view.toolFailed")
	}
	return layout.Inset{Left: ui.Space(2.5)}.Layout(gtx, th.Text(ui.TextXS, out).Mono().In(th.Destructive).Layout)
}

// cleanLine is the first line of s, tabs expanded: a row has room for one.
func cleanLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return expandTabs(s)
}

// expandTabs is s with its tabs as spaces: the fonts have no glyph for one.
func expandTabs(s string) string { return strings.ReplaceAll(s, "\t", hunkTab) }

// chatDiff is a tool's diff as a small window on the window's background: the
// file's name and its +added -deleted over a hairline, then the hunks, each
// added or removed line in its tone, as a thread's hunk is (hunkTail).
func chatDiff(gtx C, th *ui.Theme, diff string) D {
	name, added, deleted, body := splitDiff(diff)
	children := make([]layout.FlexChild, 0, len(body)+2)
	if name != "" {
		children = append(children, layout.Rigid(func(gtx C) D { return chatDiffHead(gtx, th, name, added, deleted) }))
	}
	children = append(children, layout.Rigid(func(gtx C) D { return chatDiffLines(gtx, th, body) }))
	d := layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusMD) },
		func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...) })
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusMD), th.Over(th.Border))
	return d
}

// chatDiffLines is a diff's lines, added and removed ones in their tones;
// the hunks' "@@" headers are left out, the lines alone tell the change.
func chatDiffLines(gtx C, th *ui.Theme, body []string) D {
	return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		rows := make([]layout.FlexChild, 0, len(body))
		for _, line := range body {
			if strings.HasPrefix(line, "@@") {
				continue
			}
			rows = append(rows, layout.Rigid(func(gtx C) D { return hunkLine(gtx, th, line) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

func chatDiffHead(gtx C, th *ui.Theme, name string, added, deleted int) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, th.Text(ui.TextXS, name).Mono().Weight(font.Medium).Truncated().Layout),
					ui.Gap(2),
					layout.Rigid(th.DiffStat(added, deleted).Layout),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			h := gtx.Dp(1)
			gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, h)
			return ui.Fill(gtx, th.Over(th.Border), 0)
		}),
	)
}

// splitDiff is a unified diff's file name, its added and deleted lines and the
// lines under the file header. A "---" or "+++" is a header only before the
// first hunk; after it the line is the file's own.
func splitDiff(diff string) (name string, added, deleted int, body []string) {
	inHunk := false
	for line := range strings.SplitSeq(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case !inHunk && strings.HasPrefix(line, "--- "):
		case !inHunk && strings.HasPrefix(line, "+++ "):
			name = path.Base(strings.TrimSpace(line[4:]))
		case !inHunk && !strings.HasPrefix(line, "@@"):
		default:
			inHunk = inHunk || strings.HasPrefix(line, "@@")
			switch {
			case strings.HasPrefix(line, "+"):
				added++
			case strings.HasPrefix(line, "-"):
				deleted++
			}
			body = append(body, line)
		}
	}
	return name, added, deleted, body
}

// permitLabel is the provider's words for a permission's answer, or, for one
// it sends by kind alone (Claude Code, Codex), lich's.
func permitLabel(o api.ChatPermitOption) string {
	if o.Label != "" {
		return o.Label
	}
	switch o.Kind {
	case api.PermitAllowOnce:
		return i18n.T("chat.permit.allowOnce")
	case api.PermitAllowAlways:
		return i18n.T("chat.permit.allowAlways")
	case api.PermitRejectOnce:
		return i18n.T("chat.permit.rejectOnce")
	}
	return o.ID
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
				choice = permitLabel(o)
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
		b := box.Button(p.answer(it.ID, o.ID), permitLabel(o))
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

// chatVerbSep splits the catalog's list of verbs, which each language writes
// at its own length.
const chatVerbSep = "|"

// chatVerb is the verb a turn is named by while it runs: one of the language's
// own, picked by the session and the turn, so it holds for the whole turn and
// is the same on every frame.
func chatVerb(session string, turn int) string {
	verbs := strings.Split(i18n.T("chat.view.verbs"), chatVerbSep)
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%d", session, turn)
	return verbs[int(h.Sum32()%uint32(len(verbs)))]
}

// chatLive is the turn in flight: the spinner, then a verb of the turn shimmering
// with the time it has run, and Claude Code's Ctrl+B hint while the turn
// waits on a command it moves.
func chatLive(gtx C, th *ui.Theme, session string, r chatRow) D {
	row := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return chatSpinner(gtx, th) }),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			// A turn with no start to count from has no clock.
			var since time.Time
			if r.took > 0 {
				since = gtx.Now.Add(-r.took)
			}
			return chatLiveLabel(gtx, th, chatVerb(session, r.turn), th.MutedForeground, since)
		}),
	}
	if r.movable {
		row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, "· "+i18n.T("chat.shells.move")).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}

// chatAgent is an answer's text with its model, effort and time under it.
func (u *window) chatAgent(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	it := r.item
	src := it.Text
	notify := func(t ui.Toast) { u.toaster.Show(t) }
	c := p.copyOf(it.ID)
	return c.hover(gtx, func(gtx C, over bool) D {
		body := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return p.md.LayoutLinksWindow(gtx, th, it.ID, src, markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths}, p.window)
			}),
		}
		// a text another follows reads on into it: the run's last one keeps
		// the time and the copy, which takes the whole run
		if !r.more {
			stamped := it
			if r.runText != "" {
				stamped.Text = r.runText
			}
			body = append(body, ui.VGap(1), layout.Rigid(func(gtx C) D {
				return layout.W.Layout(gtx, func(gtx C) D {
					return u.chatStampRow(gtx, th, c, stamped.Text, func(gtx C) D { return u.chatAnswerStamp(gtx, th, p, stamped, u.providerName(sv)) }, over, false)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, body...)
	})
}

// chatElapsed is a running time in the window's own units: "6s", "1m 12s",
// "1h 04m".
func chatElapsed(d time.Duration) string {
	d = max(d, 0)
	unit := func(key string, n int) string { return i18n.T(key, i18n.Args{"count": n}) }
	switch {
	case d < time.Minute:
		return unit("session.age.second", int(d/time.Second))
	case d < time.Hour:
		return unit("session.age.minute", int(d/time.Minute)) + " " + unit("session.age.second", int(d%time.Minute/time.Second))
	}
	return unit("session.age.hour", int(d/time.Hour)) + " " + unit("session.age.minute", int(d%time.Hour/time.Minute))
}

// chatTook is a time that stands still: chatElapsed's, in milliseconds under a
// second, where a counter would read 0s.
func chatTook(d time.Duration) string {
	if d < time.Second {
		return i18n.T("session.age.millisecond", i18n.Args{"count": int(max(d, 0) / time.Millisecond)})
	}
	return chatElapsed(d)
}

// chatTick asks for the frame at which the time since start reads differently,
// the next second: a counter redraws once a second, not once a frame.
func chatTick(gtx C, since time.Time) {
	elapsed := max(gtx.Now.Sub(since), 0)
	gtx.Execute(op.InvalidateCmd{At: since.Add(elapsed.Truncate(time.Second) + time.Second)})
}
