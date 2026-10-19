package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// itemHeights is each list item's height as last laid out, so the scrollbar
// measures the files instead of guessing: layout.List sizes the list as its
// visible items' average times their count, and a 6-line card beside a
// 250-line one swings that guess with every scroll.
type itemHeights struct {
	key heightsKey
	px  []int // 0 for an item not laid out since the key last changed
}

// heightsKey is what resizes every card at once, offscreen ones included.
type heightsKey struct {
	width         int
	split, hiding bool
	bulk          int
	family        string
	size          unit.Sp
}

func (h *itemHeights) reset(key heightsKey, count int) {
	if key == h.key && len(h.px) == count {
		return
	}
	h.key, h.px = key, make([]int, count)
}

// scrollSpan is where a viewport of the given height sits over the items, as
// fractions of their total height, and that total. An item not laid out yet
// counts as the average of those that were.
func scrollSpan(px []int, first, offset, viewport int) (start, end float32, total int) {
	measured, sum := 0, 0
	for _, h := range px {
		if h > 0 {
			measured++
			sum += h
		}
	}
	if measured == 0 {
		return 0, 1, 0
	}
	average := sum / measured
	above := 0
	for i, h := range px {
		if h == 0 {
			h = average
		}
		total += h
		if i < first {
			above += h
		}
	}
	top := float32(above+offset) / float32(total)
	start = min(max(top, 0), 1)
	end = min(max(top+float32(viewport)/float32(total), 0), 1)
	return start, end, total
}

// scrollArea is th.ScrollArea's list for items of very different heights:
// the bar is drawn from the heights measured in p.heights, not layout.List's
// estimate.
func (p *reviewPanel) scrollArea(gtx C, th *ui.Theme, count int, item layout.ListElement) {
	p.heights.reset(heightsKey{width: gtx.Constraints.Max.X, split: p.split, hiding: p.hiding,
		bulk: p.bulk, family: p.family, size: p.size}, count)
	bar := th.ScrollArea(&p.list).ScrollbarStyle
	barWidth := gtx.Dp(bar.Width())
	inner := gtx
	inner.Constraints.Max.X = max(inner.Constraints.Max.X-barWidth, 0)
	inner.Constraints.Min.X = max(inner.Constraints.Min.X-barWidth, 0)
	dims := p.list.List.Layout(inner, count, func(gtx C, i int) D {
		dims := item(gtx, i)
		p.heights.px[i] = dims.Size.Y
		return dims
	})
	pos := p.list.Position
	start, end, total := scrollSpan(p.heights.px, pos.First, pos.Offset, dims.Size.Y)
	gtx.Constraints.Min = image.Pt(dims.Size.X+barWidth, dims.Size.Y)
	layout.E.Layout(gtx, func(gtx C) D { return bar.Layout(gtx, layout.Vertical, start, end) })
	if d := p.list.ScrollDistance(); d != 0 {
		p.list.Position.Offset += int(d * float32(total))
	}
}
