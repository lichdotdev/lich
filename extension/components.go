package extension

// NoProps is the props of a component that takes none.
type NoProps struct{}

var (
	Box       = component[BoxProps]("box")
	Scroll    = component[ScrollProps]("scroll")
	Wrap      = component[WrapProps]("wrap")
	WrapItem  = component[WrapItemProps]("wrapItem")
	Separator = component[NoProps]("separator")

	Label     = component[LabelProps]("label")
	FormLabel = component[FormLabelProps]("formLabel")
	Glyph     = component[GlyphProps]("icon")
	Skeleton  = component[SkeletonProps]("skeleton")
	Progress  = component[ProgressProps]("progress")
	Spinner   = component[SpinnerProps]("spinner")
	BusyRing  = component[BusyRingProps]("busyRing")

	Button      = component[ButtonProps]("button")
	Chip        = component[ChipProps]("chip")
	Checkbox    = component[CheckboxProps]("checkbox")
	Switch      = component[SwitchProps]("switch")
	Toggle      = component[ToggleProps]("toggle")
	ToggleGroup = component[ToggleGroupProps]("toggleGroup")
	Tabs        = component[TabsProps]("tabs")

	Input      = component[InputProps]("input")
	InputGroup = component[InputGroupProps]("inputGroup")
	Textarea   = component[TextareaProps]("textarea")
	InlineEdit = component[InlineEditProps]("inlineEdit")

	Collapse     = component[CollapseProps]("collapse")
	Presence     = component[NoProps]("presence")
	SortableList = component[SortableListProps]("sortableList")
	ResizeHandle = component[ResizeHandleProps]("resizeHandle")
)

type BoxProps struct {
	Direction  Direction `json:"direction,omitempty"`
	Gap        float64   `json:"gap,omitempty"`
	Padding    float64   `json:"padding,omitempty"`
	PaddingX   float64   `json:"paddingX,omitempty"`
	PaddingY   float64   `json:"paddingY,omitempty"`
	Align      Align     `json:"align,omitempty"`
	Justify    Justify   `json:"justify,omitempty"`
	Grow       float64   `json:"grow,omitempty"`
	Width      float64   `json:"width,omitempty"`
	Height     float64   `json:"height,omitempty"`
	Background string    `json:"background,omitempty"`
	Radius     float64   `json:"radius,omitempty"`
}

type ScrollProps struct {
	Direction Direction `json:"direction,omitempty"`
}

type WrapProps struct {
	GapX         float64 `json:"gapX,omitempty"`
	GapY         float64 `json:"gapY,omitempty"`
	Justify      Justify `json:"justify,omitempty"`
	ContentStart bool    `json:"contentStart,omitempty"`
}

type WrapItemProps struct {
	MarginAuto bool `json:"marginAuto,omitempty"`
}

type LabelProps struct {
	Text       string     `json:"text"`
	Size       float64    `json:"size,omitempty"`
	Color      string     `json:"color,omitempty"`
	Weight     FontWeight `json:"weight,omitempty"`
	Mono       bool       `json:"mono,omitempty"`
	MaxLines   int        `json:"maxLines,omitempty"`
	LineHeight float64    `json:"lineHeight,omitempty"`
	Align      TextAlign  `json:"align,omitempty"`
}

type FormLabelProps struct {
	Text      string     `json:"text"`
	Size      float64    `json:"size,omitempty"`
	Weight    FontWeight `json:"weight,omitempty"`
	Uppercase bool       `json:"uppercase,omitempty"`
	Disabled  bool       `json:"disabled,omitempty"`
}

type GlyphProps struct {
	Name  Icon    `json:"name"`
	Size  float64 `json:"size,omitempty"`
	Color string  `json:"color,omitempty"`
}

