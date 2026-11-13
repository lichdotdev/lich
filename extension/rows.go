package extension

var (
	IconLine         = component[IconColorProps]("iconLine")
	BoxIcon          = component[IconColorProps]("boxIcon")
	IconLabel        = component[IconLabelProps]("iconLabel")
	MonoGlyph        = component[MetaGlyphProps]("monoGlyph")
	MutedGlyph       = component[MutedGlyphProps]("mutedGlyph")
	CenteredIcon     = component[GlyphProps]("centeredIcon")
	TriggerChevron   = component[TriggerChevronProps]("triggerChevron")
	TinyChip         = component[TinyChipProps]("tinyChip")
	HoverLink        = component[LinkProps]("hoverLink")
	TrailingIconLink = component[TrailingIconLinkProps]("trailingIconLink")
	TwoLines         = component[TwoLinesProps]("twoLines")
	SpinnerLine      = component[TextProps]("spinnerLine")
	SpinnerBeside    = component[NoProps]("spinnerBeside")
	Stat             = component[StatProps]("stat")

	CheckRow   = component[CheckRowProps]("checkRow")
	NavRow     = component[NavRowProps]("navRow")
	PanelRow   = component[PanelRowProps]("panelRow")
	FillRow    = component[FillRowProps]("fillRow")
	OrRule     = component[WordProps]("orRule")
	MenuHeader = component[MenuHeaderProps]("menuHeader")

	// MenuDivider is the "menuSeparator" component, named apart from the
	// MenuSeparator entry kind.
	MenuDivider    = component[NoProps]("menuSeparator")
	RailHairline   = component[NoProps]("railHairline")
	CaptionDivider = component[CaptionDividerProps]("captionDivider")

	GhostTextButton = component[GhostTextButtonProps]("ghostTextButton")
	CardIconButton  = component[CardIconButtonProps]("cardIconButton")
	GhostIconButton = component[GhostIconButtonProps]("ghostIconButton")
	SquareFace      = component[FaceProps]("squareFace")
	LabelFace       = component[LabelFaceProps]("labelFace")
)

// IconColorProps is the props of IconLine, which draws its children after
// the icon, and of BoxIcon, which sizes the icon to its parent.
type IconColorProps struct {
	Icon  Icon   `json:"icon"`
	Color string `json:"color,omitempty"`
}

// IconLabelProps puts Label beside a 12dp icon painted in Glyph.
type IconLabelProps struct {
	Icon  Icon       `json:"icon"`
	Glyph string     `json:"glyph,omitempty"`
	Label LabelProps `json:"label"`
}

// StatProps is the props of Stat, which draws its first child, a glyph the
// extension sizes (an icon, or a busyRing for work in flight), and Text after
// it.
type StatProps struct {
	Text  string `json:"text"`
	Color string `json:"color,omitempty"`
}

type MutedGlyphProps struct {
	Icon Icon    `json:"icon"`
	Size float64 `json:"size,omitempty"`
}

type TriggerChevronProps struct {
	Open bool `json:"open,omitempty"`
}

type TinyChipProps struct {
	Word string `json:"word"`
	Tone string `json:"tone,omitempty"`
}

type LinkProps struct {
	Text    string  `json:"text"`
	OnPress Handler `json:"onPress,omitempty"`
}

type TrailingIconLinkProps struct {
	Label   string  `json:"label"`
	Icon    Icon    `json:"icon"`
	OnPress Handler `json:"onPress,omitempty"`
}

type TwoLinesProps struct {
	Top   string `json:"top"`
	Under string `json:"under,omitempty"`
}

type CheckRowProps struct {
	Checked  bool    `json:"checked,omitempty"`
	Label    string  `json:"label"`
	Note     string  `json:"note,omitempty"`
	OnChange Handler `json:"onChange,omitempty"`
}

// NavRowProps draws its children as one selectable row, On when it is the
// page the person is on.
type NavRowProps struct {
	On      bool    `json:"on,omitempty"`
	Height  float64 `json:"height,omitempty"`
	PadX    float64 `json:"padX,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

// PanelRowProps draws its children as one row inset Edge from the panel's
// sides. Only a Live row lights under the pointer, across the panel's whole
// width.
type PanelRowProps struct {
	Live    bool    `json:"live,omitempty"`
	Edge    float64 `json:"edge,omitempty"`
	PadX    float64 `json:"padX,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

// FillRowProps paints the accent at Alpha, from 0 to 1, behind a row of the
// given size.
type FillRowProps struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Alpha  float64 `json:"alpha"`
}

type WordProps struct {
	Word string `json:"word"`
}

type MenuHeaderProps struct {
	Title string `json:"title"`
	Count string `json:"count,omitempty"`
}

type CaptionDividerProps struct {
	Inset  float64 `json:"inset,omitempty"`
	Height float64 `json:"height"`
}

type GhostTextButtonProps struct {
	Label   string  `json:"label"`
	Color   string  `json:"color,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

// CardIconButtonProps is a bare icon button, Turn radians off upright; with
// no OnPress it draws disabled.
type CardIconButtonProps struct {
	Icon    Icon    `json:"icon"`
	Size    float64 `json:"size,omitempty"`
	Turn    float64 `json:"turn,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

// GhostIconButtonProps is an icon button that names itself in a tooltip.
type GhostIconButtonProps struct {
	Icon     Icon    `json:"icon"`
	Hint     string  `json:"hint,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
	OnPress  Handler `json:"onPress,omitempty"`
}

// FaceProps is the props of SquareFace, an 8-unit tile whose child is the
// icon centered on Background.
type FaceProps struct {
	Background string `json:"background,omitempty"`
}

// LabelFaceProps is a tile with its first child, the icon, then Label.
type LabelFaceProps struct {
	Background string `json:"background,omitempty"`
	Label      string `json:"label"`
}
