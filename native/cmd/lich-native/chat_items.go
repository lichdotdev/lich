package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"slices"
	"strconv"
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
	// chatAvatarSize and chatAvatarMark are mockup's 20px avatar and its mark.
	chatAvatarSize = ui.Space(5)
	chatAvatarMark = ui.Space(3)
)

// chatAvatarMix is how much of the provider's color the avatar's fill takes,
// the mockup's 22%.
const chatAvatarMix = 0.22

// chatCaretGlyph is the caret of an answer being written.
const chatCaretGlyph = "▍"

// chatClock is the short time under a message, the 24-hour layout the window's
// other clocks (the Git graph, a stopped console) use.
const chatClock = "15:04"

func (u *window) chatRow(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	switch r.kind {
	case chatRowWork:
		return u.chatWork(gtx, th, p.fold("work:"+strconv.Itoa(r.turn)), sv, r)
	case chatRowShell:
		return chatBranch(gtx, th, true, func(gtx C) D { return chatShellCall(gtx, th, p.fold("out:"+r.item.ID), r.item, r.live) })
	case chatRowLive:
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
		c := p.copyOf(it.ID)
		return c.hover(gtx, func(gtx C, over bool) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return u.sentCards(gtx, th, p, it.ID, it.Files) }),
				layout.Rigid(func(gtx C) D {
					if it.Text == "" {
						return D{}
					}
					return chatUser(gtx, th, it.Text)
				}),
				ui.VGap(1),
				layout.Rigid(func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D { return u.chatStampRow(gtx, th, p, c, it, over, true) })
				}),
			)
		})
	case api.ItemAgent:
		return u.chatAgent(gtx, th, p, sv, r)
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
		return chatQuestion(gtx, th, p, it)
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

// chatStampRow is the line under a message: its time, and while the pointer is
// over the message the copy button beside it (the person's message keeps its
// time at rest; an answer shows both only under the pointer), on the side of the time that
// faces the middle (at the left of a message the time is at the right). The line
// is as tall with the button as without it.
func (u *window) chatStampRow(gtx C, th *ui.Theme, p *chatPane, c *chatCopy, it backend.ChatItem, over, copyFirst bool) D {
	stamp := u.chatStamp
	if copyFirst {
		stamp = u.chatDatedStamp
	}
	gtx.Constraints.Min.Y = gtx.Dp(chatCopySlot)
	if !over && !copyFirst {
		// An answer's foot, its time and its copy button, shows only under the
		// pointer; the line it stands on is there either way.
		return D{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
	}
	row := []layout.FlexChild{layout.Rigid(func(gtx C) D { return stamp(gtx, th, p, it) })}
	if over {
		button := layout.Rigid(func(gtx C) D { return c.button(gtx, th, it.Text) })
		if copyFirst {
			row = append([]layout.FlexChild{button, ui.Gap(1)}, row...)
		} else {
			row = append(row, ui.Gap(1), button)
		}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
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

// chatWork is a turn's tool calls folded into one line, opening on a line per
// call under a connector. While a call runs the line shimmers, with the
// number of calls and the time the turn has run; once it is over it says how
// long the turn took, how many calls it made and when it ended. It stands
// open while its turn is in a run of calls and shuts when the run ends; a
// shell command's is a row of its own.
func (u *window) chatWork(gtx C, th *ui.Theme, f *chatFold, sv sessionView, r chatRow) D {
	f.follow(gtx.Now, r.calling)
	f.update(gtx)
	label := func(gtx C, fg color.NRGBA) D {
		if r.live {
			return chatLiveLabel(gtx, th, i18n.T("chat.view.toolCalls", i18n.Args{"count": len(r.tools)}), fg, gtx.Now.Add(-r.took))
		}
		args := i18n.Args{"duration": chatTook(r.took), "count": len(r.tools)}
		key := "chat.view.worked"
		if r.ended > 0 {
			key, args["time"] = "chat.view.workedAt", time.UnixMilli(r.ended).In(gtx.Now.Location()).Format(chatClock)
		}
		return th.Text(ui.TextSM, i18n.T(key, args)).In(fg).Layout(gtx)
	}
	// The line carries who works when it opens the turn, and a hairline runs
	// from it to the column's edge.
	var line []layout.FlexChild
	if r.avatar {
		tint, mark, _ := u.agentLook(sv.ID)
		line = append(line, layout.Rigid(func(gtx C) D { return chatAvatar(gtx, th, tint, mark) }), ui.Gap(2))
	}
	line = append(line,
		layout.Rigid(func(gtx C) D { return chatFoldLine(gtx, th, f, nil, label) }),
		ui.Gap(3),
		layout.Flexed(1, func(gtx C) D { return chatHairline(gtx, th) }),
	)
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...) })}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, append(children, chatCalls(gtx, th, f, r)...)...)
}

