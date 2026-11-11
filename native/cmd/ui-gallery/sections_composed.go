package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// composedState is the live state of the components the product composes
// from the kit's primitives.
type composedState struct {
	emptyActions [4]widget.Clickable
	sample       *ui.AvatarImage
}

func (g *gallery) composedSection(gtx C, th *ui.Theme) D {
	kids := g.emptyStates(th)
	kids = append(kids, g.avatars(th)...)
	return stack(4, kids...)(gtx)
}

func (g *gallery) emptyStates(th *ui.Theme) []layout.Widget {
	action := func(i int, text string) layout.Widget {
		return th.Button(&g.emptyActions[i], text).Layout
	}
	return []layout.Widget{
		caption(th, "EmptyState: glyph, description, action"),
		height(200, th.EmptyState(g.icon("folder-open"), "No project open", "Open a folder to get started.", action(0, "Open project")).Layout),
		caption(th, "EmptyState: title and action only"),
		height(160, th.EmptyState(nil, "Nothing here", "", action(1, "Reload")).Layout),
		caption(th, "InlineEmptyState: description and action"),
		th.InlineEmptyState("This file cannot be previewed", "It is a binary format.", action(2, "Open")).Layout,
		caption(th, "InlineEmptyState: action only"),
		th.InlineEmptyState("No preview", "", action(3, "Reveal")).Layout,
	}
}

// sampleImage is a small gradient, standing in for a decoded picture.
func sampleImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 96, 96))
	for y := range 96 {
		for x := range 96 {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 2), G: uint8(y * 2), B: 200, A: 255})
		}
	}
	return img
}

func (g *gallery) avatars(th *ui.Theme) []layout.Widget {
	if g.sample == nil {
		g.sample = ui.NewAvatarImage(sampleImage())
	}
	tinted := th.Avatar(nil, "W", 32)
	tinted.Background, tinted.Foreground = color.NRGBA{R: 186, G: 58, B: 80, A: 255}, th.Foreground
	return []layout.Widget{
		caption(th, "Avatar: image at 16, 24 and 40; neutral initial; tinted initial"),
		row(3,
			th.Avatar(g.sample, "L", 16).Layout,
			th.Avatar(g.sample, "L", 24).Layout,
			th.Avatar(g.sample, "L", 40).Layout,
			th.Avatar(nil, "M", 24).Layout,
			tinted.Layout,
		),
	}
}
