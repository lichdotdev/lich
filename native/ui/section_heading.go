package ui

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

// SectionHeadingStyle is a group's small, muted, uppercase label.
type SectionHeadingStyle struct {
	Text   string
	Size   unit.Sp
	Weight font.Weight
	// Leading is 0 to keep the size's Tailwind line height.
	Leading unit.Sp
	Inset   layout.Inset
	theme   *Theme
}

func (th *Theme) SectionHeading(text string) SectionHeadingStyle {
	return SectionHeadingStyle{Text: text, Size: TextXS, Weight: font.Medium, theme: th}
}

func (h SectionHeadingStyle) Layout(gtx C) D {
	l := h.theme.Text(h.Size, strings.ToUpper(h.Text)).Muted().Weight(h.Weight)
	if h.Leading != 0 {
		l = l.Leading(h.Leading)
	}
	return h.Inset.Layout(gtx, l.Layout)
}
