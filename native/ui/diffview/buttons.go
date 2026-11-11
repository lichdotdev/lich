package diffview

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icRevertBlock = icons.Lucide("undo-2")

// The buttons drawn inside a diff, expand and revert: 0.6875rem text, 0.375rem
// horizontal padding, a 0.25rem radius, a 0.75rem icon.
const (
	affordanceText   = unit.Sp(11)
	expandLeading    = unit.Sp(13.2) // line-height: 1.2
	revertLeading    = unit.Sp(14.3) // line-height: 1.3
	affordancePadX   = unit.Dp(6)
	affordanceRadius = unit.Dp(4)
	affordanceGap    = unit.Dp(4)
	affordanceIcon   = unit.Dp(12)
)

// Labels are the words on a diff's buttons, read when each is drawn so a
// change of language shows at once.
type Labels struct {
	Expand func(count int) string
	Revert func() string
}

// Buttons owns the clickables behind a diff's buttons, keyed by what they act
// on so a button keeps its state across the rebuilds its own click causes.
// theme is the one on screen, set every frame by Layout.
type Buttons struct {
	labels Labels
	theme  *ui.Theme
	expand map[int]*widget.Clickable
	revert map[int]*widget.Clickable
}

// NewButtons builds the Buttons Options takes.
func NewButtons(labels Labels) *Buttons {
	return &Buttons{labels: labels, expand: map[int]*widget.Clickable{}, revert: map[int]*widget.Clickable{}}
}

func clickable(m map[int]*widget.Clickable, key int) *widget.Clickable {
	c, ok := m[key]
	if !ok {
		c = new(widget.Clickable)
		m[key] = c
	}
	return c
}

// Expand offers a gap's lines, keyed by its last line, which stays put
// while the gap fills from its top.
func (a *Buttons) Expand(g godemirror.Gap, onClick func()) layout.Widget {
	c := clickable(a.expand, g.To)
	count := g.To - g.From + 1
	return func(gtx C) D {
		text := a.labels.Expand(count)
		for c.Clicked(gtx) {
			onClick()
		}
		th := a.theme
		fg, bg := th.MutedForeground, color.NRGBA{}
		if c.Hovered() {
			fg, bg = th.AccentForeground, th.Accent
		}
		return a.button(gtx, c, bg, color.NRGBA{}, th.Text(affordanceText, text).Leading(expandLeading).In(fg).Layout)
	}
}

// Revert undoes one chunk, keyed by where it starts.
func (a *Buttons) Revert(chunk godemirror.Chunk, onClick func()) layout.Widget {
	c := clickable(a.revert, chunk.OldStart)
	return func(gtx C) D {
		for c.Clicked(gtx) {
			onClick()
		}
		th := a.theme
		bg := th.Background
		if c.Hovered() {
			bg = th.Over(th.Accent)
		}
		return a.button(gtx, c, bg, th.Over(th.Border), func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icRevertBlock.Layout(gtx, affordanceIcon, th.Foreground) }),
				layout.Rigid(layout.Spacer{Width: affordanceGap}.Layout),
				layout.Rigid(th.Text(affordanceText, a.labels.Revert()).Leading(revertLeading).Layout),
			)
		})
	}
}

// button lays content out padded on a rounded background with an optional
// 1px border, as c's target, with the hand cursor.
func (a *Buttons) button(gtx C, c *widget.Clickable, bg, edge color.NRGBA, content layout.Widget) D {
	return c.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				size := gtx.Constraints.Min
				r := gtx.Dp(affordanceRadius)
				if edge.A != 0 {
					paint.FillShape(gtx.Ops, edge, clip.UniformRRect(image.Rectangle{Max: size}, r).Op(gtx.Ops))
					paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rect(1, 1, size.X-1, size.Y-1), r-1).Op(gtx.Ops))
				} else if bg.A != 0 {
					paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: size}, r).Op(gtx.Ops))
				}
				defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
				pointer.CursorPointer.Add(gtx.Ops)
				return D{Size: size}
			},
			func(gtx C) D {
				return layout.Inset{Left: affordancePadX, Right: affordancePadX}.Layout(gtx, content)
			},
		)
	})
}
