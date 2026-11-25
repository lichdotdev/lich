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

// The agent's task list stands in the activity strip over the composer while
// the latest turn's list has a task left: the task running behind the
// sidebar's 3x3 and the count. A click opens every task in a panel over the
// composer, in the slash list's place.

const (
	// chatPlanPanel is the task list's panel's kind in chatTools.
	chatPlanPanel = "plan"
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

func planWords(e api.ChatPlanEntry) string {
	if e.Status == api.PlanInProgress && e.ActiveForm != "" {
		return e.ActiveForm
	}
	return e.Content
}

// chatPlanState is the strip's task line.
type chatPlanState struct {
	toggle ui.Clickable
}

// chatPlan draws the task running of session id's list and the count, lit
// while its panel is open; nothing while no task is left. The task running
// waves and shimmers only while the turn runs.
func (u *window) chatPlan(gtx C, th *ui.Theme, p *chatPane, id string, working bool) D {
	entries := chatPlanOf(u.model.chat.of(id))
	if len(entries) == 0 {
		return D{}
	}
	s, open := &p.plan, p.tools.kind == chatPlanPanel
	cur := planCurrent(entries)
	return s.toggle.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = gtx.Dp(ui.Space(8))
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if !open && !s.toggle.Hovered() {
					return D{Size: gtx.Constraints.Min}
				}
				return ui.Fill(gtx, th.Accent, ui.RadiusMD)
			},
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.Y = gtx.Dp(ui.Space(8))
					return layout.W.Layout(gtx, func(gtx C) D {
						tail := func(gtx C) D { return chatPlanTail(gtx, th, s, open, entries) }
						return chatPlanRow(gtx, th, entries[cur], working, tail)
					})
				})
			},
		)
	})
}

// chatPlanTail ends the running task's line, right after its words: the count
// and the chevron, lit under the pointer or while the panel is open.
func chatPlanTail(gtx C, th *ui.Theme, s *chatPlanState, open bool, entries []api.ChatPlanEntry) D {
	fg := th.MutedForeground
	if s.toggle.Hovered() || open {
		fg = th.Foreground
	}
	chevron := icChatClosed
	if open {
		chevron = icChatOpen
	}
	count := i18n.T("chat.plan.count", i18n.Args{"done": planDone(entries), "total": len(entries)})
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, count).In(fg).Layout)
		}),
		layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), fg) }),
	)
}

// planActs opens or shuts the task list's panel on a click of its line, and
// shuts it once no task is left.
func (u *window) planActs(gtx C, p *chatPane, id string) {
	if len(chatPlanOf(u.model.chat.of(id))) == 0 {
		if p.tools.kind == chatPlanPanel {
			p.tools.shut()
		}
		return
	}
	if !p.plan.toggle.Clicked(gtx) {
		return
	}
	if p.tools.kind == chatPlanPanel {
		p.tools.shut()
		return
	}
	p.openPanel(chatPlanPanel)
}

// planView is every task of the list, each behind its 3x3, and how many are
// done.
func (u *window) planView(gtx C, th *ui.Theme, p *chatPane, id string, working bool) D {
	t := &p.tools
	entries := chatPlanOf(u.model.chat.of(id))
	cur := planCurrent(entries)
	lines := make([]layout.Widget, len(entries))
	for i, e := range entries {
		lines[i] = func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.Y = gtx.Dp(pickerRowH)
				return layout.W.Layout(gtx, func(gtx C) D { return chatPlanTask(gtx, th, e, i == cur, working) })
			})
		}
	}
	head := func(gtx C) D {
		done := i18n.T("chat.plan.done", i18n.Args{"done": planDone(entries), "total": len(entries)})
		return toolsHead(gtx, th, i18n.T("chat.plan.title"), done, nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

// chatPlanTask is a task of the panel: the one running as the strip draws
// it, one done muted and one pending in the foreground.
func chatPlanTask(gtx C, th *ui.Theme, e api.ChatPlanEntry, current, working bool) D {
	if current {
		return chatPlanRow(gtx, th, e, working, nil)
	}
	label := th.Text(ui.TextSM, planWords(e)).Truncated()
	if e.Status == api.PlanCompleted {
		label = label.Muted()
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return planGlyph(gtx, th, e, working) }),
		ui.Gap(2),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min.X = 0
			return label.Layout(gtx)
		}),
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

// chatPlanRow is the task running: its words in semibold Foreground,
// shimmering while the turn runs, behind its 3x3, and tail after them.
func chatPlanRow(gtx C, th *ui.Theme, e api.ChatPlanEntry, working bool, tail layout.Widget) D {
	label := th.Text(ui.TextSM, planWords(e)).Truncated().In(th.Foreground).Weight(font.SemiBold)
	if working {
		label = label.Shimmer()
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
