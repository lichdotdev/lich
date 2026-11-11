package extrender

import (
	"bytes"
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		extension.EmptyState.Name:       drawEmptyState,
		extension.InlineEmptyState.Name: drawEmptyState,
		extension.Avatar.Name:           drawAvatar,
		extension.SectionHeading.Name:   drawSectionHeading,
		extension.MarkedText.Name:       drawMarkedText,
		extension.Image.Name:            drawImage,
		extension.FileIcon.Name:         drawFileIcon,
		extension.SkeletonBar.Name:      drawSkeletonBar,
		extension.SkeletonText.Name:     drawSkeletonText,
		extension.SkeletonShape.Name:    drawSkeletonShape,
		extension.SkeletonFill.Name:     drawSkeletonFill,
		extension.SkeletonRow.Name:      drawSkeletonRow,
		extension.Stack.Name:            drawStack,
		extension.Row.Name:              drawRow,
		extension.Columns.Name:          drawColumns,
		extension.FixedColumn.Name:      drawFixedColumn,
		extension.AlignedEnd.Name:       drawAlignEnd,
		extension.Spanning.Name:         drawSpanning,
		extension.SplitPane.Name:        drawSplitPane,
		extension.ThinScrollArea.Name:   drawThinScroll,
	})
}

// fileIconSize is the kit's own file glyph in the window's lists.
var fileIconSize = ui.Space(3.5)

// spaces is a gap in dp as the kit's Space units.
func spaces(dp float64) float32 { return float32(dp / 4) }

// stacked is n's children one under another, the content of a component
// that wraps what it is given.
func (r *Renderer) stacked(th *ui.Theme, n extension.Node, id string) layout.Widget {
	return ui.Stack(0, r.children(th, n, id)...)
}

func drawEmptyState(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.EmptyStateProps](n)
	var actions layout.Widget
	if len(n.Children) > 0 {
		actions = ui.Row(2, r.children(th, n, id)...)
	}
	e := th.EmptyState(r.icon(p.Icon), p.Title, p.Description, actions)
	if n.Type == extension.InlineEmptyState.Name {
		e = th.InlineEmptyState(p.Title, p.Description, actions)
		e.Icon = r.icon(p.Icon)
	}
	tuneEmptyState(&e, p)
	return e.Layout(gtx)
}

// tuneEmptyState overrides each default the extension set.
func tuneEmptyState(e *ui.EmptyStateStyle, p extension.EmptyStateProps) {
	if p.IconSize > 0 {
		e.IconSize = unit.Dp(p.IconSize)
	}
	setSpaces(&e.IconGap, p.IconGap)
	setSpaces(&e.DescriptionGap, p.DescriptionGap)
	setSpaces(&e.ActionsGap, p.ActionsGap)
	e.TitleSize = textSize(p.TitleSize, e.TitleSize)
	e.DescriptionSize = textSize(p.DescriptionSize, e.DescriptionSize)
	e.TitleLeading = textSize(p.TitleLeading, e.TitleLeading)
	if p.TitleWeight != "" {
		e.TitleWeight = weight(p.TitleWeight)
	}
	if p.MaxLines > 0 {
		e.MaxLines = p.MaxLines
	}
	if d, ok := anchorDirection(p.Anchor); ok {
		e.Anchor = d
	}
	if p.Align != "" {
		e.Align = alignment(p.Align)
	}
	if p.Pad > 0 {
		e.Pad = unit.Dp(p.Pad)
	}
}

func setSpaces(dst *float32, dp float64) {
	if dp > 0 {
		*dst = spaces(dp)
	}
}

func anchorDirection(a extension.Anchor) (layout.Direction, bool) {
	d, ok := map[extension.Anchor]layout.Direction{
		extension.AnchorTopLeft: layout.NW, extension.AnchorTop: layout.N, extension.AnchorTopRight: layout.NE,
		extension.AnchorLeft: layout.W, extension.AnchorCenter: layout.Center, extension.AnchorRight: layout.E,
		extension.AnchorBottomLeft: layout.SW, extension.AnchorBottom: layout.S, extension.AnchorBottomRight: layout.SE,
	}[a]
	return d, ok
}

// pictureState is a picture decoded once per change of its bytes; one that
// does not decode draws nothing until the bytes change.
type pictureState struct {
	decoded bool
	data    []byte
	avatar  *ui.AvatarImage
	image   *ui.ImageData
}

func (s *pictureState) decode(data []byte) *ui.ImageData {
	if s.decoded && bytes.Equal(data, s.data) {
		return s.image
	}
	s.decoded, s.data, s.image, s.avatar = true, bytes.Clone(data), nil, nil
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		s.image, s.avatar = ui.NewImageData(img), ui.NewAvatarImage(img)
	}
	return s.image
}

func drawAvatar(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.AvatarProps](n)
	s := stateOf[pictureState](r, id)
	s.decode(p.Image)
	a := th.Avatar(s.avatar, p.Initial, unit.Dp(p.Size))
	a.Background = r.color(th, p.Background, a.Background)
	a.Foreground = r.color(th, p.Foreground, a.Foreground)
	return a.Layout(gtx)
}

func drawSectionHeading(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SectionHeadingProps](n)
	h := th.SectionHeading(p.Text)
	h.Size = textSize(p.Size, h.Size)
	h.Leading = textSize(p.Leading, h.Leading)
	if p.Weight != "" {
		h.Weight = weight(p.Weight)
	}
	h.Inset = layout.Inset{Top: unit.Dp(p.PadTop), Right: unit.Dp(p.PadRight), Bottom: unit.Dp(p.PadBottom), Left: unit.Dp(p.PadLeft)}
	return h.Layout(gtx)
}

