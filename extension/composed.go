package extension

var (
	EmptyState       = component[EmptyStateProps]("emptyState")
	InlineEmptyState = component[EmptyStateProps]("inlineEmptyState")
	Avatar           = component[AvatarProps]("avatar")
	SectionHeading   = component[SectionHeadingProps]("sectionHeading")
	MarkedText       = component[MarkedTextProps]("markedText")
	Image            = component[ImageProps]("image")
	FileIcon         = component[FileIconProps]("fileIcon")

	SkeletonBar   = component[SkeletonBarProps]("skeletonBar")
	SkeletonText  = component[SkeletonTextProps]("skeletonText")
	SkeletonShape = component[NoProps]("skeletonShape")
	SkeletonFill  = component[SkeletonFillProps]("skeletonFill")
	SkeletonRow   = component[SkeletonRowProps]("skeletonRow")

	Stack       = component[GapProps]("stack")
	Row         = component[GapProps]("row")
	Columns     = component[GapProps]("columns")
	FixedColumn = component[FixedColumnProps]("fixedColumn")
	// AlignedEnd is the "alignEnd" component, named apart from the AlignEnd value.
	AlignedEnd = component[NoProps]("alignEnd")
	Spanning   = component[NoProps]("spanning")
	SplitPane  = component[SplitPaneProps]("splitPane")

	ThinScrollArea = component[ScrollProps]("thinScroll")
)

// EmptyStateProps is the props of EmptyState and InlineEmptyState, which
// differ in the defaults of every field left at zero. The children are the
// actions. Gaps and Pad are dp, sizes sp.
type EmptyStateProps struct {
	Icon            Icon       `json:"icon,omitempty"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	IconSize        float64    `json:"iconSize,omitempty"`
	IconGap         float64    `json:"iconGap,omitempty"`
	DescriptionGap  float64    `json:"descriptionGap,omitempty"`
	ActionsGap      float64    `json:"actionsGap,omitempty"`
	TitleSize       float64    `json:"titleSize,omitempty"`
	DescriptionSize float64    `json:"descriptionSize,omitempty"`
	TitleLeading    float64    `json:"titleLeading,omitempty"`
	TitleWeight     FontWeight `json:"titleWeight,omitempty"`
	MaxLines        int        `json:"maxLines,omitempty"`
	Anchor          Anchor     `json:"anchor,omitempty"`
	Align           Align      `json:"align,omitempty"`
	Pad             float64    `json:"pad,omitempty"`
}

// AvatarProps draws Image, PNG or JPEG bytes, in a circle, or Initial on
// Background when there is none.
type AvatarProps struct {
	Image      []byte  `json:"image,omitempty"`
	Initial    string  `json:"initial,omitempty"`
	Size       float64 `json:"size"`
	Background string  `json:"background,omitempty"`
	Foreground string  `json:"foreground,omitempty"`
}

type SectionHeadingProps struct {
	Text      string     `json:"text"`
	Size      float64    `json:"size,omitempty"`
	Weight    FontWeight `json:"weight,omitempty"`
	Leading   float64    `json:"leading,omitempty"`
	PadTop    float64    `json:"padTop,omitempty"`
	PadRight  float64    `json:"padRight,omitempty"`
	PadBottom float64    `json:"padBottom,omitempty"`
	PadLeft   float64    `json:"padLeft,omitempty"`
}

type MarkedSegment struct {
	Text   string `json:"text"`
	Marked bool   `json:"marked,omitempty"`
}

type MarkedTextProps struct {
	Segments []MarkedSegment `json:"segments"`
}

// ImageProps draws Data, PNG or JPEG bytes. A Width or Height of 0 leaves
// that side to the room the parent gives.
type ImageProps struct {
	Data     []byte   `json:"data"`
	Fit      ImageFit `json:"fit,omitempty"`
	Position Anchor   `json:"position,omitempty"`
	Width    float64  `json:"width,omitempty"`
	Height   float64  `json:"height,omitempty"`
}

type FileIconProps struct {
	Path string  `json:"path"`
	Size float64 `json:"size,omitempty"`
}

type SkeletonBarProps struct {
	Width  float64 `json:"width,omitempty"`
	Line   float64 `json:"line,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type SkeletonTextProps struct {
	Size  float64 `json:"size,omitempty"`
	Width float64 `json:"width,omitempty"`
}

// SkeletonFillProps repeats its children, in turn, as rows down to the
// bottom of its parent or MaxRows of them.
type SkeletonFillProps struct {
	MaxRows int     `json:"maxRows,omitempty"`
	PadY    float64 `json:"padY,omitempty"`
}

type SkeletonRowProps struct {
	PadX float64 `json:"padX,omitempty"`
	PadY float64 `json:"padY,omitempty"`
}

// GapProps is the props of Stack (children one under another), Row (side by
// side, centered) and Columns (side by side, sharing the width evenly).
type GapProps struct {
	Gap float64 `json:"gap,omitempty"`
}

type FixedColumnProps struct {
	Width float64 `json:"width"`
}

// SplitPaneProps draws its two children side by side (row) or one over the
// other (column), with a divider Gap wide between them. Ratio is the share
// the first takes, held between MinRatio and MaxRatio (0 means 1) and
// MinFirst and MinSecond. OnResize carries the new ratio in EventData.Width
// when a drag ends.
type SplitPaneProps struct {
	Direction Direction `json:"direction,omitempty"`
	Ratio     float64   `json:"ratio"`
	MinRatio  float64   `json:"minRatio,omitempty"`
	MaxRatio  float64   `json:"maxRatio,omitempty"`
	Gap       float64   `json:"gap,omitempty"`
	MinFirst  float64   `json:"minFirst,omitempty"`
	MinSecond float64   `json:"minSecond,omitempty"`
	OnResize  Handler   `json:"onResize,omitempty"`
}
