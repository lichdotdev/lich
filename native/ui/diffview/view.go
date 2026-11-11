// Package diffview draws a diff unified or side by side: a godemirror
// DiffView with lich's buttons, its widgets under lines and its height fixed
// by the caller's row count, so a scrolling parent scrolls and the editor
// never does.
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
// are nil where the diff offers neither.
type Options struct {
	Buttons *Buttons
	Expand  func(godemirror.Gap)
	Revert  func(godemirror.Chunk)
}

// View is one diff. The embedded editor answers selection and fills a gap's
// lines.
type View struct {
	*godemirror.DiffView
	buttons *Buttons
}

// New is a View in family at size.
func New(family string, size unit.Sp, opts Options) (*View, error) {
	view, err := codeview.NewDiffEditor(family, size, godemirror.DiffOptions{
		HighlightChanges: true,
		StyleTags:        codeview.StyleTags,
		Expand:           opts.Expand,
		Revert:           opts.Revert,
		ExpandWidget:     opts.Buttons.Expand,
		RevertWidget:     opts.Buttons.Revert,
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
	// Height is the diff's rows in pixels; Blocks' room is added to it.
	Height int
	Blocks *Blocks
	Items  []Block
}

// Layout draws f and reports whether the room its blocks take changed, which
// the next frame has to be drawn at.
func (v *View) Layout(gtx C, f Frame) (D, bool, error) {
	v.buttons.theme = f.Theme
	v.SetTheme(codeview.OnSurface(f.Theme, f.Code))
	v.Split, v.Nested = f.Split, true
	v.SetPatch(f.Patch)
	f.Blocks.place(v.DiffView, f.Items)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, f.Height+f.Blocks.height()))
	f.Blocks.startFrame()
	dims, err := v.DiffView.Layout(gtx)
	return dims, f.Blocks.endFrame(), err
}
