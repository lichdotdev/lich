package extrender

import (
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// color is a token's color in th, or a hex value; anything else is the
// foreground, so a typo stays readable.
func (r *Renderer) color(th *ui.Theme, s string, fallback color.NRGBA) color.NRGBA {
	if s == "" {
		return fallback
	}
	if c, ok := tokens(th)[s]; ok {
		return c
	}
	if strings.HasPrefix(s, "#") {
		if c, err := ui.ParseColor(s); err == nil {
			return c
		}
	}
	return th.Foreground
}

func tokens(th *ui.Theme) map[string]color.NRGBA {
	return map[string]color.NRGBA{
		"background": th.Background, "foreground": th.Foreground, "canvas": th.Canvas,
		"card": th.Card, "card-foreground": th.CardForeground,
		"popover": th.Popover, "popover-foreground": th.PopoverForeground,
		"primary": th.Primary, "primary-foreground": th.PrimaryForeground,
		"secondary": th.Secondary, "secondary-foreground": th.SecondaryForeground,
		"muted": th.Muted, "muted-foreground": th.MutedForeground,
		"accent": th.Accent, "accent-foreground": th.AccentForeground,
		"destructive": th.Destructive, "tone-pass": th.TonePass, "tone-wait": th.ToneWait,
		"border": th.Border, "input": th.Palette.Input, "ring": th.Palette.Ring,
		"sidebar": th.Sidebar, "sidebar-foreground": th.SidebarForeground,
		"sidebar-accent": th.SidebarAccent, "sidebar-accent-foreground": th.SidebarAccentForeground,
		"sidebar-border": th.SidebarBorder,
	}
}

// icon is the glyph an extension names, nil for a name no set has: an icon
// lookup panics on an unknown name, and this name comes from outside.
func (r *Renderer) icon(name extension.Icon) *icons.Icon {
	if name == "" {
		return nil
	}
	ic, _ := icons.LookupLucide(string(name))
	return ic
}

// textSize is size in sp, or def when the extension left it out.
func textSize(size float64, def unit.Sp) unit.Sp {
	if size <= 0 {
		return def
	}
	return unit.Sp(size)
}

func weight(w extension.FontWeight) font.Weight {
	switch w {
	case extension.FontMedium:
		return font.Medium
	case extension.FontSemibold:
		return font.SemiBold
	}
	return font.Normal
}

func variant(v extension.Variant) ui.Variant {
	switch v {
	case extension.VariantOutline:
		return ui.VariantOutline
	case extension.VariantSecondary:
		return ui.VariantSecondary
	case extension.VariantGhost:
		return ui.VariantGhost
	case extension.VariantDestructive:
		return ui.VariantDestructive
	}
	return ui.VariantDefault
}

func size(s extension.Size) ui.Size {
	switch s {
	case extension.SizeXS:
		return ui.SizeXS
	case extension.SizeSM:
		return ui.SizeSM
	case extension.SizeLG:
		return ui.SizeLG
	case extension.SizeIcon:
		return ui.SizeIcon
	case extension.SizeIconXS:
		return ui.SizeIconXS
	case extension.SizeIconSM:
		return ui.SizeIconSM
	case extension.SizeIconLG:
		return ui.SizeIconLG
	}
	return ui.SizeDefault
}
