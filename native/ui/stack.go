package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
)

// Stack lays widgets top to bottom, gap Space units apart (flex-col gap-N).
func Stack(gap float32, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, spaced(VGap, gap, layout.Rigid, ws)...)
	}
}

// Row lays widgets side by side, gap Space units apart, centred across the
// row (flex items-center gap-N).
func Row(gap float32, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, spaced(Gap, gap, layout.Rigid, ws)...)
	}
}

// Columns gives each widget an equal share of the width, gap Space units
// apart. A widget keeps its own width inside its share: one that wants all of
// it asks for it.
func Columns(gap float32, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		even := func(w layout.Widget) layout.FlexChild {
			return layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min = image.Point{}
				return w(gtx)
			})
		}
		return layout.Flex{Alignment: layout.Start}.Layout(gtx, spaced(Gap, gap, even, ws)...)
	}
}

func spaced(gapOf func(float32) layout.FlexChild, gap float32, child func(layout.Widget) layout.FlexChild, ws []layout.Widget) []layout.FlexChild {
	kids := make([]layout.FlexChild, 0, 2*len(ws))
	for i, w := range ws {
		if i > 0 {
			kids = append(kids, gapOf(gap))
		}
		kids = append(kids, child(w))
	}
	return kids
}

// FixedColumn is a Flex child exactly width wide, so columns line up down a
// list.
func FixedColumn(width unit.Dp, w layout.Widget) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		px := gtx.Dp(width)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		return w(gtx)
	})
}

// AlignEnd is w against the right edge of the width it is given.
func AlignEnd(w layout.Widget) layout.Widget {
	return func(gtx C) D { return layout.E.Layout(gtx, w) }
}

// Spanning is w taking all the room a Flexed gives it, as text does, so a
// Rigid after it lands at the row's end and not beside it.
func Spanning(w layout.Widget) layout.Widget {
	return func(gtx C) D {
		dims := w(gtx)
		dims.Size.X = max(dims.Size.X, gtx.Constraints.Min.X)
		return dims
	}
}
