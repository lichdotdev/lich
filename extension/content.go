package extension

var (
	Notice      = component[TextProps]("notice")
	Footnote    = component[TextProps]("footnote")
	BodyLine    = component[TextProps]("bodyLine")
	PanelNotice = component[PanelNoticeProps]("panelNotice")

	IconAction    = component[IconActionProps]("iconAction")
	CornerButton  = component[IconToggleProps]("cornerButton")
	BarIconButton = component[IconToggleProps]("barIconButton")

	DiffStat      = component[DiffStatProps]("diffStat")
	MetaGlyph     = component[MetaGlyphProps]("metaGlyph")
	Pill          = component[PillProps]("pill")
	KeyCap        = component[KeyCapProps]("keyCap")
	PopoverRow    = component[PopoverRowProps]("popoverRow")
	SelectTrigger = component[SelectTriggerProps]("selectTrigger")
	VerticalRule  = component[NoProps]("verticalRule")
)

type TextProps struct {
	Text string `json:"text"`
}

type PanelNoticeProps struct {
	Text string  `json:"text"`
	PadX float64 `json:"padX,omitempty"`
}

type IconActionProps struct {
	Icon    Icon    `json:"icon"`
	Label   string  `json:"label"`
	Pressed bool    `json:"pressed,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

// IconToggleProps is a bare icon button; with no OnPress it draws disabled.
type IconToggleProps struct {
	Icon    Icon    `json:"icon"`
	On      bool    `json:"on,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

type DiffStatProps struct {
	Added   int `json:"added,omitempty"`
	Deleted int `json:"deleted,omitempty"`
}

type MetaGlyphProps struct {
	Icon  Icon   `json:"icon"`
	Text  string `json:"text"`
	Color string `json:"color,omitempty"`
}

// PillProps draws its children inside the pill.
type PillProps struct {
	PadLeft  float64 `json:"padLeft,omitempty"`
	PadRight float64 `json:"padRight,omitempty"`
}

type KeyCapProps struct {
	Keys string `json:"keys"`
}

// PopoverRowProps draws its children as one clickable row.
type PopoverRowProps struct {
	OnPress Handler `json:"onPress,omitempty"`
}

// SelectTriggerProps draws the closed face of a select. Put it inside a
// DropdownMenu to open choices from it.
type SelectTriggerProps struct {
	Value string  `json:"value"`
	Width float64 `json:"width,omitempty"`
	Lead  *Node   `json:"lead,omitempty"`
}
