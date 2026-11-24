package main

import (
	"image"
	"slices"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The agent's task list stands over the composer, in the queue's place, while
// the latest turn's list has a task left: folded, the task running behind the
// sidebar's 3x3; unfolded, the one before it and the next too, and a line
// counting the rest.

const (
	// planShown is how many tasks the list draws; the rest go into its count.
	planShown = 3
	// planGlyphSize is the 3x3's side. It scales as a whole, as the chat's
	// spinner does (chatSpinnerSize), so a zoom keeps its proportions.
	planGlyphSize unit.Dp = 14
	// planPendingFade is how much of MutedForeground a pending task's cells
	// keep: cellFade's rest level is too faint to read without a lit cell.
	planPendingFade = 0.22
	planWaveStep    = 110 * time.Millisecond
)

// planWavePath is the diagonals of the 3x3 the running task's wave lights,
// from the top left corner to the bottom right, then a step dark.
var planWavePath = [...][]int{{0}, {1, 3}, {2, 4, 6}, {5, 7}, {8}, {}}

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

// planCurrent is the task being done; the first one left where none says it
// runs, -1 where none is left.
func planCurrent(entries []api.ChatPlanEntry) int {
	if i := slices.IndexFunc(entries, func(e api.ChatPlanEntry) bool { return e.Status == api.PlanInProgress }); i >= 0 {
		return i
	}
	return slices.IndexFunc(entries, planLeft)
}

// planWindow is the tasks drawn: the one before the current, it and the
// next, slid back from the end so there are planShown where the list has them.
func planWindow(entries []api.ChatPlanEntry) (from, to int) {
	from = max(0, min(planCurrent(entries)-1, len(entries)-planShown))
	return from, min(len(entries), from+planShown)
}

func planWords(e api.ChatPlanEntry) string {
	if e.Status == api.PlanInProgress && e.ActiveForm != "" {
		return e.ActiveForm
	}
	return e.Content
}

// chatPlanState is the list's fold.
type chatPlanState struct {
	toggle ui.Clickable
	open   bool
}

// chatPlan draws session id's task list over the composer: folded, the task
// running and the count; a click unfolds the tasks around it. The task running
// waves and shimmers only while the turn runs.
func (u *window) chatPlan(gtx C, th *ui.Theme, s *chatPlanState, id string, working bool) D {
	entries := chatPlanOf(u.model.chat.of(id))
	if len(entries) == 0 {
		s.open = false
		return D{}
	}
	if s.toggle.Clicked(gtx) {
		s.open = !s.open
	}
	cur := planCurrent(entries)
	from, to := cur, cur+1
	if s.open {
		from, to = planWindow(entries)
	}
	rows := make([]layout.FlexChild, 0, planShown+1)
	for i := from; i < to; i++ {
		tail := layout.Widget(nil)
		if i == cur {
			tail = func(gtx C) D { return chatPlanTail(gtx, th, s, entries) }
		}
		rows = append(rows, layout.Rigid(func(gtx C) D { return chatPlanRow(gtx, th, entries[i], i == cur, working, tail) }))
	}
	if more := len(entries) - (to - from); s.open && more > 0 {
		rows = append(rows, layout.Rigid(func(gtx C) D { return chatPlanMore(gtx, th, entries, more) }))
	}
	return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return s.toggle.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: chatTextInset, Right: ui.Space(1), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		})
	})
}

// chatPlanTail ends the running task's line, right after its words: the count
// while folded, and the chevron, lit under the pointer.
func chatPlanTail(gtx C, th *ui.Theme, s *chatPlanState, entries []api.ChatPlanEntry) D {
	fg := th.MutedForeground
	if s.toggle.Hovered() {
		fg = th.Foreground
	}
	chevron := icChatClosed
	if s.open {
		chevron = icChatOpen
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if s.open {
				return D{}
			}
			count := i18n.T("chat.plan.count", i18n.Args{"done": planDone(entries), "total": len(entries)})
			return layout.Inset{Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, count).In(fg).Layout)
		}),
		layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), fg) }),
	)
}

func planDone(entries []api.ChatPlanEntry) int {
	done := 0
	for _, e := range entries {
		if !planLeft(e) {
			done++
		}
	}
	return done
}

func chatPlanRow(gtx C, th *ui.Theme, e api.ChatPlanEntry, current, working bool, tail layout.Widget) D {
	label := th.Text(ui.TextSM, planWords(e)).Muted().Truncated()
	if current {
		label = label.In(th.Foreground).Weight(font.SemiBold)
		if working {
			label = label.Shimmer()
		}
	}
	return layout.Inset{Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return planGlyph(gtx, th, e, working) }),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.X = 0
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if tail == nil {
					return D{}
				}
				return layout.Inset{Left: ui.Space(2)}.Layout(gtx, tail)
			}),
		)
	})
}

// chatPlanMore counts the tasks the list leaves out, under the tasks' words,
// with how many of all are done.
func chatPlanMore(gtx C, th *ui.Theme, entries []api.ChatPlanEntry, more int) D {
	text := i18n.T("chat.plan.more", i18n.Args{"more": more, "done": planDone(entries), "total": len(entries)})
	return layout.Inset{Left: planGlyphSize + ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, text).Muted().Truncated().Layout)
}

// planGlyph is a task's 3x3: a block in TonePass done, the wave in Primary
// running, the ring of a session waiting in ToneWait once the turn stops
// with it open, and faint cells pending.
func planGlyph(gtx C, th *ui.Theme, e api.ChatPlanEntry, working bool) D {
	side := max(spriteCells, gtx.Dp(planGlyphSize))
	tone, a := th.MutedForeground, cellAlphas{}
	switch {
	case e.Status == api.PlanCompleted:
		tone = th.TonePass
		for c := range a {
			a[c] = 1
		}
	case e.Status == api.PlanInProgress && working:
		var next time.Time
		a, next = planWaveAt(gtx.Now)
		tone = th.Primary
		nextFrame(gtx, th, next.Sub(gtx.Now))
	case e.Status == api.PlanInProgress:
		tone = th.ToneWait
		for _, c := range ringPath {
			a[c] = 1
		}
	default:
		for c := range a {
			a[c] = planPendingFade
		}
	}
	drawCells(gtx, side, th.Background, tone, a)
	return D{Size: image.Pt(side, side)}
}

// planWaveLevels is the wave with its front on step k of planWavePath and
// the two diagonals behind it fading.
func planWaveLevels(k int64) cellLevels {
	var l cellLevels
	n := int64(len(planWavePath))
	for j := range int64(spinnerTrail) {
		for _, c := range planWavePath[((k-j)%n+n)%n] {
			l[c] = max(l[c], uint8(spinnerTrail-j))
		}
	}
	return l
}

// planWaveAt is the wave read off the clock at now, gliding between its
// steps as the sidebar's snake does (spinnerAt), and when it is next due.
func planWaveAt(now time.Time) (cellAlphas, time.Time) {
	k := floorDiv(now.UnixNano(), int64(planWaveStep))
	start := time.Unix(0, k*int64(planWaveStep))
	a := planWaveLevels(k - spinnerGlideSteps).faded()
	for j := k - spinnerGlideSteps + 1; j < k; j++ {
		a = glide(a, planWaveLevels(j), planWaveStep)
	}
	return glide(a, planWaveLevels(k), now.Sub(start)), earliest(start.Add(planWaveStep), now.Add(glideFrame))
}
