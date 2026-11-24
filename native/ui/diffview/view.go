// Package diffview draws a diff unified or side by side: a godemirror
// DiffView with lich's buttons, its widgets under lines and its height fixed
// by the caller, so a scrolling parent scrolls and the editor never does.
package diffview

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// Options are what a View acts with: Buttons is required, Expand and Revert
// are nil where the diff offers neither. Gutter and RevertAt are godemirror's,
// their zero values its defaults.
type Options struct {
	Buttons  *Buttons
	Expand   func(godemirror.Gap)
	Revert   func(godemirror.Chunk)
	Gutter   godemirror.UnifiedGutter
	RevertAt godemirror.RevertPlacement
}

// View is one diff. The embedded editor answers selection and fills a gap's
// lines.
type View struct {
	*godemirror.DiffView
	buttons *Buttons
}

// New is a View in family at size.
func New(family string, size unit.Sp, opts Options) (*View, error) {
	revert := opts.Buttons.Revert
	if opts.RevertAt == godemirror.RevertInGutter {
		revert = opts.Buttons.RevertIcon
	}
	view, err := codeview.NewDiffEditor(family, size, godemirror.DiffOptions{
		HighlightChanges: true,
		StyleTags:        codeview.StyleTags,
		Expand:           opts.Expand,
		Revert:           opts.Revert,
		ExpandWidget:     opts.Buttons.Expand,
		RevertWidget:     revert,
		UnifiedGutter:    opts.Gutter,
		RevertPlacement:  opts.RevertAt,
	})
	if err != nil {
		return nil, err
	}
	return &View{DiffView: view, buttons: opts.Buttons}, nil
}

// Frame is what a View draws in one frame.
type Frame struct {
	Theme *ui.Theme
	Code  godemirror.Theme
	Patch godemirror.FilePatch
	Split bool
	// Wrap breaks long lines; Height then comes from View.Height.
	Wrap bool
	// Height is the diff's rows in pixels; Blocks' room is added to it.
	Height int
	Blocks *Blocks
	Items  []Block
}

// Layout draws f and reports whether the room its blocks take changed, which
// the next frame has to be drawn at.
func (v *View) Layout(gtx C, f Frame) (D, bool, error) {
	v.set(f)
	f.Blocks.place(v.DiffView, f.Items)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, f.Height+f.Blocks.height()))
	f.Blocks.startFrame()
	dims, err := v.DiffView.Layout(gtx)
	return dims, f.Blocks.endFrame(), err
}

// Height is the room f's rows take at width px, blocks left out: what
// Frame.Height holds for a wrapped diff. It measures every row, so the caller
// keeps it until the diff, the width or the layout changes.
func (v *View) Height(gtx C, f Frame, width int) (int, error) {
	v.set(f)
	return v.DiffView.Height(gtx, width)
}

func (v *View) set(f Frame) {
	v.buttons.theme = f.Theme
	v.SetTheme(codeview.OnSurface(f.Theme, f.Code))
	v.Split, v.Wrap, v.Nested = f.Split, f.Wrap, true
	v.SetPatch(f.Patch)
}
