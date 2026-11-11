package ui

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// ImageData is a decoded image ready to draw.
type ImageData struct {
	op paint.ImageOp
}

// NewImageData prepares an already decoded image.
func NewImageData(img image.Image) *ImageData {
	return &ImageData{op: paint.NewImageOp(img)}
}

// Size is the image's own size in pixels.
func (d *ImageData) Size() image.Point { return d.op.Size() }

// DecodeImage decodes data, PNG or JPEG out of the box; a caller that needs
// another format registers it with a blank import.
func DecodeImage(data []byte) (*ImageData, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return NewImageData(img), nil
}

// ImageStyle draws Src at Width by Height, zero for the constraint's width or
// height, by Fit (widget.Contain by default) at Position.
type ImageStyle struct {
	Src           *ImageData
	Fit           widget.Fit
	Position      layout.Direction
	Width, Height unit.Dp
}

// Image is Src contained and centred in the room it is given.
func (th *Theme) Image(src *ImageData) ImageStyle {
	return ImageStyle{Src: src, Fit: widget.Contain, Position: layout.Center}
}

func (s ImageStyle) Layout(gtx C) D {
	if s.Width > 0 {
		w := min(gtx.Dp(s.Width), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
	}
	if s.Height > 0 {
		h := min(gtx.Dp(s.Height), gtx.Constraints.Max.Y)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	}
	return widget.Image{Src: s.Src.op, Fit: s.Fit, Position: s.Position}.Layout(gtx)
}
