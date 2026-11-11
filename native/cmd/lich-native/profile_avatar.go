package main

import (
	"bytes"
	"image/color"
	"image/png"
	"log"

	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// profileLook is what an avatar draws: the initial on the profile's color, or
// its image over that color.
type profileLook struct {
	name, slug, color string
	image             *ui.AvatarImage
}

// decodeAvatar is a profile's image ready to draw; nil when it has none, or one
// the window cannot decode, which then shows its initial.
func decodeAvatar(p backend.Profile) *ui.AvatarImage {
	if len(p.Image) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(p.Image))
	if err != nil {
		log.Printf("profile %s: image: %v", p.Slug, err)
		return nil
	}
	return ui.NewAvatarImage(img)
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
	a := th.Avatar(look.image, profileInitial(look.name), size)
	a.Background, a.Foreground = profileColor(look.slug, look.color), th.Foreground
	return a.Layout(gtx)
}
