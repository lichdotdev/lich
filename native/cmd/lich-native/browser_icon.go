package main

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"log"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	xdraw "golang.org/x/image/draw"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// tabIcon is a page's icon, scaled once per size it is drawn at: Gio only
// scales linearly, which shimmers when a 32px icon shrinks to 12dp. sized is
// only touched on the UI goroutine.
type tabIcon struct {
	src   image.Image
	sized map[int]paint.ImageOp
}

func (ic *tabIcon) at(px int) paint.ImageOp {
	if img, ok := ic.sized[px]; ok {
		return img
	}
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), ic.src, ic.src.Bounds(), xdraw.Over, nil)
	img := paint.NewImageOp(dst)
	ic.sized[px] = img
	return img
}

// tabIcon is the icon key names, nil while it is read, when it failed, or for
// none; the first ask reads it from the backend and redraws once it is in.
func (m *model) tabIcon(key string) *tabIcon {
	if m == nil || key == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ic, asked := m.browsers.icons[key]; asked || m.client == nil {
		return ic
	}
	m.browsers.icons[key] = nil
	go m.readTabIcon(m.client, key)
	return nil
}

func (m *model) readTabIcon(client *backend.Client, key string) {
	data, err := client.BrowserIcon(context.Background(), key)
	if err != nil {
		log.Printf("browser: read a page's icon: %v", err)
		return
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("browser: decode a page's icon: %v", err)
		return
	}
	m.mu.Lock()
	m.browsers.icons[key] = &tabIcon{src: img, sized: map[int]paint.ImageOp{}}
	m.mu.Unlock()
	m.invalidate()
}

// tabIconMark draws ic where a tab's mark sits, at x, and returns the width
// the mark takes.
func tabIconMark(gtx C, ic *tabIcon, x int) int {
	side := gtx.Dp(ui.Space(3))
	defer op.Offset(image.Pt(x, (gtx.Constraints.Max.Y-side)/2)).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: image.Pt(side, side)}.Push(gtx.Ops).Pop()
	ic.at(side).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return side + gtx.Dp(ui.Space(1.5))
}
