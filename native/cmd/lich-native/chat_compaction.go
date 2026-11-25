package main

import (
	"image"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The compaction's live line, mockup B: the fold glyph squeezing in place of
// the flame, its bar of chatCompactSegments under the words, as wide as the
// quota meter's.
const (
	chatCompactSegments = 11
	chatCompactSqueeze  = 1400 * time.Millisecond
	// chatCompactSquash is how far the glyph folds at the middle of a squeeze.
	chatCompactSquash = 0.45
)

var (
	chatCompactBarW  = ui.Space(55)
	chatCompactGlyph = ui.Space(4)
)

// compactRunning is whether the turn in flight is compacting: the last turn of
// items holds a compaction still running.
func compactRunning(items []backend.ChatItem) bool {
	if len(items) == 0 {
		return false
	}
	last := items[len(items)-1].Turn
	return slices.ContainsFunc(items, func(it backend.ChatItem) bool {
		return it.Turn == last && it.Compaction != nil && it.Compaction.State == api.CompactionRunning
	})
}

// compactTurn is whether turn was the person's compaction: its marker closes it,
// with no line of its own.
func compactTurn(items []backend.ChatItem, turn int) bool {
	return slices.ContainsFunc(items, func(it backend.ChatItem) bool {
		return it.Turn == turn && it.Compaction != nil && !it.Compaction.Auto && it.Compaction.State != api.CompactionCancelled
	})
}

// drawnCompaction is whether a compaction item has a marker: one running is
// the live line's, and a cancelled one did nothing.
func drawnCompaction(it backend.ChatItem) bool {
	cc := it.Compaction
	return cc != nil && cc.State != api.CompactionRunning && cc.State != api.CompactionCancelled
}

// chatCompactSquish is the glyph's height at now, 1 and down to
// 1-chatCompactSquash halfway through a squeeze.
func chatCompactSquish(now time.Time) float32 {
	phase := float64(now.UnixNano()%int64(chatCompactSqueeze)) / float64(chatCompactSqueeze)
	return float32(1 - chatCompactSquash*(1-math.Cos(2*math.Pi*phase))/2)
}

// chatCompacting is the live line of a turn that compacts: the glyph, the
// words shimmering with the time it has run, and under them a wave across
// the bar, which never fills: no provider says how far along it is.
func chatCompacting(gtx C, th *ui.Theme, r chatRow) D {
	var since time.Time
	if r.took > 0 {
		since = gtx.Now.Add(-r.took)
	}
	glyph := func(gtx C) D {
		size := gtx.Dp(chatCompactGlyph)
		half := float32(size) / 2
		defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(half, half), f32.Pt(1, chatCompactSquish(gtx.Now)))).Push(gtx.Ops).Pop()
		th.Loop(gtx)
		return icCompacting.Layout(gtx, chatCompactGlyph, th.MutedForeground)
	}
	line := layout.Flex{Alignment: layout.Middle}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return line.Layout(gtx,
				layout.Rigid(glyph),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D {
					return chatLiveLabel(gtx, th, i18n.T("chat.compaction.running"), th.MutedForeground, since)
				}),
			)
		}),
		ui.VGap(2.5),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: chatCompactGlyph + ui.Space(2)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(chatCompactBarW))
				return th.WaveBar(gtx, chatCompactSegments, th.MutedForeground)
			})
		}),
	)
}

// chatCompactTitle is what a finished compaction says: compacted, by itself
// for one the provider ran on its own, or not, and after it the tokens on
// each side, or why it failed.
func chatCompactTitle(cc *api.ChatCompaction) (title, detail string) {
	switch {
	case cc.State == api.CompactionFailed:
		return i18n.T("chat.compaction.failed"), cc.Error
	case cc.Auto:
		title = i18n.T("chat.compaction.auto")
	default:
		title = i18n.T("chat.compaction.done")
	}
	if cc.Before > 0 && cc.After > 0 {
		l := i18n.Current()
		detail = i18n.T("chat.compaction.tokens", i18n.Args{"before": compactCount(cc.Before, l), "after": compactCount(cc.After, l)})
	}
	return title, detail
}

// chatCompaction is a finished compaction's marker, set in the conversation
// where it happened, alone in the column's middle: the hook
// output in its tooltip, and the summary opening under it on a click.
func (u *window) chatCompaction(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem) D {
	cc := it.Compaction
	title, detail := chatCompactTitle(cc)
	f := p.fold("compact:" + it.ID)
	f.update(gtx)
	opens := cc.Summary != ""
	label := func(gtx C) D {
		fg := th.MutedForeground
		if opens && f.click.Hovered() {
			fg = th.Foreground
		}
		head := th.Text(ui.TextSM, title).Weight(font.Medium)
		if cc.State == api.CompactionFailed {
			head = head.In(th.Destructive)
		}
		row := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return icCompacting.Layout(gtx, ui.Space(3.5), fg) }),
			ui.Gap(1.5),
			layout.Rigid(head.Layout),
		}
		if detail != "" {
			row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, "· "+detail).In(fg).Layout))
		}
		if opens {
			ic := icChatClosed
			if f.open {
				ic = icChatOpen
			}
			row = append(row, ui.Gap(1), layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	}
	mark := label
	if opens {
		mark = func(gtx C) D { return f.click.Layout(gtx, label) }
	}
	if cc.Hooks != "" {
		inner := mark
		mark = func(gtx C) D { return th.Tooltip(&u.root, p.stamp("compact:"+it.ID), cc.Hooks).Layout(gtx, inner) }
	}
	// the mark stands alone in the column's middle, set apart by the turn's space
	rule := func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, rule), ui.Gap(3), layout.Rigid(mark), ui.Gap(3), layout.Flexed(1, rule))
		}),
		layout.Rigid(func(gtx C) D {
			if !opens || f.shut(gtx.Now) {
				return f.body.Layout(gtx, false, chatNothing)
			}
			return f.body.Layout(gtx, f.open, func(gtx C) D {
				return layout.Inset{Top: ui.Space(2)}.Layout(gtx, th.Text(ui.TextSM, expandTabs(cc.Summary)).Muted().Wrapped().Layout)
			})
		}),
	)
}
