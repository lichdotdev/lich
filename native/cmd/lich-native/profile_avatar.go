package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	xdraw "golang.org/x/image/draw"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// profileLook is what an avatar draws: the initial on the profile's color, or
// its image over that color.
type profileLook struct {
	name, slug, color string
	image             *avatarImage
}

// avatarImage is a profile's decoded image and the sizes it has been drawn at.
// Gio only scales linearly, which shimmers at the 8x a 16dp avatar shrinks a
// 128px image by, so each size is scaled once on the CPU. It is only drawn on
// the UI goroutine, which owns sized.
type avatarImage struct {
	src   image.Image
	sized map[int]paint.ImageOp
}

// decodeAvatar is a profile's image ready to draw; nil when it has none, or one
// the window cannot decode, which then shows its initial.
func decodeAvatar(p backend.Profile) *avatarImage {
	if len(p.Image) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(p.Image))
	if err != nil {
		log.Printf("profile %s: image: %v", p.Slug, err)
		return nil
	}
	return &avatarImage{src: img, sized: map[int]paint.ImageOp{}}
}

func (a *avatarImage) at(px int) paint.ImageOp {
	if op, ok := a.sized[px]; ok {
		return op
	}
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), a.src, a.src.Bounds(), xdraw.Over, nil)
	op := paint.NewImageOp(dst)
	a.sized[px] = op
	return op
}

// profileColor is the color a profile is shown in: the card color it picked,
// else the same hash the scope selector paints a project with, so a profile
// keeps its color across screens. A name the palette does not know is no pick.
func profileColor(slug, picked string) color.NRGBA {
	if c, ok := cardColors[picked]; ok {
		return c
	}
	return projectSwatch(slug)
}

// profileAvatar is a profile's image, or its initial, on its color.
func profileAvatar(gtx C, th *ui.Theme, look profileLook, size unit.Dp) D {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	bg := profileColor(look.slug, look.color)
	ui.Fill(gtx, bg, size/2)
	if look.image == nil {
		label := th.On(bg).Text(ui.TextXS, profileInitial(look.name)).Weight(font.SemiBold)
		return layout.Center.Layout(gtx, label.Layout)
	}
	defer clip.Ellipse{Max: image.Pt(px, px)}.Push(gtx.Ops).Pop()
	look.image.at(px).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return D{Size: image.Pt(px, px)}
}
