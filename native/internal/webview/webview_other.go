//go:build !linux

package webview

import (
	"errors"
	"image"

	"gioui.org/app"
)

// SetParent records the window pages go in; nothing holds a page here yet.
func SetParent(app.ViewEvent) {}

// Page is one web page; none opens on this system yet.
type Page struct{}

// Open says this system shows no page of its own yet, so the window draws
// the screencast instead.
func Open(string) (*Page, error) { return nil, errors.ErrUnsupported }

func (*Page) Place(image.Rectangle) bool { return false }
func (*Page) Hide()                      {}
func (*Page) Navigate(string)            {}
func (*Page) Close()                     {}
