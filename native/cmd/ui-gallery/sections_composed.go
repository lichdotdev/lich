package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// composedState is the live state of the components the product composes
// from the kit's primitives.
type composedState struct {
	emptyActions [4]widget.Clickable
	sample       *ui.AvatarImage

	skeletonButton widget.Clickable
}

func (g *gallery) composedSection(gtx C, th *ui.Theme) D {
	kids := g.emptyStates(th)
	kids = append(kids, g.avatars(th)...)
	kids = append(kids, g.headings(th)...)
	kids = append(kids, g.skeletons(th)...)
	return ui.Stack(4, kids...)(gtx)
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
		ui.Row(3,
			th.Avatar(g.sample, "L", 16).Layout,
			th.Avatar(g.sample, "L", 24).Layout,
			th.Avatar(g.sample, "L", 40).Layout,
			th.Avatar(nil, "M", 24).Layout,
			tinted.Layout,
		),
	}
}

const headingCompact unit.Sp = 10.5

func (g *gallery) headings(th *ui.Theme) []layout.Widget {
	semi := th.SectionHeading("Settings group")
	semi.Weight, semi.Inset = font.SemiBold, layout.Inset{Left: ui.Space(3)}
	compact := th.SectionHeading("Shortcuts group")
	compact.Size, compact.Weight = headingCompact, font.SemiBold
	return []layout.Widget{
		caption(th, "SectionHeading: default, semibold with inset, compact"),
		ui.Stack(1, th.SectionHeading("Description").Layout, semi.Layout, compact.Layout),
	}
}

func (g *gallery) skeletons(th *ui.Theme) []layout.Widget {
	button := th.Button(&g.skeletonButton, "Open in session")
	fill := func(gtx C) D {
		gtx.Constraints.Max.Y = gtx.Dp(96)
		return ui.SkeletonFill{MaxRows: 6, PadY: ui.Space(1)}.Layout(gtx, func(gtx C, i int) D {
			return ui.SkeletonRow{PadX: ui.Space(3), PadY: ui.Space(2)}.Layout(gtx, th.SkeletonText(ui.TextSM, ui.Space(30+float32(i%3)*10)).Layout)
		})
	}
	return []layout.Widget{
		caption(th, "SkeletonBar and SkeletonText: bars of text-sm and text-xs lines"),
		ui.Row(3, th.SkeletonBar(ui.Space(16), ui.Space(5), ui.Space(3.5)).Layout, th.SkeletonBar(ui.Space(10), ui.Space(4), ui.Space(3)).Layout),
		ui.Row(3, th.SkeletonText(ui.TextSM, ui.Space(24)).Layout, th.SkeletonText(ui.TextXS, ui.Space(16)).Layout),
		caption(th, "SkeletonShape: a button's measure"),
		th.SkeletonShape(button.Layout).Layout,
		caption(th, "SkeletonFill and SkeletonRow: rows down to the bottom"),
		fill,
	}
}
