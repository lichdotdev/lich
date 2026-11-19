//go:build !linux

package webview

import (
	"context"
	"errors"
	"image"
	"image/color"

	"gioui.org/app"
)

// PrepareWindow leaves the window as it opens: nothing holds a page here yet.
func PrepareWindow() {}

// SetParent records the window pages go in; nothing holds a page here yet.
func SetParent(app.ViewEvent) {}

// SetGround does nothing: no page sits in the window here.
func SetGround(color.NRGBA) {}

// KeyboardToWindow does nothing: no page takes the keyboard here.
func KeyboardToWindow() {}

// Page is one web page; none opens on this system yet.
type Page struct{}

// Open says this system shows no page of its own yet, so the window draws
// the screencast instead.
func Open(string, func(Load)) (*Page, error) { return nil, errors.ErrUnsupported }

func (*Page) Place(image.Rectangle, []image.Rectangle) bool { return false }
func (*Page) PointerToWindow()                              {}
func (*Page) PointerToPage()                                {}
func (*Page) URL() string                                   { return "" }
func (*Page) Eval(context.Context, string) (string, error)  { return "", errors.ErrUnsupported }
func (*Page) SetZoom(float64)                               {}
func (*Page) Hide()                                         {}
func (*Page) Navigate(string)                               {}
func (*Page) Close()                                        {}
