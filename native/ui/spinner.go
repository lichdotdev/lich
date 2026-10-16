package ui

import (
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/ui/icons"
)

// spinLap is animate-spin's: one turn a second.
const spinLap = time.Second

var icSpinner = icons.Lucide("loader-circle")

// Spinner is LoaderCircle under animate-spin, size square, and asks for the
// next frame while it is drawn.
func Spinner(gtx layout.Context, size unit.Dp, fg color.NRGBA) layout.Dimensions {
	turn := float64(gtx.Now.UnixNano()%int64(spinLap)) / float64(spinLap)
	half := float32(gtx.Dp(size)) / 2
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), float32(turn*2*math.Pi))).Push(gtx.Ops).Pop()
	gtx.Execute(op.InvalidateCmd{})
	return icSpinner.Layout(gtx, size, fg)
}