// chatCalls are a group's calls, each in a fold of its own that opens and
// shuts with the group. A call that comes once the group is drawn enters as a
// row does.
func chatCalls(gtx C, th *ui.Theme, f *chatFold, r chatRow) []layout.FlexChild {
	ids := make([]string, len(r.tools))
	for i, it := range r.tools {
		ids[i] = it.ID
	}
	f.enter.Update(gtx.Now, ids)
	children := make([]layout.FlexChild, 0, len(r.tools))
	for i, it := range r.tools {
		children = append(children, layout.Rigid(func(gtx C) D {
			return f.enter.LayoutFlat(gtx, it.ID, func(gtx C) D {
				fold := f.call(it.ID)
				if f.shut(gtx.Now) {
					return fold.Layout(gtx, false, chatNothing)
				}
				return fold.Layout(gtx, f.open, func(gtx C) D {
					var top unit.Dp
					if i == 0 {
						top = ui.Space(1)
					}
					return layout.Inset{Top: top}.Layout(gtx, func(gtx C) D {
						return chatBranch(gtx, th, i == len(r.tools)-1, func(gtx C) D { return chatTool(gtx, th, it, r.live) })
					})
				})
			})
		}))
	}
	return children
}

// chatShellCall is a shell command's call with its output under it, as
// Claude Code prints it, on a branch from the person's command: the row keeps
// the mark, not the output's first line, and output past chatShellLines opens
// on a click of the line under it.
func chatShellCall(gtx C, th *ui.Theme, more *chatFold, it backend.ChatItem, live bool) D {
	tool := *it.Tool
	out := strings.Trim(tool.Output, "\n")
	tool.Output = ""
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

// chatTool is one call: its glyph, name, what it acted on, the time it has
// run, and how it ended, with the diff it made under it. While it runs in a
// live turn its name and target shimmer and its time counts; once it ends the
// time stands still, and a call whose end nobody reported shows none.
func chatTool(gtx C, th *ui.Theme, it backend.ChatItem, live bool) D {
	t := it.Tool
	if t == nil {
		return D{}
	}
	running := live && callRunning(it)
	line := func(gtx C) D {
		h := gtx.Dp(chatToolRow)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			name := th.Text(ui.TextXS, t.Name).Weight(font.Medium)
			target := th.Text(ui.TextXS, cleanLine(t.Title)).Mono().Muted()
			if running {
				// The band lights toward Foreground, so the name runs muted to show it.
				name, target = name.Muted().Shimmer(), target.Shimmer()
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return toolIcon(t.Name).Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
				ui.Gap(2),
				layout.Rigid(name.Layout),
				ui.Gap(2.5),
				layout.Flexed(1, func(gtx C) D {
					// The text's own width, not the room's: the band sweeps the words.
					gtx.Constraints.Min.X = 0
					d := target.Layout(gtx)
					d.Size.X = gtx.Constraints.Max.X
					return d
				}),
				layout.Rigid(func(gtx C) D { return chatToolTime(gtx, th, it, running) }),
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

// chatToolOutcome is how a call ended: nothing while it runs, then a check in
// TonePass or a cross in Destructive with the first line of its output.
func chatToolOutcome(gtx C, th *ui.Theme, t *backend.ChatToolCall) D {
	out := cleanLine(t.Output)
	mark := func(ic *icons.Icon, c color.NRGBA) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2.5)}.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), c) })
		})
	}
	switch t.State {
	case api.ToolPending, api.ToolRunning:
		return D{}
	case api.ToolFailed:
		if out == "" {
			out = i18n.T("chat.view.toolFailed")
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, mark(icChatFail, th.Destructive), ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, out).Mono().In(th.Destructive).Layout))
	}
	row := []layout.FlexChild{mark(icCheck, th.TonePass)}
	if out != "" {
		row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, out).Mono().Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
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

