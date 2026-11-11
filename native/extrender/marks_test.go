package extrender

import (
	"testing"

	"gioui.org/io/input"

	"github.com/lichdotdev/lich/extension"
)

func TestEveryMarkDrawsWithEveryPropSet(t *testing.T) {
	text := extension.Label.New(extension.LabelProps{Text: "x"})
	c := extension.ColorProps{Color: "tone-pass"}
	tree := extension.Stack.New(extension.GapProps{},
		extension.StatusDot.New(c), extension.HollowDot.New(c), extension.BeatingDot.New(c), extension.MarkDot.New(c),
		extension.Swatch.New(c), extension.CornerDot.New(c), extension.PulsingCornerDot.New(c),
		extension.Dot.New(extension.DotProps{Color: "primary", Size: 8}),
		extension.DotAt.New(extension.DotAtProps{X: 4, Y: 4, Radius: 3, Color: "tone-wait"}),
		extension.PulseRing.New(extension.PulseRingProps{X: 12, Y: 12, Radius: 4, Reach: 6, Color: "tone-wait"}),
		extension.NotifyDot.New(extension.SizeProps{Width: 24, Height: 24}),
		extension.GaugeBar.New(extension.GaugeBarProps{Percent: 35, Tone: "tone-pass"}),
		extension.SegmentBar.New(extension.SegmentBarProps{Segments: []extension.Segment{{Share: 60, Color: "muted-foreground"}, {Share: 40}}}),
		extension.DonutGauge.New(extension.DonutGaugeProps{Size: 16, Percent: 60, Color: "tone-wait"}),
		extension.GaugeRow.New(extension.GaugeRowProps{LabelWidth: 80, ReadingWidth: 40, LeftWidth: 60, Label: text,
			Bar: extension.GaugeBar.New(extension.GaugeBarProps{Percent: 12}), Reading: text, Left: text}),
		extension.CountBadge.New(extension.CountBadgeProps{Count: 7}),
		extension.OutlineBadge.New(extension.TextProps{Text: "Draft"}),
		extension.AbbrBadge.New(extension.AbbrBadgeProps{Abbr: "GO", Background: "card", Foreground: "tone-pass"}),
		extension.Tag.New(extension.TagProps{Text: "lich", Color: "foreground"}),
		extension.KeyHint.New(extension.KeyHintProps{Keys: "Esc", Label: "Close"}),
		extension.Box.New(extension.BoxProps{Height: 130}, extension.DropHint.New(extension.DropHintProps{
			Icon: "paperclip", Before: "Attach to ", Label: "main", After: ".", Detail: "Pasted at the prompt"})),
		extension.Tray.New(extension.TrayProps{Fill: "accent"}, text),
		extension.DropTray.New(extension.DropTrayProps{Fill: "accent", Edge: "tone-wait"}, text),
		extension.Box.New(extension.BoxProps{Width: 80, Height: 20}, extension.FillIf.New(extension.FillIfProps{Color: "accent", Radius: 4})),
		extension.Checker.New(extension.CheckerProps{Width: 40, Height: 20, Cell: 6}),
		extension.Fillet.New(extension.FilletProps{X: 0, Y: 4, Radius: 8, Right: true, Color: "accent"}),
		extension.FillRRect.New(extension.FillRRectProps{X: 2, Y: 2, Width: 40, Height: 20, Radius: 4, Color: "accent"}),
		extension.InsetBorder.New(extension.InsetBorderProps{Width: 40, Height: 20, Radius: 4, Color: "border"}),
		extension.InsetRing.New(extension.InsetRingProps{Width: 40, Height: 20, Radius: 4, StrokeWidth: 2, Color: "ring"}),
		extension.StrokeArc.New(extension.StrokeArcProps{X: 10, Y: 10, Radius: 6, Width: 2, Start: 0.5, Sweep: 3, Color: "primary"}),
		extension.DashedBorder.New(extension.DashedBorderProps{Width: 60, Height: 30, Radius: 6, Color: "muted-foreground", DashLength: 4, GapLength: 3}),
		extension.DashedRing.New(extension.RingProps{X: 4, Y: 4, Width: 60, Height: 30, Radius: 6, StrokeWidth: 1.5, Color: "tone-wait", Halo: "#ffffffb0"}),
		extension.MarchingRing.New(extension.RingProps{X: 4, Y: 4, Width: 60, Height: 30, Radius: 6, StrokeWidth: 1.5, Color: "tone-wait", Halo: "#ffffffb0"}),
		extension.DashedCircle.New(extension.DashedCircleProps{X: 20, Y: 20, Outer: 14, Stroke: 2, Dash: 3, Color: "muted-foreground"}),
	)
	r := New(nil, func(extension.EventData) {})
	router := new(input.Router)
	frame(r, router, tree)
	frame(r, router, tree)
}
