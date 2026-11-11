package extrender

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/extension"
)

func pngOf(t *testing.T, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = []uint8{c.R, c.G, c.B, c.A}[i%4]
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestEveryComposedComponentDrawsWithEveryPropSet(t *testing.T) {
	pic := pngOf(t, color.NRGBA{R: 200, A: 255})
	text := extension.Label.New(extension.LabelProps{Text: "x"})
	empty := extension.EmptyStateProps{
		Icon: "folder", Title: "Nothing", Description: "Open one", IconSize: 32, IconGap: 8, DescriptionGap: 4,
		ActionsGap: 12, TitleSize: 16, DescriptionSize: 12, TitleLeading: 20, TitleWeight: extension.FontMedium,
		MaxLines: 2, Anchor: extension.AnchorTopLeft, Align: extension.AlignCenter, Pad: 8,
	}
	tree := extension.Stack.New(extension.GapProps{Gap: 4},
		extension.EmptyState.New(empty, extension.Button.New(extension.ButtonProps{Text: "Open"})),
		extension.InlineEmptyState.New(empty),
		extension.Avatar.New(extension.AvatarProps{Image: pic, Initial: "L", Size: 24, Background: "accent", Foreground: "foreground"}),
		extension.Avatar.New(extension.AvatarProps{Initial: "M", Size: 24}),
		extension.SectionHeading.New(extension.SectionHeadingProps{Text: "Group", Size: 11, Weight: extension.FontSemibold, Leading: 14, PadTop: 1, PadRight: 2, PadBottom: 3, PadLeft: 4}),
		extension.MarkedText.New(extension.MarkedTextProps{Segments: []extension.MarkedSegment{{Text: "a"}, {Text: "b", Marked: true}}}),
		extension.Image.New(extension.ImageProps{Data: pic, Fit: extension.ImageCover, Position: extension.AnchorBottomRight, Width: 20, Height: 10}),
		extension.Image.New(extension.ImageProps{Data: []byte("not a picture")}),
		extension.FileIcon.New(extension.FileIconProps{Path: "main.go", Size: 16}),
		extension.FileIcon.New(extension.FileIconProps{Path: "Makefile"}),
		extension.SkeletonBar.New(extension.SkeletonBarProps{Width: 40, Line: 12, Height: 20}),
		extension.SkeletonText.New(extension.SkeletonTextProps{Size: 12, Width: 60}),
		extension.SkeletonShape.New(extension.NoProps{}, text),
		extension.Box.New(extension.BoxProps{Height: 40}, extension.SkeletonFill.New(extension.SkeletonFillProps{MaxRows: 3, PadY: 2},
			extension.SkeletonRow.New(extension.SkeletonRowProps{PadX: 4, PadY: 2}, text))),
		extension.SkeletonFill.New(extension.SkeletonFillProps{}),
		extension.Row.New(extension.GapProps{Gap: 8}, text, text),
		extension.Columns.New(extension.GapProps{Gap: 8}, text, text),
		extension.FixedColumn.New(extension.FixedColumnProps{Width: 50}, text),
		extension.AlignedEnd.New(extension.NoProps{}, text),
		extension.Spanning.New(extension.NoProps{}, text),
		extension.Box.New(extension.BoxProps{Height: 40}, extension.SplitPane.New(extension.SplitPaneProps{
			Direction: extension.DirectionColumn, Ratio: 0.4, MinRatio: 0.1, MaxRatio: 0.9, Gap: 4, MinFirst: 10, MinSecond: 10, OnResize: "resize",
		}, text)),
		extension.Box.New(extension.BoxProps{Height: 40}, extension.ThinScrollArea.New(extension.ScrollProps{Direction: extension.DirectionRow}, text, text)),
		extension.Box.New(extension.BoxProps{Height: 40}, extension.ThinScrollArea.New(extension.ScrollProps{}, text)),
	)
	r := New(nil, func(extension.EventData) {})
	router := new(input.Router)
	frame(r, router, tree)
	frame(r, router, tree)
}

func TestAPictureIsDecodedOncePerChangeOfItsBytes(t *testing.T) {
	r := New(nil, func(extension.EventData) {})
	router := new(input.Router)
	red, blue := pngOf(t, color.NRGBA{R: 255, A: 255}), pngOf(t, color.NRGBA{B: 255, A: 255})
	draw := func(data []byte) *pictureState {
		frame(r, router, extension.Image.New(extension.ImageProps{Data: data}))
		return r.state[""].(*pictureState)
	}
	first := draw(red).image
	if first == nil {
		t.Fatal("a PNG did not decode")
	}
	if again := draw(bytes.Clone(red)).image; again != first {
		t.Error("the same bytes were decoded again")
	}
	if other := draw(blue).image; other == first {
		t.Error("new bytes kept the old picture")
	}
}

func TestADraggedSplitPaneSendsItsRatio(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	panel := extension.Label.New(extension.LabelProps{Text: "x"})
	tree := extension.SplitPane.New(extension.SplitPaneProps{Ratio: 0.5, Gap: 8, OnResize: "resize"}, panel, panel)
	router := new(input.Router)
	frame(r, router, tree)
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(200, 50)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(298, 50)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(298, 50)},
	)
	frame(r, router, tree)
	if len(got) != 1 || got[0].Handler != "resize" || got[0].Ratio != 0.75 {
		t.Fatalf("events = %+v, want one resize to 0.75", got)
	}
	frame(r, router, tree)
	if s := r.state[""].(*splitState); s.ratio != 0.75 {
		t.Errorf("ratio after the drag, before the extension answers = %v, want the dragged 0.75", s.ratio)
	}
}