// chatLive is the turn in flight: a flame, then a verb of the turn shimmering
// with the time it has run.
func chatLive(gtx C, th *ui.Theme, session string, r chatRow) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return chatFlame(gtx, th) }),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			// A turn with no start to count from has no clock.
			var since time.Time
			if r.took > 0 {
				since = gtx.Now.Add(-r.took)
			}
			return chatLiveLabel(gtx, th, chatVerb(session, r.turn), th.MutedForeground, since)
		}),
	)
}

// chatAgent is an answer's text with its time under it, and over it, for the
// first one of a turn, who is speaking. The one being written ends in a blinking
// caret.
func (u *window) chatAgent(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	it := r.item
	// The caret stays lit while the text grows, and blinks once it rests.
	src := it.Text + chatCaretGlyph
	if !r.live || !p.stream.streaming(gtx, it) {
		src = chatSource(gtx, it.Text, r.live)
	}
	notify := func(t ui.Toast) { u.toaster.Show(t) }
	var children []layout.FlexChild
	if r.lead {
		children = append(children, layout.Rigid(func(gtx C) D { return u.chatWho(gtx, th, sv, it, r.bare) }), ui.VGap(1.5))
	}
	c := p.copyOf(it.ID)
	return c.hover(gtx, func(gtx C, over bool) D {
		body := append(slices.Clone(children),
			layout.Rigid(func(gtx C) D {
				return p.md.LayoutLinks(gtx, th, it.ID, src, markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths})
			}),
			ui.VGap(1),
			layout.Rigid(func(gtx C) D {
				return layout.W.Layout(gtx, func(gtx C) D { return u.chatStampRow(gtx, th, p, c, it, over, false) })
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, body...)
	})
}

// chatWho is who speaks: the provider's mark in the color of the session's
// card, on a fill of that color mixed into the surface, its name, and muted the
// model and effort that wrote the answer, as the item says. The session's
// options are never read here: they are what the chat is set to now, and an
// answer written before a change keeps the model that wrote it.
func (u *window) chatWho(gtx C, th *ui.Theme, sv sessionView, it backend.ChatItem, bare bool) D {
	tint, mark, _ := u.agentLook(sv.ID)
	name := sv.Kind
	if info, ok := u.model.providers.find(sv.Kind); ok && info.name != "" {
		name = info.name
	}
	var running []string
	for _, part := range []string{it.Model, it.Effort} {
		if part != "" {
			running = append(running, part)
		}
	}
	var row []layout.FlexChild
	if !bare {
		row = append(row, layout.Rigid(func(gtx C) D { return chatAvatar(gtx, th, tint, mark) }), ui.Gap(2))
	}
	row = append(row, layout.Rigid(th.Text(ui.TextSM, name).Weight(font.Medium).Layout))
	if len(running) > 0 {
		row = append(row, ui.Gap(2), layout.Rigid(th.Text(ui.TextSM, "· "+strings.Join(running, " · ")).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}

// chatAvatar is the provider's mark in tint on a rounded square of tint mixed
// into the surface.
func chatAvatar(gtx C, th *ui.Theme, tint color.NRGBA, mark *icons.Icon) D {
	px := gtx.Dp(chatAvatarSize)
	size := image.Pt(px, px)
	paint.FillShape(gtx.Ops, ui.MixOKLab(th.Background, tint, chatAvatarMix), clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusMD)).Op(gtx.Ops))
	inner := gtx.Dp(chatAvatarMark)
	off := op.Offset(image.Pt((px-inner)/2, (px-inner)/2)).Push(gtx.Ops)
	mark.Layout(gtx, chatAvatarMark, tint)
	off.Pop()
	return D{Size: size}
}

// chatSource is an answer's text as drawn: while it is being written, ending
// in the caret.
func chatSource(gtx C, text string, live bool) string {
	if !live {
		return text
	}
	return text + chatCaret(gtx)
}

// chatCaret is the caret that ends the answer being written: there for half a
// second and gone for the next, asking for a frame at each change and not
// between them.
func chatCaret(gtx C) string {
	const half = 500 * time.Millisecond
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Truncate(half).Add(half)})
	if gtx.Now.UnixMilli()/half.Milliseconds()%2 == 0 {
		return chatCaretGlyph
	}
	return ""
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
