package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

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

	picture *ui.ImageData

	splitH, splitV           ui.SplitPane
	splitHRatio, splitVRatio float64
}

func (g *gallery) composedSection(gtx C, th *ui.Theme) D {
	kids := g.emptyStates(th)
	kids = append(kids, g.avatars(th)...)
	kids = append(kids, g.headings(th)...)
	kids = append(kids, g.skeletons(th)...)
	kids = append(kids, g.markedTexts(th)...)
	kids = append(kids, g.images(th)...)
	kids = append(kids, g.splitPanes(th)...)
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

func (g *gallery) markedTexts(th *ui.Theme) []layout.Widget {
	line := func(segs ...ui.MarkedSegment) layout.Widget { return th.MarkedText(segs).Layout }
	return []layout.Widget{
		caption(th, "MarkedText: no mark, one mark, several marks"),
		line(ui.MarkedSegment{Text: "func resolve(path string) error {"}),
		line(ui.MarkedSegment{Text: "func "}, ui.MarkedSegment{Text: "resolve", Marked: true}, ui.MarkedSegment{Text: "(path string) error {"}),
		line(ui.MarkedSegment{Text: "err", Marked: true}, ui.MarkedSegment{Text: " := resolve("}, ui.MarkedSegment{Text: "err", Marked: true}, ui.MarkedSegment{Text: ")"}),
	}
}

// sampleData decodes the sample through DecodeImage, the bytes path; a failure
// falls back to the image path, so the page draws either way.
func (g *gallery) sampleData() *ui.ImageData {
	if g.picture != nil {
		return g.picture
	}
	src := sampleImage()
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err == nil {
		if decoded, err := ui.DecodeImage(buf.Bytes()); err == nil {
			g.picture = decoded
			return g.picture
		}
	}
	g.picture = ui.NewImageData(src)
	return g.picture
}

func (g *gallery) images(th *ui.Theme) []layout.Widget {
	src := g.sampleData()
	box := func(fit widget.Fit, name string, w, h unit.Dp) layout.Widget {
		img := th.Image(src)
		img.Fit, img.Width, img.Height = fit, w, h
		return ui.Stack(1, caption(th, name), func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, th.Over(th.Palette.Muted), ui.RadiusSM) },
				img.Layout)
		})
	}
	return []layout.Widget{
		caption(th, "Image: fits in a 96x64 box, and at its own size"),
		ui.Row(3,
			box(widget.Contain, "contain", 96, 64),
			box(widget.Cover, "cover", 96, 64),
			box(widget.Fill, "fill", 96, 64),
			box(widget.Contain, "96x96", 96, 96),
		),
	}
}

func (g *gallery) splitPanes(th *ui.Theme) []layout.Widget {
	if g.splitHRatio == 0 {
		g.splitHRatio, g.splitVRatio = 0.5, 0.5
		g.splitH.MinRatio, g.splitH.MaxRatio = 0.1, 0.9
		g.splitV.MinRatio, g.splitV.MaxRatio = 0.1, 0.9
	}
	panel := func(text string) layout.Widget {
		return func(gtx C) D {
			ui.Fill(gtx, th.Over(th.Palette.Muted), ui.RadiusMD)
			return layout.Center.Layout(gtx, th.Text(ui.TextXS, text).Muted().Layout)
		}
	}
	split := func(s *ui.SplitPane, axis layout.Axis, ratio *float64, a, b string) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Max.Y = gtx.Dp(120)
			gtx.Constraints.Min = gtx.Constraints.Max
			return s.Layout(gtx, axis, ratio, gtx.Dp(ui.Space(1.5)), gtx.Dp(48), gtx.Dp(48), panel(a), panel(b))
		}
	}
	return []layout.Widget{
		caption(th, "SplitPane: drag the gap; 48dp minimum per side, 10% to 90%"),
		split(&g.splitH, layout.Horizontal, &g.splitHRatio, "first", "second"),
		split(&g.splitV, layout.Vertical, &g.splitVRatio, "top", "bottom"),
	}
}
