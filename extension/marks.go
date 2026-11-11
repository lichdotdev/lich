package extension

var (
	StatusDot        = component[ColorProps]("statusDot")
	HollowDot        = component[ColorProps]("hollowDot")
	BeatingDot       = component[ColorProps]("beatingDot")
	MarkDot          = component[ColorProps]("markDot")
	Swatch           = component[ColorProps]("swatch")
	CornerDot        = component[ColorProps]("cornerDot")
	PulsingCornerDot = component[ColorProps]("pulsingCornerDot")
	Dot              = component[DotProps]("dot")
	DotAt            = component[DotAtProps]("dotAt")
	PulseRing        = component[PulseRingProps]("pulseRing")
	NotifyDot        = component[SizeProps]("notifyDot")

	GaugeBar   = component[GaugeBarProps]("gaugeBar")
	SegmentBar = component[SegmentBarProps]("segmentBar")
	DonutGauge = component[DonutGaugeProps]("donutGauge")
	GaugeRow   = component[GaugeRowProps]("gaugeRow")

	CountBadge   = component[CountBadgeProps]("countBadge")
	OutlineBadge = component[TextProps]("outlineBadge")
	AbbrBadge    = component[AbbrBadgeProps]("abbrBadge")
	Tag          = component[TagProps]("tag")
	KeyHint      = component[KeyHintProps]("keyHint")
	DropHint     = component[DropHintProps]("dropHint")

	Tray         = component[TrayProps]("tray")
	DropTray     = component[DropTrayProps]("dropTray")
	FillIf       = component[FillIfProps]("fillIf")
	Checker      = component[CheckerProps]("checker")
	Fillet       = component[FilletProps]("fillet")
	FillRRect    = component[FillRRectProps]("fillRRect")
	InsetBorder  = component[InsetBorderProps]("insetBorder")
	InsetRing    = component[InsetRingProps]("insetRing")
	StrokeArc    = component[StrokeArcProps]("strokeArc")
	DashedBorder = component[DashedBorderProps]("dashedBorder")

	DashedRing   = component[RingProps]("dashedRing")
	MarchingRing = component[RingProps]("marchingRing")
	DashedCircle = component[DashedCircleProps]("dashedCircle")
)

// Shapes that draw at a point or in a rectangle take X, Y, Width and Height
// in dp, counted from the top left of their parent.

type ColorProps struct {
	Color string `json:"color,omitempty"`
}

type DotProps struct {
	Color string  `json:"color,omitempty"`
	Size  float64 `json:"size"`
}

type DotAtProps struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Color  string  `json:"color,omitempty"`
}

// PulseRingProps beats a ring out from the circle at X, Y, Reach wide.
type PulseRingProps struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Reach  float64 `json:"reach"`
	Color  string  `json:"color,omitempty"`
}

// SizeProps is the props of NotifyDot, which sits in the top right corner of
// a button of this size.
type SizeProps struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type GaugeBarProps struct {
	Percent int    `json:"percent"`
	Tone    string `json:"tone,omitempty"`
}

type Segment struct {
	Share int    `json:"share"`
	Color string `json:"color,omitempty"`
}

// SegmentBarProps splits its bar by the Share of each segment.
type SegmentBarProps struct {
	Segments []Segment `json:"segments"`
}

type DonutGaugeProps struct {
	Size    float64 `json:"size"`
	Percent int     `json:"percent"`
	Color   string  `json:"color,omitempty"`
}

// GaugeRowProps lays out four slots in a row: Label, Bar sharing what is
// left, Reading and Left, each of the three on the sides in a fixed width.
type GaugeRowProps struct {
	LabelWidth   float64 `json:"labelWidth"`
	ReadingWidth float64 `json:"readingWidth"`
	LeftWidth    float64 `json:"leftWidth"`
	Label        Node    `json:"label"`
	Bar          Node    `json:"bar"`
	Reading      Node    `json:"reading"`
	Left         Node    `json:"left"`
}

// CountBadgeProps draws nothing for a Count of 0.
type CountBadgeProps struct {
	Count int `json:"count"`
}

type AbbrBadgeProps struct {
	Abbr       string `json:"abbr"`
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
}

type TagProps struct {
	Text  string `json:"text"`
	Color string `json:"color,omitempty"`
}

type KeyHintProps struct {
	Keys  string `json:"keys"`
	Label string `json:"label"`
}

// DropHintProps fills its parent with a veil, Label in bold between Before
// and After; Detail shows only when the parent is tall enough.
type DropHintProps struct {
	Icon   Icon   `json:"icon"`
	Before string `json:"before,omitempty"`
	Label  string `json:"label"`
	After  string `json:"after,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// TrayProps draws its children on a rounded Fill.
type TrayProps struct {
	Fill string `json:"fill,omitempty"`
}

// DropTrayProps is a Tray with a rim of Edge around it.
type DropTrayProps struct {
	Fill string `json:"fill,omitempty"`
	Edge string `json:"edge,omitempty"`
}

// FillIfProps fills its parent, and draws nothing for a transparent Color.
type FillIfProps struct {
	Color  string  `json:"color,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

type CheckerProps struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Cell   float64 `json:"cell"`
}

// FilletProps is the concave corner joining a tab to its bar, curving to
// the right when Right is set.
type FilletProps struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Right  bool    `json:"right,omitempty"`
	Color  string  `json:"color,omitempty"`
}

type FillRRectProps struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Radius float64 `json:"radius,omitempty"`
	Color  string  `json:"color,omitempty"`
}

type InsetBorderProps struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Radius float64 `json:"radius,omitempty"`
	Color  string  `json:"color,omitempty"`
}

type InsetRingProps struct {
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Radius      float64 `json:"radius,omitempty"`
	StrokeWidth float64 `json:"strokeWidth"`
	Color       string  `json:"color,omitempty"`
}

// StrokeArcProps strokes an arc of the circle at X, Y, from Start sweeping
// Sweep radians, clockwise from three o'clock.
type StrokeArcProps struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Width  float64 `json:"width"`
	Start  float64 `json:"start,omitempty"`
	Sweep  float64 `json:"sweep"`
	Color  string  `json:"color,omitempty"`
}

type DashedBorderProps struct {
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Radius     float64 `json:"radius,omitempty"`
	Color      string  `json:"color,omitempty"`
	DashLength float64 `json:"dashLength"`
	GapLength  float64 `json:"gapLength"`
}

// RingProps is the props of DashedRing and MarchingRing, whose dashes
// travel around it. Halo outlines each dash.
type RingProps struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Radius      float64 `json:"radius,omitempty"`
	StrokeWidth float64 `json:"strokeWidth"`
	Color       string  `json:"color,omitempty"`
	Halo        string  `json:"halo,omitempty"`
}

type DashedCircleProps struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Outer  float64 `json:"outer"`
	Stroke float64 `json:"stroke"`
	Dash   float64 `json:"dash"`
	Color  string  `json:"color,omitempty"`
}
