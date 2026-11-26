//go:build !linux && !windows && !darwin

package webview

import (
	"image"
	"image/color"

	"gioui.org/app"
)

// PrepareWindow leaves the window as it opens: no page sits in it here.
func PrepareWindow() {}

// SetParent records nothing: no page sits in the window here.
func SetParent(app.ViewEvent) uint64 { return 0 }

// ReleaseAll does nothing: no page sits in the window here.
func ReleaseAll() {}

// SetGround does nothing: no page sits in the window here.
func SetGround(color.NRGBA) {}

// KeyboardToWindow does nothing: no page takes the keyboard here.
func KeyboardToWindow() {}

// Page is a page of lich-browser's; none has a window here, the backend
// telling every tab's XID as zero, so the pane draws the screencast.
type Page struct{ xid uint64 }

func Embed(xid uint64) *Page                                { return &Page{xid: xid} }
func (p *Page) XID() uint64                                 { return p.xid }
func (*Page) Place(image.Rectangle, []image.Rectangle) bool { return false }
func (*Page) Placed() image.Rectangle                       { return image.Rectangle{} }
func (*Page) PointerToWindow()                              {}
func (*Page) PointerToPage()                                {}
func (*Page) Hide()                                         {}
func (*Page) Release()                                      {}
