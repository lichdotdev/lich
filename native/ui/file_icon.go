package ui

import (
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

const fileIconOpacity = 0.7

var icFileGlyph = icons.Lucide("file")

// FileIconStyle is a file's Material icon by its path, in the theme's scheme,
// or the muted file glyph at 70% opacity for a file it has none for.
type FileIconStyle struct {
	Path  string
	Size  unit.Dp
	theme *Theme
}

func (th *Theme) FileIcon(path string, size unit.Dp) FileIconStyle {
	return FileIconStyle{Path: path, Size: size, theme: th}
}

func (f FileIconStyle) Layout(gtx C) D {
	th := f.theme
	if ic := th.FileIcons.For(f.Path); ic != nil {
		return ic.Layout(gtx, f.Size, th.Foreground)
	}
	return icFileGlyph.Layout(gtx, f.Size, th.Over(Alpha(th.MutedForeground, fileIconOpacity)))
}
