package webview

import (
	"image"
	"image/color"

	"gioui.org/app"
	"golang.org/x/sys/windows"
)

// PrepareWindow leaves the window as it opens: any Win32 window can hold a
// page.
func PrepareWindow() {}

// parent is the HWND Gio draws into, zero while there is none.
var parent uintptr

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	gdi32            = windows.NewLazySystemDLL("gdi32.dll")
	procSetParent    = user32.NewProc("SetParent")
	procGetLong      = user32.NewProc("GetWindowLongPtrW")
	procSetLong      = user32.NewProc("SetWindowLongPtrW")
	procSetPos       = user32.NewProc("SetWindowPos")
	procShow         = user32.NewProc("ShowWindow")
	procSetRgn       = user32.NewProc("SetWindowRgn")
	procEnable       = user32.NewProc("EnableWindow")
	procSetFocus     = user32.NewProc("SetFocus")
	procCreateRect   = gdi32.NewProc("CreateRectRgn")
	procCombineRgn   = gdi32.NewProc("CombineRgn")
	procDeleteObject = gdi32.NewProc("DeleteObject")
)

// Win32 constants (winuser.h, wingdi.h).
const (
	gwlStyle        = ^uintptr(15) // GWL_STYLE, -16
	wsChild         = 0x40000000
	wsPopup         = 0x80000000
	swHide          = 0
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040
	rgnDiff         = 4
)

// offScreen is where lich-browser keeps a tab no window shows
// (browser_windows.cpp); a variable, as SetWindowPos reads it as a negative
// int from a uintptr's bits.
var offScreen = -20000

// SetParent records the window pages go in, from the window's ViewEvent, and
// answers its HWND, zero for none: the backend tells lich-browser. Gio sends
// the empty ViewEvent before it destroys its window, which would take the
// pages with it, so they go back to lich-browser then.
func SetParent(e app.ViewEvent) uint64 {
	w, ok := e.(app.Win32ViewEvent)
	if !ok || w.HWND == 0 {
		ReleaseAll()
		parent = 0
		return 0
	}
	if parent != w.HWND {
		ReleaseAll()
	}
	parent = w.HWND
	return uint64(parent)
}

func hasParent() bool { return parent != 0 }

func style(hwnd uintptr, set, clear uintptr) {
	s, _, _ := procGetLong.Call(hwnd, gwlStyle)
	procSetLong.Call(hwnd, gwlStyle, s&^clear|set)
}

// embed makes the page a child of the window: its style first, as SetParent
// asks of a window that becomes a child.
func embed(xid uint64, at image.Point) {
	h := uintptr(xid)
	style(h, wsChild, wsPopup)
	procSetParent.Call(h, parent)
	procSetPos.Call(h, 0, uintptr(at.X), uintptr(at.Y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
}

func place(xid uint64, r image.Rectangle) {
	procSetPos.Call(uintptr(xid), 0, uintptr(r.Min.X), uintptr(r.Min.Y), uintptr(r.Dx()), uintptr(r.Dy()),
		swpNoZOrder|swpNoActivate|swpShowWindow)
}

// shape leaves the cuts out of the page's region, which clips both what it
// draws and where it takes the pointer.
func shape(xid uint64, size image.Point, cuts []image.Rectangle) {
	if len(cuts) == 0 {
		procSetRgn.Call(uintptr(xid), 0, 1)
		return
	}
	rgn, _, _ := procCreateRect.Call(0, 0, uintptr(size.X), uintptr(size.Y))
	for _, c := range cuts {
		cut, _, _ := procCreateRect.Call(uintptr(c.Min.X), uintptr(c.Min.Y), uintptr(c.Max.X), uintptr(c.Max.Y))
		procCombineRgn.Call(rgn, rgn, cut, rgnDiff)
		procDeleteObject.Call(cut)
	}
	// The window owns the region from here on.
	procSetRgn.Call(uintptr(xid), rgn, 1)
}

// pointerTo enables the page or disables it, which sends the pointer over
// it to the window beneath.
func pointerTo(xid uint64, page bool) {
	on := uintptr(0)
	if page {
		on = 1
	}
	procEnable.Call(uintptr(xid), on)
}

func hide(xid uint64) { procShow.Call(uintptr(xid), swHide) }

// release hands the page back to lich-browser as it made it, a toplevel off
// screen: Windows tells lich-browser nothing of a window taken back.
func release(xid uint64) {
	h := uintptr(xid)
	procEnable.Call(h, 1)
	procSetRgn.Call(h, 0, 1)
	procSetParent.Call(h, 0)
	style(h, wsPopup, wsChild)
	off := offScreen
	procSetPos.Call(h, 0, uintptr(off), uintptr(off), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate|swpShowWindow|swpFrameChanged)
}

func keyboardToWindow() { procSetFocus.Call(parent) }

// paintGround does nothing: Gio paints the window where a page leaves it.
func paintGround(color.NRGBA) {}
