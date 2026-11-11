package extension

var (
	Tooltip       = component[TooltipProps]("tooltip")
	DropdownMenu  = component[DropdownMenuProps]("dropdownMenu")
	ContextMenu   = component[ContextMenuProps]("contextMenu")
	DropdownPanel = component[DropdownPanelProps]("dropdownPanel")
	Dialog        = component[DialogProps]("dialog")
)

// TooltipProps wraps its child. Content replaces Text for a card tooltip.
type TooltipProps struct {
	Text    string         `json:"text,omitempty"`
	Content *Node          `json:"content,omitempty"`
	Variant TooltipVariant `json:"variant,omitempty"`
	Side    Side           `json:"side,omitempty"`
}

type MenuEntry struct {
	Kind                MenuEntryKind `json:"kind,omitempty"`
	Label               string        `json:"label,omitempty"`
	Icon                Icon          `json:"icon,omitempty"`
	Shortcut            string        `json:"shortcut,omitempty"`
	ShortcutDestructive bool          `json:"shortcutDestructive,omitempty"`
	Lead                *Node         `json:"lead,omitempty"`
	Variant             Variant       `json:"variant,omitempty"`
	Inset               bool          `json:"inset,omitempty"`
	Disabled            bool          `json:"disabled,omitempty"`
	Checked             bool          `json:"checked,omitempty"`
	Sub                 []MenuEntry   `json:"sub,omitempty"`
	Columns             int           `json:"columns,omitempty"`
	Swatch              string        `json:"swatch,omitempty"`
	OnSelect            Handler       `json:"onSelect,omitempty"`
	OnCheckedChange     Handler       `json:"onCheckedChange,omitempty"`
}

// DropdownMenuProps opens Entries from its child.
type DropdownMenuProps struct {
	Entries []MenuEntry `json:"entries"`
}

// ContextMenuProps opens Entries on a secondary click anywhere on its child.
type ContextMenuProps struct {
	Entries []MenuEntry `json:"entries"`
}

// DropdownPanelProps opens its children under Trigger.
type DropdownPanelProps struct {
	Trigger  Node    `json:"trigger"`
	Width    float64 `json:"width,omitempty"`
	AlignEnd bool    `json:"alignEnd,omitempty"`
}

// DialogProps draws its children as the body, or as the whole panel with Raw.
type DialogProps struct {
	Open        bool    `json:"open,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Actions     []Node  `json:"actions,omitempty"`
	MaxWidth    float64 `json:"maxWidth,omitempty"`
	HideClose   bool    `json:"hideClose,omitempty"`
	Raw         bool    `json:"raw,omitempty"`
	Top         float64 `json:"top,omitempty"`
	Scrim       float64 `json:"scrim,omitempty"`
	OnClose     Handler `json:"onClose,omitempty"`
}
