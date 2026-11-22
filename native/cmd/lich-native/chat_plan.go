package main

import (
	"image"
	"image/color"
	"slices"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The agent's task list stands over the composer, in the queue's place, while
// the latest turn's list has a task left: one line, the count and the task
// running, which a click unfolds into the whole list and folds back.

var (
	icPlan        = icons.Lucide("list-todo")
	icPlanDone    = icons.Lucide("circle-check")
	icPlanRunning = icons.Lucide("loader-circle")
	icPlanPending = icons.Lucide("circle")
)

// chatPlanMax caps the unfolded list, which scrolls past it.
const chatPlanMax = unit.Dp(160)

// chatPlanState is the list's fold and its scroll.
type chatPlanState struct {
	toggle ui.Clickable
	open   bool
	list   widget.List
}

// chatPlanOf is the task list to show of items: the latest turn's, while one
// of its tasks is not done.
func chatPlanOf(items []backend.ChatItem) []api.ChatPlanEntry {
	last := 0
	for _, it := range items {
		last = max(last, it.Turn)
	}
	for _, it := range slices.Backward(items) {
		if it.Kind != api.ItemPlan {
			continue
		}
		if it.Turn != last || it.Plan == nil || !slices.ContainsFunc(it.Plan.Entries, planLeft) {
			return nil
		}
		return it.Plan.Entries
	}
	return nil
}

func planLeft(e api.ChatPlanEntry) bool { return e.Status != api.PlanCompleted }

// planRunning is the task being done, its words while it runs; the first one
// left where none says it runs.
func planRunning(entries []api.ChatPlanEntry) string {
	i := slices.IndexFunc(entries, func(e api.ChatPlanEntry) bool { return e.Status == api.PlanInProgress })
	if i < 0 {
		i = slices.IndexFunc(entries, planLeft)
	}
	if i < 0 {
		return ""
	}
	return planWords(entries[i])
}

func planWords(e api.ChatPlanEntry) string {
	if e.Status == api.PlanInProgress && e.ActiveForm != "" {
		return e.ActiveForm
	}
	return e.Content
}

// chatPlan draws session id's task list over the composer; the task running
// shimmers only while the turn does.
func (u *window) chatPlan(gtx C, th *ui.Theme, s *chatPlanState, id string, working bool) D {
	entries := chatPlanOf(u.model.chat.of(id))
	if len(entries) == 0 {
		s.open = false
		return D{}
	}
	if s.toggle.Clicked(gtx) {
		s.open = !s.open
	}
	return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return chatPlanLine(gtx, th, s, entries, working) }),
			layout.Rigid(func(gtx C) D {
				if !s.open {
					return D{}
				}
				return chatPlanList(gtx, th, s, entries, working)
			}),
		)
	})
}

// chatPlanLine is the folded line: the glyph, "Tasks 2/5" and the task
// running, shimmering as the live line's verb does; unfolded, the list shows
// that task and the line keeps the count.
func chatPlanLine(gtx C, th *ui.Theme, s *chatPlanState, entries []api.ChatPlanEntry, working bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	done := 0
	for _, e := range entries {
		if !planLeft(e) {
			done++
		}
	}
	fg := th.MutedForeground
	if s.toggle.Hovered() {
		fg = th.Foreground
	}
	chevron := icChatClosed
	if s.open {
		chevron = icChatOpen
	}
	return s.toggle.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(1), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icPlan.Layout(gtx, chatCrumbGlyph, fg) }),
				ui.Gap(2),
				layout.Rigid(th.Text(ui.TextSM, i18n.T("chat.plan.count", i18n.Args{"done": done, "total": len(entries)})).In(fg).Layout),
				ui.Gap(2),
				layout.Flexed(1, func(gtx C) D {
					if s.open {
						return D{Size: image.Pt(gtx.Constraints.Min.X, 0)}
					}
					return planShimmer(th.Text(ui.TextSM, planRunning(entries)).Truncated(), working).Layout(gtx)
				}),
				ui.Gap(1),
				layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), fg) }),
			)
		})
	})
}

// chatPlanList is the unfolded list, no taller than chatPlanMax.
func chatPlanList(gtx C, th *ui.Theme, s *chatPlanState, entries []api.ChatPlanEntry, working bool) D {
	s.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(chatPlanMax))
	gtx.Constraints.Min.Y = 0
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.ThinScrollArea(&s.list).Layout(gtx, len(entries), func(gtx C, i int) D {
			return chatPlanRow(gtx, th, entries[i], working)
		})
	})
}

func chatPlanRow(gtx C, th *ui.Theme, e api.ChatPlanEntry, working bool) D {
	ic, label := icPlanPending, th.Text(ui.TextSM, planWords(e)).Muted().Truncated()
	switch e.Status {
	case api.PlanCompleted:
		ic = icPlanDone
	case api.PlanInProgress:
		ic, label = icPlanRunning, planShimmer(label.In(th.Foreground), working)
	}
	return layout.Inset{Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, chatCrumbGlyph, th.MutedForeground) }),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.X = 0
				d := label.Layout(gtx)
				if e.Status == api.PlanCompleted {
					strike(gtx, d.Size, th.MutedForeground)
				}
				return d
			}),
		)
	})
}

func planShimmer(l ui.LabelStyle, working bool) ui.LabelStyle {
	if working {
		return l.Shimmer()
	}
	return l
}

// strike draws a line through a label of size, at its middle.
func strike(gtx C, size image.Point, ink color.NRGBA) {
	y := size.Y / 2
	defer clip.Rect{Min: image.Pt(0, y), Max: image.Pt(size.X, y+max(gtx.Dp(1), 1))}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: ink}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
