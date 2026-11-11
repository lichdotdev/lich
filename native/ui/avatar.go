package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	xdraw "golang.org/x/image/draw"
)

// AvatarImage is a decoded picture and the sizes it has been drawn at. Gio
// only scales linearly, which shimmers at the 8x a 16dp avatar shrinks a
// 128px image by, so each size is scaled once on the CPU. It is only drawn on
// the UI goroutine, which owns sized.
type AvatarImage struct {
	src   image.Image
	sized map[int]paint.ImageOp
}

// NewAvatarImage wraps a decoded picture for Avatar.
func NewAvatarImage(src image.Image) *AvatarImage {
	return &AvatarImage{src: src, sized: map[int]paint.ImageOp{}}
}

func (a *AvatarImage) at(px int) paint.ImageOp {
	if img, ok := a.sized[px]; ok {
		return img
	}
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), a.src, a.src.Bounds(), xdraw.Over, nil)
	img := paint.NewImageOp(dst)
	a.sized[px] = img
	return img
}

// AvatarStyle is a Size disc on Background: Image when there is one, else
// Initial in Foreground.
type AvatarStyle struct {
	Image                  *AvatarImage
	Initial                string
	Size                   unit.Dp
	Background, Foreground color.NRGBA
	theme                  *Theme
}

// Avatar is the neutral disc: Accent ground, muted initial.
func (th *Theme) Avatar(img *AvatarImage, initial string, size unit.Dp) AvatarStyle {
	return AvatarStyle{Image: img, Initial: initial, Size: size, Background: th.Accent, Foreground: th.MutedForeground, theme: th}
}

func (a AvatarStyle) Layout(gtx C) D {
	px := gtx.Dp(a.Size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	Fill(gtx, a.Background, a.Size/2)
	if a.Image == nil {
		return layout.Center.Layout(gtx, a.theme.Text(TextXS, a.Initial).Weight(font.SemiBold).In(a.Foreground).Layout)
	}
	defer clip.Ellipse{Max: image.Pt(px, px)}.Push(gtx.Ops).Pop()
	a.Image.at(px).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return D{Size: image.Pt(px, px)}
}
