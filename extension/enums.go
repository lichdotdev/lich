package extension

type (
	Variant         string
	Size            string
	ToggleVariant   string
	ToggleSize      string
	SwitchSize      string
	TabsVariant     string
	TabsOrientation string
	TooltipVariant  string
	Side            string
	Justify         string
	Align           string
	Direction       string
	PanelEdge       string
	TextAlign       string
	FontWeight      string
	MenuEntryKind   string
	ToastVariant    string
)

const (
	VariantDefault     Variant = "default"
	VariantOutline     Variant = "outline"
	VariantSecondary   Variant = "secondary"
	VariantGhost       Variant = "ghost"
	VariantDestructive Variant = "destructive"

	SizeDefault Size = "default"
	SizeXS      Size = "xs"
	SizeSM      Size = "sm"
	SizeLG      Size = "lg"
	SizeIcon    Size = "icon"
	SizeIconXS  Size = "icon-xs"
	SizeIconSM  Size = "icon-sm"
	SizeIconLG  Size = "icon-lg"

	ToggleVariantDefault ToggleVariant = "default"
	ToggleVariantOutline ToggleVariant = "outline"

	ToggleSizeDefault ToggleSize = "default"
	ToggleSizeSM      ToggleSize = "sm"
	ToggleSizeLG      ToggleSize = "lg"

	SwitchSizeDefault SwitchSize = "default"
	SwitchSizeSM      SwitchSize = "sm"

	TabsVariantDefault TabsVariant = "default"
	TabsVariantLine    TabsVariant = "line"
	TabsVariantGhost   TabsVariant = "ghost"

	TabsHorizontal TabsOrientation = "horizontal"
	TabsVertical   TabsOrientation = "vertical"

	TooltipLabel TooltipVariant = "label"
	TooltipCard  TooltipVariant = "card"

	SideTop    Side = "top"
	SideBottom Side = "bottom"
	SideLeft   Side = "left"
	SideRight  Side = "right"

	JustifyStart   Justify = "start"
	JustifyCenter  Justify = "center"
	JustifyEnd     Justify = "end"
	JustifyBetween Justify = "between"

	AlignStart   Align = "start"
	AlignCenter  Align = "center"
	AlignEnd     Align = "end"
	AlignStretch Align = "stretch"

	DirectionColumn Direction = "column"
	DirectionRow    Direction = "row"

	PanelEdgeLeft  PanelEdge = "left"
	PanelEdgeRight PanelEdge = "right"

	TextAlignStart  TextAlign = "start"
	TextAlignCenter TextAlign = "center"
	TextAlignEnd    TextAlign = "end"

	FontRegular  FontWeight = "regular"
	FontMedium   FontWeight = "medium"
	FontSemibold FontWeight = "semibold"

	MenuItem         MenuEntryKind = "item"
	MenuCheckboxItem MenuEntryKind = "checkbox"
	MenuRadioItem    MenuEntryKind = "radio"
	MenuSubTrigger   MenuEntryKind = "sub"
	MenuLabel        MenuEntryKind = "label"
	MenuSeparator    MenuEntryKind = "separator"
	MenuSwatch       MenuEntryKind = "swatch"

	ToastDefault ToastVariant = "default"
	ToastSuccess ToastVariant = "success"
	ToastInfo    ToastVariant = "info"
	ToastWarning ToastVariant = "warning"
	ToastError   ToastVariant = "error"
)

// EnumValues is every value each enum accepts on the wire, by enum name.
func EnumValues() map[string][]string {
	return map[string][]string{
		"variant":         values(VariantDefault, VariantOutline, VariantSecondary, VariantGhost, VariantDestructive),
		"size":            values(SizeDefault, SizeXS, SizeSM, SizeLG, SizeIcon, SizeIconXS, SizeIconSM, SizeIconLG),
		"toggleVariant":   values(ToggleVariantDefault, ToggleVariantOutline),
		"toggleSize":      values(ToggleSizeDefault, ToggleSizeSM, ToggleSizeLG),
		"switchSize":      values(SwitchSizeDefault, SwitchSizeSM),
		"tabsVariant":     values(TabsVariantDefault, TabsVariantLine, TabsVariantGhost),
		"tabsOrientation": values(TabsHorizontal, TabsVertical),
		"tooltipVariant":  values(TooltipLabel, TooltipCard),
		"side":            values(SideTop, SideBottom, SideLeft, SideRight),
		"justify":         values(JustifyStart, JustifyCenter, JustifyEnd, JustifyBetween),
		"align":           values(AlignStart, AlignCenter, AlignEnd, AlignStretch),
		"direction":       values(DirectionColumn, DirectionRow),
		"panelEdge":       values(PanelEdgeLeft, PanelEdgeRight),
		"textAlign":       values(TextAlignStart, TextAlignCenter, TextAlignEnd),
		"fontWeight":      values(FontRegular, FontMedium, FontSemibold),
		"menuEntryKind":   values(MenuItem, MenuCheckboxItem, MenuRadioItem, MenuSubTrigger, MenuLabel, MenuSeparator, MenuSwatch),
		"toastVariant":    values(ToastDefault, ToastSuccess, ToastInfo, ToastWarning, ToastError),
	}
}

func values[T ~string](all ...T) []string {
	out := make([]string, len(all))
	for i, v := range all {
		out[i] = string(v)
	}
	return out
}
