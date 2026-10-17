package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The notification icon is drawn at this many pixels a side: what GNOME and
// mako scale down to their 48–64 px slot without blurring. It is a disc with
// everything inside it, because swaync crops the image to a circle.
const (
	noteIconSize = 96
	noteIconMark = 46
	// noteIconMarkAt puts the mark up and left of center, clear of the badge.
	noteIconMarkAt = 17
	noteIconDot    = 34
	noteIconRing   = 4
	noteIconBadge  = 22
	// noteIconBadgeAt is the badge's center on both axes: as far into the
	// corner as the disc lets a badge of its size go.
	noteIconBadgeAt = 66
)

var noteIcons = struct {
	sync.Mutex
	written map[string]string
}{written: map[string]string{}}

// noteIconFile is kind's mark on a dark disc with status's badge in its
// corner, drawn off screen once per pair and kept as a PNG in lich's cache.
// It goes out as the notification's app icon rather than as image data:
// swaync lays the app's own icon over the corner of an image, and the badge
// already says it is lich.
func noteIconFile(kind, status string) (string, error) {
	key := kind + "-" + status
	noteIcons.Lock()
	defer noteIcons.Unlock()
	if path, ok := noteIcons.written[key]; ok {
		return path, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "lich", "notification-icons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	img, err := drawNoteIcon(kind, status)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, key+".png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	noteIcons.written[key] = path
	return path, nil
}

func drawNoteIcon(kind, status string) (*image.RGBA, error) {
	win, err := headless.NewWindow(noteIconSize, noteIconSize)
	if err != nil {
		return nil, fmt.Errorf("headless window: %w", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Constraints: layout.Exact(image.Pt(noteIconSize, noteIconSize))}
	pal := ui.Dark
	paint.FillShape(ops, pal.Card, clip.Ellipse{Max: image.Pt(noteIconSize, noteIconSize)}.Op(ops))
	t := op.Offset(image.Pt(noteIconMarkAt, noteIconMarkAt)).Push(ops)
	mark := gtx
	mark.Constraints = layout.Exact(image.Pt(noteIconMark, noteIconMark))
	providerIcon(kind).Layout(mark, unit.Dp(noteIconMark), pal.Foreground)
	t.Pop()
	drawNoteBadge(gtx, pal.Card, noteDotTone(pal, status))
	if err := win.Frame(ops); err != nil {
		return nil, fmt.Errorf("headless frame: %w", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: image.Pt(noteIconSize, noteIconSize)})
	if err := win.Screenshot(img); err != nil {
		return nil, fmt.Errorf("headless screenshot: %w", err)
	}
	return img, nil
}

func noteDotTone(pal ui.Palette, status string) color.NRGBA {
	if status == statusWaiting {
		return pal.ToneWait
	}
	return pal.TonePass
}

// drawNoteBadge is the status circle over the disc's bottom-right
// with the lich mark inside, ringed in the disc's color so it reads against
// the provider's mark. lich draws its own badge because servers that badge
// the app icon themselves (swaync) put a square one over the corner.
func drawNoteBadge(gtx layout.Context, ring, tone color.NRGBA) {
	ops := gtx.Ops
	outer := noteIconDot + 2*noteIconRing
	at := noteIconBadgeAt - outer/2
	paint.FillShape(ops, ring, clip.Ellipse{Min: image.Pt(at, at), Max: image.Pt(at+outer, at+outer)}.Op(ops))
	in := at + noteIconRing
	paint.FillShape(ops, tone, clip.Ellipse{Min: image.Pt(in, in), Max: image.Pt(in+noteIconDot, in+noteIconDot)}.Op(ops))
	inset := in + (noteIconDot-noteIconBadge)/2
	t := op.Offset(image.Pt(inset, inset)).Push(ops)
	badge := gtx
	badge.Constraints = layout.Exact(image.Pt(noteIconBadge, noteIconBadge))
	icons.LichMark().LayoutFilled(badge, unit.Dp(noteIconBadge), ring)
	t.Pop()
}