func drawMarkedText(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.MarkedTextProps](n)
	segs := make([]ui.MarkedSegment, len(p.Segments))
	for i, s := range p.Segments {
		segs[i] = ui.MarkedSegment{Text: s.Text, Marked: s.Marked}
	}
	return th.MarkedText(segs).Layout(gtx)
}

func drawImage(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ImageProps](n)
	src := stateOf[pictureState](r, id).decode(p.Data)
	if src == nil {
		return D{}
	}
	img := th.Image(src)
	img.Width, img.Height = unit.Dp(p.Width), unit.Dp(p.Height)
	if fit, ok := imageFits[p.Fit]; ok {
		img.Fit = fit
	}
	if d, ok := anchorDirection(p.Position); ok {
		img.Position = d
	}
	return img.Layout(gtx)
}

var imageFits = map[extension.ImageFit]widget.Fit{
	extension.ImageContain: widget.Contain, extension.ImageCover: widget.Cover, extension.ImageFill: widget.Fill,
	extension.ImageScaleDown: widget.ScaleDown, extension.ImageUnscaled: widget.Unscaled,
}

func drawFileIcon(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.FileIconProps](n)
	size := fileIconSize
	if p.Size > 0 {
		size = unit.Dp(p.Size)
	}
	return th.FileIcon(p.Path, size).Layout(gtx)
}

func drawSkeletonBar(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SkeletonBarProps](n)
	return th.SkeletonBar(unit.Dp(p.Width), unit.Dp(p.Line), unit.Dp(p.Height)).Layout(gtx)
}

func drawSkeletonText(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SkeletonTextProps](n)
	return th.SkeletonText(textSize(p.Size, ui.TextSM), unit.Dp(p.Width)).Layout(gtx)
}

func drawSkeletonShape(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return th.SkeletonShape(r.stacked(th, n, id)).Layout(gtx)
}

func drawSkeletonFill(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SkeletonFillProps](n)
	if len(n.Children) == 0 {
		return D{}
	}
	kids := r.children(th, n, id)
	return ui.SkeletonFill{MaxRows: p.MaxRows, PadY: unit.Dp(p.PadY)}.Layout(gtx, func(gtx C, i int) D {
		return kids[i%len(kids)](gtx)
	})
}

func drawSkeletonRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SkeletonRowProps](n)
	return ui.SkeletonRow{PadX: unit.Dp(p.PadX), PadY: unit.Dp(p.PadY)}.Layout(gtx, r.stacked(th, n, id))
}

func drawStack(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return ui.Stack(spaces(props[extension.GapProps](n).Gap), r.children(th, n, id)...)(gtx)
}

func drawRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return ui.Row(spaces(props[extension.GapProps](n).Gap), r.children(th, n, id)...)(gtx)
}

func drawColumns(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return ui.Columns(spaces(props[extension.GapProps](n).Gap), r.children(th, n, id)...)(gtx)
}

func drawFixedColumn(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	w := unit.Dp(props[extension.FixedColumnProps](n).Width)
	return layout.Flex{}.Layout(gtx, ui.FixedColumn(w, r.stacked(th, n, id)))
}

func drawAlignEnd(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return ui.AlignEnd(r.stacked(th, n, id))(gtx)
}

func drawSpanning(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return ui.Spanning(r.stacked(th, n, id))(gtx)
}

// splitState is the pane's drag and the ratio it shows: the extension's
// until a drag ends, then the dragged one until the extension sends another.
type splitState struct {
	pane  ui.SplitPane
	prop  float64
	ratio float64
}

func drawSplitPane(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SplitPaneProps](n)
	s := stateOf[splitState](r, id)
	if p.Ratio != s.prop {
		s.prop, s.ratio = p.Ratio, p.Ratio
	}
	s.pane.MinRatio, s.pane.MaxRatio = p.MinRatio, p.MaxRatio
	axis := layout.Horizontal
	if p.Direction == extension.DirectionColumn {
		axis = layout.Vertical
	}
	first, second := r.side(th, n, id, 0), r.side(th, n, id, 1)
	before := s.ratio
	d := s.pane.Layout(gtx, axis, &s.ratio, gtx.Dp(unit.Dp(p.Gap)), gtx.Dp(unit.Dp(p.MinFirst)), gtx.Dp(unit.Dp(p.MinSecond)), first, second)
	if s.ratio != before {
		r.fire(p.OnResize, extension.EventData{Ratio: s.ratio})
	}
	return d
}

// side is the i-th child of n, or nothing where the extension sent fewer.
func (r *Renderer) side(th *ui.Theme, n extension.Node, id string, i int) layout.Widget {
	if i >= len(n.Children) {
		return func(C) D { return D{} }
	}
	return r.child(th, n.Children[i], id, i)
}

func drawThinScroll(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	list := stateOf[widget.List](r, id)
	list.Axis = layout.Vertical
	if props[extension.ScrollProps](n).Direction == extension.DirectionRow {
		list.Axis = layout.Horizontal
	}
	kids := r.children(th, n, id)
	return th.ThinScrollArea(list).Layout(gtx, len(kids), func(gtx C, i int) D { return kids[i](gtx) })
}
