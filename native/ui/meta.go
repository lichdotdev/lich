package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

const pillPadY unit.Dp = 3

// PillStyle is Content in a pill's room, with no edge.
type PillStyle struct {
	Content           layout.Widget
	PadLeft, PadRight unit.Dp
	theme             *Theme
}

func (th *Theme) Pill(content layout.Widget, padLeft, padRight unit.Dp) PillStyle {
	return PillStyle{Content: content, PadLeft: padLeft, PadRight: padRight, theme: th}
}

// Layout draws the content in the pill's room with no edge: a badge is text
// and glyph, never a box.
func (p PillStyle) Layout(gtx C) D {
	gtx.Constraints.Min = image.Point{}
	return layout.Inset{Left: p.PadLeft, Right: p.PadRight, Top: pillPadY, Bottom: pillPadY}.Layout(gtx, p.Content)
}

type keyCapGeometry struct {
	text         unit.Sp
	height, padX unit.Dp
}

var (
	keyCapLoose = keyCapGeometry{text: 11, height: 18, padX: 5}
	// keyCapInButton stays small and behind the button's word, so the action
	// reads first and the key second.
	keyCapInButton = keyCapGeometry{text: 10.5, height: 16, padX: 4}
)

const keyCapGap unit.Dp = 2

// keyCapEdgeMix is how far a key cap's edge leans from Border toward the
// foreground.
const keyCapEdgeMix = 0.12

// keyCapInkFill and keyCapInkText are the opacities of a cap's ink over the
// button it sits in: its face and its letters.
const (
	keyCapInkFill = 0.12
	keyCapInkText = 0.65
)

// KeyCapStyle is Keys as one cap per key ("Ctrl+Enter" is Ctrl and ↵):
// rounded, a hairline border with a thicker bottom one, on bg-muted, never
// narrower than tall. With Ink set it belongs to a button instead: smaller,
// faded Ink text on a faint wash of Ink, no border, so it reads on any fill.
type KeyCapStyle struct {
	Keys  string
	Ink   color.NRGBA
	theme *Theme
}

func (th *Theme) KeyCap(keys string) KeyCapStyle { return KeyCapStyle{Keys: keys, theme: th} }

func (k KeyCapStyle) Layout(gtx C) D {
	var children []layout.FlexChild
	for i, key := range splitKeys(k.Keys) {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: keyCapGap}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return k.cap(gtx, key) }))
	}
	macro := op.Record(gtx.Ops)
	d := layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	caps := macro.Stop()
	// A reader hears the chord as written, not each cap on its own.
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	semantic.LabelOp(k.Keys).Add(gtx.Ops)
	caps.Add(gtx.Ops)
	area.Pop()
	return d
}

// splitKeys cuts a "+"-joined chord into its keys, keeping a "+" key itself
// ("Ctrl++" is Ctrl and +).
func splitKeys(keys string) []string {
	parts := strings.Split(keys, "+")
	var out []string
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" && i+1 < len(parts) {
			out = append(out, "+")
			i++
			continue
		}
		out = append(out, parts[i])
	}
	return out
}

func (k KeyCapStyle) cap(gtx C, key string) D {
	th := k.theme
	if key == "Enter" {
		key = "↵"
	}
	geo := keyCapLoose
	text := th.Text(geo.text, key).Muted()
	if k.Ink.A != 0 {
		geo = keyCapInButton
		text = th.Text(geo.text, key).In(Alpha(k.Ink, keyCapInkText))
	}
	gtx.Constraints.Min = image.Point{}
	macro := op.Record(gtx.Ops)
	t := text.Leading(geo.text).Layout(gtx)
	label := macro.Stop()
	h := gtx.Dp(geo.height)
	size := image.Pt(max(t.Size.X+2*gtx.Dp(geo.padX), h), h)
	box := gtx
	box.Constraints = layout.Exact(size)
	if k.Ink.A != 0 {
		Fill(box, Alpha(k.Ink, keyCapInkFill), RadiusSM)
	} else {
		k.edge(box, size)
	}
	// The bottom edge is a pixel thicker: the label sits a pixel high to stay
	// centred on the cap's face.
	off := op.Offset(image.Pt((size.X-t.Size.X)/2, (size.Y-t.Size.Y)/2-gtx.Dp(0.5))).Push(gtx.Ops)
	label.Add(gtx.Ops)
	off.Pop()
	return D{Size: size, Baseline: (size.Y-t.Size.Y)/2 + t.Baseline}
}

func (k KeyCapStyle) edge(gtx C, size image.Point) {
	th := k.theme
	Fill(gtx, th.Over(th.Palette.Muted), RadiusSM)
	// A theme whose Border is its Muted, as Dark's is, would lose the cap's
	// edge into its fill: the edge leans toward the text to stay one.
	border := MixOKLab(th.Over(th.Border), th.Foreground, keyCapEdgeMix)
	InsetBorder(gtx, size, gtx.Dp(RadiusSM), border)
	r := gtx.Dp(RadiusSM)
	paint.FillShape(gtx.Ops, border, clip.Rect{Min: image.Pt(r, size.Y-2*gtx.Dp(1)), Max: image.Pt(size.X-r, size.Y-gtx.Dp(1))}.Op())
}

// MetaGlyphStyle is a size-3 glyph and its text-xs words, gap-1.
type MetaGlyphStyle struct {
	Icon  *icons.Icon
	Text  string
	Color color.NRGBA
	theme *Theme
}

func (th *Theme) MetaGlyph(ic *icons.Icon, text string, fg color.NRGBA) MetaGlyphStyle {
	return MetaGlyphStyle{Icon: ic, Text: text, Color: fg, theme: th}
}

func (m MetaGlyphStyle) Layout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return m.Icon.Layout(gtx, Space(3), m.Color) }),
		Gap(1),
		layout.Rigid(m.theme.Text(TextXS, m.Text).In(m.Color).Layout),
	)
}
