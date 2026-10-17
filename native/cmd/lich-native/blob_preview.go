package main

// An image read through /blob and drawn on the checkerboard with its
// dimensions, format and size under it: BinaryPreview.tsx's ImageView, for
// the Code tab's preview and a binary file's card in the Review.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	xdraw "golang.org/x/image/draw"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// maxPreviewPixels bounds an image by its header before it is decoded: the
// backend's 20 MB cap is on the compressed file, and a PNG that small can
// unpack to gigabytes. 40 MP is an 8K screenshot (33 MP) with room, held as
// 160 MB of RGBA for the moment it is scaled.
const maxPreviewPixels = 40_000_000

// maxPreviewEdge is the longest side an image is kept at once decoded. The
// dock is never wider than that at 2x, and every GPU Gio runs on takes a
// texture this size.
const maxPreviewEdge = 2048

// previewMaxHeight is BinaryPreview's max-h-[32rem].
const previewMaxHeight = 128

// checkerCell is half of .preview-checker's 0.75rem tile.
const checkerCell = 1.5

type blobKind uint8

const (
	blobNone blobKind = iota
	blobImage
	blobPDF
)

// imageExtensions are the formats internal/project.previewTypes serves that
// this binary decodes; AVIF and ICO, which Chromium draws, fall to the state
// with no preview.
var imageExtensions = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}

func previewKindOf(rel string) blobKind {
	ext := strings.ToLower(path.Ext(rel))
	switch {
	case ext == ".pdf":
		return blobPDF
	case imageExtensions[ext]:
		return blobImage
	}
	return blobNone
}

// previewImage is a decoded image, scaled to maxPreviewEdge, and what its
// caption says about the file.
type previewImage struct {
	src           paint.ImageOp
	width, height int
	format        string
	bytes         int
}

func (p *previewImage) caption() string {
	return fmt.Sprintf("%d × %d · %s · %s", p.width, p.height, p.format, formatBlobSize(p.bytes))
}

func decodePreview(data []byte) (*previewImage, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New(i18n.T("diff.blobPreview.undecodable", i18n.Args{"error": err}))
	}
	if cfg.Width*cfg.Height > maxPreviewPixels {
		return nil, errors.New(i18n.T("diff.blobPreview.tooManyPixels", i18n.Args{"width": cfg.Width, "height": cfg.Height, "megapixels": maxPreviewPixels / 1_000_000}))
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New(i18n.T("diff.blobPreview.undecodable", i18n.Args{"error": err}))
	}
	return &previewImage{
		src:    paint.NewImageOp(fitEdge(img, maxPreviewEdge)),
		width:  cfg.Width,
		height: cfg.Height,
		format: strings.ToUpper(format),
		bytes:  len(data),
	}, nil
}

// fitEdge scales img down so its longest side is at most edge.
func fitEdge(img image.Image, edge int) image.Image {
	b := img.Bounds()
	longest := max(b.Dx(), b.Dy())
	if longest <= edge {
		return img
	}
	w, h := max(1, b.Dx()*edge/longest), max(1, b.Dy()*edge/longest)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

const (
	blobBytesPerKB = 1024
	blobBytesPerMB = 1024 * 1024
)

// formatBlobSize is preview.ts formatBlobSize: KB below a megabyte, one
// decimal either way.
func formatBlobSize(n int) string {
	if n < blobBytesPerMB {
		return fmt.Sprintf("%.1f KB", float64(n)/blobBytesPerKB)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/blobBytesPerMB)
}

// blobView is one side of a preview: its read, decoded off the UI goroutine.
type blobView struct {
	read remote[*previewImage]
}

func newBlobView(invalidate func()) *blobView {
	v := &blobView{}
	v.read.invalidate = invalidate
	return v
}

// want reads rel at ref in checkout; version tells a new state of the file
// from the one held, as the web keys a side by its blob ids.
func (v *blobView) want(client *lichclient.Client, checkout, rel, ref, version string) {
	key := strings.Join([]string{checkout, rel, ref, version}, "\x00")
	v.read.want(key, checkout, func(ctx context.Context) (*previewImage, error) {
		data, err := client.Blob(ctx, checkout, rel, ref)
		if err != nil {
			return nil, err
		}
		return decodePreview(data)
	})
}

// layout is the image over its caption, or the line saying why there is none.
func (v *blobView) layout(gtx C, th *ui.Theme) D {
	img, loading, err := v.read.get()
	switch {
	case errors.Is(err, lichclient.ErrNoBlob):
		return th.Text(ui.TextXS, i18n.T("diff.binaryPreview.missing")).Muted().Layout(gtx)
	case err != nil:
		l := th.Text(ui.TextXS, lichclient.ErrorText(err)).Muted()
		l.MaxLines = 0
		return l.Layout(gtx)
	case loading || img == nil:
		return th.Text(ui.TextXS, i18n.T("diff.binaryPreview.loading")).Muted().Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return img.draw(gtx, th) }),
		ui.VGap(1),
		layout.Rigid(th.Text(text2XS, img.caption()).Mono().Muted().Leading(text2XSLeading).Layout),
	)
}

// draw is the image at its own size in dp, shrunk to fit the width and
// previewMaxHeight, centered on a checkerboard as wide as the column.
func (p *previewImage) draw(gtx C, th *ui.Theme) D {
	maxW, maxH := gtx.Constraints.Max.X, gtx.Dp(ui.Space(previewMaxHeight))
	w, h := float32(gtx.Dp(unit.Dp(p.width))), float32(gtx.Dp(unit.Dp(p.height)))
	scale := min(1, float32(maxW)/w, float32(maxH)/h)
	size := image.Pt(max(1, int(w*scale)), max(1, int(h*scale)))
	box := image.Pt(maxW, size.Y)
	defer clip.UniformRRect(image.Rectangle{Max: box}, gtx.Dp(ui.RadiusSM)).Push(gtx.Ops).Pop()
	checker(gtx, th, box)
	gtx.Constraints = layout.Exact(box)
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(size)
		return widget.Image{Src: p.src, Fit: widget.Fill}.Layout(gtx)
	})
}

// checker is .preview-checker: card ground, muted squares, so a transparent
// pixel reads as transparent rather than as the panel.
func checker(gtx C, th *ui.Theme, size image.Point) {
	paint.FillShape(gtx.Ops, th.Card, clip.Rect{Max: size}.Op())
	cell := gtx.Dp(ui.Space(checkerCell))
	var p clip.Path
	p.Begin(gtx.Ops)
	for y := 0; y*cell < size.Y; y++ {
		for x := y % 2; x*cell < size.X; x += 2 {
			r := image.Rect(x*cell, y*cell, min(size.X, (x+1)*cell), min(size.Y, (y+1)*cell))
			p.MoveTo(layout.FPt(r.Min))
			p.LineTo(layout.FPt(image.Pt(r.Max.X, r.Min.Y)))
			p.LineTo(layout.FPt(r.Max))
			p.LineTo(layout.FPt(image.Pt(r.Min.X, r.Max.Y)))
			p.Close()
		}
	}
	paint.FillShape(gtx.Ops, th.Muted, clip.Outline{Path: p.End()}.Op())
}