type SkeletonProps struct {
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type ProgressProps struct {
	Percent int `json:"percent"`
}

type SpinnerProps struct {
	Size  float64 `json:"size,omitempty"`
	Color string  `json:"color,omitempty"`
}

type BusyRingProps struct {
	Size  float64 `json:"size,omitempty"`
	Color string  `json:"color,omitempty"`
	Track string  `json:"track,omitempty"`
}

type ButtonProps struct {
	Text     string  `json:"text,omitempty"`
	Icon     Icon    `json:"icon,omitempty"`
	Variant  Variant `json:"variant,omitempty"`
	Size     Size    `json:"size,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
	Lit      bool    `json:"lit,omitempty"`
	OnPress  Handler `json:"onPress,omitempty"`
}

type ChipProps struct {
	Label   string  `json:"label"`
	Count   int     `json:"count,omitempty"`
	On      bool    `json:"on,omitempty"`
	OnPress Handler `json:"onPress,omitempty"`
}

type CheckboxProps struct {
	Checked  bool    `json:"checked,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
	OnChange Handler `json:"onChange,omitempty"`
}

type SwitchProps struct {
	Checked  bool       `json:"checked,omitempty"`
	Size     SwitchSize `json:"size,omitempty"`
	Disabled bool       `json:"disabled,omitempty"`
	OnChange Handler    `json:"onChange,omitempty"`
}

type ToggleProps struct {
	Pressed  bool          `json:"pressed,omitempty"`
	Text     string        `json:"text,omitempty"`
	Icon     Icon          `json:"icon,omitempty"`
	Variant  ToggleVariant `json:"variant,omitempty"`
	Size     ToggleSize    `json:"size,omitempty"`
	Height   float64       `json:"height,omitempty"`
	PadX     float64       `json:"padX,omitempty"`
	TextSize float64       `json:"textSize,omitempty"`
	Disabled bool          `json:"disabled,omitempty"`
	OnChange Handler       `json:"onChange,omitempty"`
}

type ToggleItem struct {
	Value    string `json:"value"`
	Text     string `json:"text,omitempty"`
	Icon     Icon   `json:"icon,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type ToggleGroupProps struct {
	Value      string        `json:"value,omitempty"`
	Items      []ToggleItem  `json:"items"`
	Variant    ToggleVariant `json:"variant,omitempty"`
	Size       ToggleSize    `json:"size,omitempty"`
	ItemHeight float64       `json:"itemHeight,omitempty"`
	ItemPadX   float64       `json:"itemPadX,omitempty"`
	ItemText   float64       `json:"itemText,omitempty"`
	OnChange   Handler       `json:"onChange,omitempty"`
}

type TabsTrigger struct {
	Text     string `json:"text,omitempty"`
	Icon     Icon   `json:"icon,omitempty"`
	Trailing *Node  `json:"trailing,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type TabsProps struct {
	Selected    int             `json:"selected,omitempty"`
	Triggers    []TabsTrigger   `json:"triggers"`
	Variant     TabsVariant     `json:"variant,omitempty"`
	Orientation TabsOrientation `json:"orientation,omitempty"`
	OnChange    Handler         `json:"onChange,omitempty"`
}

type InputProps struct {
	Value       string  `json:"value,omitempty"`
	Placeholder string  `json:"placeholder,omitempty"`
	Leading     Icon    `json:"leading,omitempty"`
	LeadingSize float64 `json:"leadingSize,omitempty"`
	Height      float64 `json:"height,omitempty"`
	Width       float64 `json:"width,omitempty"`
	TextSize    float64 `json:"textSize,omitempty"`
	Mono        bool    `json:"mono,omitempty"`
	Password    bool    `json:"password,omitempty"`
	Borderless  bool    `json:"borderless,omitempty"`
	Bare        bool    `json:"bare,omitempty"`
	Clearable   bool    `json:"clearable,omitempty"`
	Disabled    bool    `json:"disabled,omitempty"`
	Invalid     bool    `json:"invalid,omitempty"`
	OnChange    Handler `json:"onChange,omitempty"`
	OnSubmit    Handler `json:"onSubmit,omitempty"`
}

type InputGroupProps struct {
	Value       string  `json:"value,omitempty"`
	Placeholder string  `json:"placeholder,omitempty"`
	Multiline   bool    `json:"multiline,omitempty"`
	Start       []Node  `json:"start,omitempty"`
	End         []Node  `json:"end,omitempty"`
	Width       float64 `json:"width,omitempty"`
	Disabled    bool    `json:"disabled,omitempty"`
	Invalid     bool    `json:"invalid,omitempty"`
	OnChange    Handler `json:"onChange,omitempty"`
	OnSubmit    Handler `json:"onSubmit,omitempty"`
}

type TextareaProps struct {
	Value       string  `json:"value,omitempty"`
	Placeholder string  `json:"placeholder,omitempty"`
	Resizable   bool    `json:"resizable,omitempty"`
	MaxHeight   float64 `json:"maxHeight,omitempty"`
	Width       float64 `json:"width,omitempty"`
	Disabled    bool    `json:"disabled,omitempty"`
	Invalid     bool    `json:"invalid,omitempty"`
	OnChange    Handler `json:"onChange,omitempty"`
}

type InlineEditProps struct {
	Value    string  `json:"value"`
	Size     float64 `json:"size,omitempty"`
	Mono     bool    `json:"mono,omitempty"`
	PadRight float64 `json:"padRight,omitempty"`
	OnCommit Handler `json:"onCommit,omitempty"`
	OnCancel Handler `json:"onCancel,omitempty"`
}

type CollapseProps struct {
	Open bool `json:"open,omitempty"`
}

// SortableListProps reorders its keyed children; OnReorder carries the keys
// in their new order. Lists on one surface with the same non-empty Group
// exchange items: an item dropped into another list fires that list's OnMove
// with the item's key, the index it landed at and the source list's key, and
// the source fires no OnReorder. Keys are unique across a group.
type SortableListProps struct {
	Gap        float64 `json:"gap,omitempty"`
	Horizontal bool    `json:"horizontal,omitempty"`
	Within     bool    `json:"within,omitempty"`
	Leaves     bool    `json:"leaves,omitempty"`
	Group      string  `json:"group,omitempty"`
	OnReorder  Handler `json:"onReorder,omitempty"`
	OnMove     Handler `json:"onMove,omitempty"`
}

type ResizeHandleProps struct {
	Edge     PanelEdge `json:"edge"`
	Width    float64   `json:"width"`
	MinWidth float64   `json:"minWidth,omitempty"`
	MaxWidth float64   `json:"maxWidth,omitempty"`
	OnResize Handler   `json:"onResize,omitempty"`
}
