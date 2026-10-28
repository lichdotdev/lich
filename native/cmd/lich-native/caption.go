package main

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// captionButton is one of the window's own buttons, drawn in the topbar.
type captionButton int

const (
	captionMinimize captionButton = iota
	captionMaximize
	captionClose
	captionButtons
)

// captionStyle is whose look the window's buttons copy.
type captionStyle int

const (
	captionStyleWindows captionStyle = iota
	captionStyleGNOME
	captionStyleKDE
)

// captionLayout is where the topbar puts the window's buttons and how it
// draws them: the platform's own arrangement.
type captionLayout struct {
	left, right []captionButton
	style       captionStyle
	// leftReserve is room at the band's left the system draws its own
	// buttons in (macOS's traffic lights), on top of lich's content.
	leftReserve unit.Dp
}

// windowChrome is what the window manager last told about the window.
type windowChrome struct {
	// decorated: the system draws the window's caption (a tiling window
	// manager, or a platform lich leaves it to), so the topbar adds none.
	decorated  bool
	maximized  bool
	fullscreen bool
}

func chromeOf(c app.Config) windowChrome {
	return windowChrome{decorated: c.Decorated, maximized: c.Mode == app.Maximized, fullscreen: c.Mode == app.Fullscreen}
}

// drawn reports whether the topbar is the window's caption: it drags the
// window, as a title bar does.
func (c windowChrome) drawn() bool {
	return !c.decorated && !c.fullscreen
}

// buttonsShown reports whether the topbar draws the window's buttons, or
// leaves macOS room for its own, under the window buttons setting.
func (c windowChrome) buttonsShown(choice string) bool {
	switch {
	case c.fullscreen || choice == windowButtonsHide:
		return false
	case choice == windowButtonsShow:
		return true
	}
	return !c.decorated
}

// under is the layout the topbar draws under the window buttons setting:
// Always show puts the three buttons on the right when the desktop's layout
// has none.
func (l captionLayout) under(choice string) captionLayout {
	if choice == windowButtonsShow && len(l.left)+len(l.right) == 0 && l.leftReserve == 0 {
		_, l.right = parseButtonLayout(defaultButtonLayout)
	}
	return l
}

// defaultButtonLayout is what a desktop without the setting gets: the three
// buttons on the right, as KDE and Pop!_OS place them.
const defaultButtonLayout = ":minimize,maximize,close"

// parseButtonLayout reads GNOME's button-layout, "left:right" with
// comma-separated names, keeping the three buttons lich draws in their order
// and dropping the rest (appmenu, icon, spacer).
func parseButtonLayout(s string) (left, right []captionButton) {
	l, r, _ := strings.Cut(s, ":")
	return buttonNames(l), buttonNames(r)
}

func buttonNames(s string) []captionButton {
	var out []captionButton
	for name := range strings.SplitSeq(s, ",") {
		switch strings.TrimSpace(name) {
		case "minimize":
			out = append(out, captionMinimize)
		case "maximize":
			out = append(out, captionMaximize)
		case "close":
			out = append(out, captionClose)
		}
	}
	return out
}

// The buttons' geometry, from the approved titlebar mockup.
var (
	windowsButtonWidth = unit.Dp(46) // Windows 11's caption buttons
	windowsGlyph       = ui.Space(3)
	gnomeButton        = ui.Space(6) // libadwaita's 24px round buttons
	gnomeGap           = unit.Dp(10)
	gnomeGlyph         = ui.Space(3.5)
	kdeButton          = unit.Dp(22) // Breeze's buttons
	kdeGap             = ui.Space(1.5)
	kdeGlyph           = ui.Space(4)
	captionSeparator   = ui.Space(5)
)

const (
	captionHoverFill   = 0.09
	captionPressedFill = 0.15
	// The fill under GNOME's round buttons at rest.
	gnomeRestFill = 0.09
)

var (
	// windowsCloseHover is the red Windows 11 paints under a hovered Close,
	// whatever the theme.
	windowsCloseHover = color.NRGBA{R: 0xc4, G: 0x2b, B: 0x1c, A: 0xff}
	onCloseHover      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

var (
	icMinimize    = icons.Lucide("minus")
	icMaximize    = icons.Lucide("square")
	icRestore     = icons.Lucide("copy")
	icCaptionX    = icons.Lucide("x")
	icKDEMinimize = icons.Lucide("chevron-down")
	icKDEMaximize = icons.Lucide("chevron-up")
	icKDERestore  = icons.Lucide("diamond")
)

// captionControls are the window's buttons in the topbar.
type captionControls struct {
	clicks [captionButtons]widget.Clickable
	hints  [captionButtons]ui.Tooltip
}

// layout draws buttons in a row the band's height, in style, and reports a
// button clicked this frame.
func (c *captionControls) layout(gtx C, th *ui.Theme, root *ui.Root, style captionStyle, buttons []captionButton, chrome windowChrome) (D, captionButton, bool) {
	clicked, ok := captionButton(0), false
	children := make([]layout.FlexChild, 0, 2*len(buttons))
	for i, b := range buttons {
		if c.clicks[b].Clicked(gtx) {
			clicked, ok = b, true
		}
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: captionGap(style)}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			tip := th.Tooltip(root, &c.hints[b], captionLabel(b, chrome))
			tip.Side = ui.SideBottom
			return tip.Layout(gtx, func(gtx C) D { return c.button(gtx, th, style, b, chrome) })
		}))
	}
	d := layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	return d, clicked, ok
}

func captionGap(style captionStyle) unit.Dp {
	switch style {
	case captionStyleGNOME:
		return gnomeGap
	case captionStyleKDE:
		return kdeGap
	}
	return 0
}

func captionLabel(b captionButton, chrome windowChrome) string {
	switch b {
	case captionMinimize:
		return i18n.T("shell.windowControls.minimize")
	case captionMaximize:
		if chrome.maximized {
			return i18n.T("shell.windowControls.restore")
		}
		return i18n.T("shell.windowControls.maximize")
	}
	return i18n.T("shell.windowControls.close")
}

func (c *captionControls) button(gtx C, th *ui.Theme, style captionStyle, b captionButton, chrome windowChrome) D {
	click := &c.clicks[b]
	return click.Layout(gtx, func(gtx C) D {
		switch style {
		case captionStyleWindows:
			return windowsButton(gtx, th, click, b, chrome)
		case captionStyleKDE:
			return kdeButtonFace(gtx, th, click, b, chrome)
		}
		return gnomeButtonFace(gtx, th, click, b, chrome)
	})
}

// windowsButton is a 46-wide caption button the band's full height.
func windowsButton(gtx C, th *ui.Theme, click *widget.Clickable, b captionButton, chrome windowChrome) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(windowsButtonWidth), gtx.Constraints.Max.Y))
	fg := th.Foreground
	switch {
	case b == captionClose && (click.Hovered() || click.Pressed()):
		ui.Fill(gtx, windowsCloseHover, 0)
		fg = onCloseHover
	case click.Pressed():
		ui.Fill(gtx, th.Over(ui.Alpha(th.Foreground, captionPressedFill)), 0)
	case click.Hovered():
		ui.Fill(gtx, th.Over(ui.Alpha(th.Foreground, captionHoverFill)), 0)
	}
	return layout.Center.Layout(gtx, func(gtx C) D { return captionIcon(captionStyleWindows, b, chrome).Layout(gtx, windowsGlyph, fg) })
}

// gnomeButtonFace is libadwaita's round button: a faint disc at rest.
func gnomeButtonFace(gtx C, th *ui.Theme, click *widget.Clickable, b captionButton, chrome windowChrome) D {
	size := gtx.Dp(gnomeButton)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	fill := float32(gnomeRestFill)
	if click.Hovered() || click.Pressed() {
		fill = captionPressedFill
	}
	ui.Fill(gtx, th.Over(ui.Alpha(th.Foreground, fill)), gnomeButton/2)
	return layout.Center.Layout(gtx, func(gtx C) D { return captionIcon(captionStyleGNOME, b, chrome).Layout(gtx, gnomeGlyph, th.Foreground) })
}

// kdeButtonFace is Breeze's bare glyph; Close gets the red disc on hover.
func kdeButtonFace(gtx C, th *ui.Theme, click *widget.Clickable, b captionButton, chrome windowChrome) D {
	size := gtx.Dp(kdeButton)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	fg := th.Foreground
	hot := click.Hovered() || click.Pressed()
	switch {
	case hot && b == captionClose:
		ui.Fill(gtx, th.Destructive, kdeButton/2)
		fg = onCloseHover
	case hot:
		ui.Fill(gtx, th.Over(ui.Alpha(th.Foreground, captionPressedFill)), kdeButton/2)
	}
	return layout.Center.Layout(gtx, func(gtx C) D { return captionIcon(captionStyleKDE, b, chrome).Layout(gtx, kdeGlyph, fg) })
}

// captionIcon is b's glyph in style; Maximize turns into Restore while the
// window is maximized.
func captionIcon(style captionStyle, b captionButton, chrome windowChrome) *icons.Icon {
	kde := style == captionStyleKDE
	switch {
	case b == captionClose:
		return icCaptionX
	case b == captionMinimize && kde:
		return icKDEMinimize
	case b == captionMinimize:
		return icMinimize
	case chrome.maximized && kde:
		return icKDERestore
	case chrome.maximized:
		return icRestore
	case kde:
		return icKDEMaximize
	}
	return icMaximize
}

// performCaption carries out a click on b. Close goes through the window
// manager's own close, so onClosing answers it as it answers any other.
func (u *window) performCaption(b captionButton) {
	switch b {
	case captionMinimize:
		u.win.Option(app.Minimized.Option())
	case captionMaximize:
		mode := app.Maximized
		if u.chrome.maximized {
			mode = app.Windowed
		}
		u.win.Option(mode.Option())
	case captionClose:
		u.win.Perform(system.ActionClose)
	}
}

// toggleFullscreen leaves the caption's buttons to windowChrome.drawn, which
// hides them while the window is full screen.
func (u *window) toggleFullscreen() {
	mode := app.Fullscreen
	if u.chrome.fullscreen {
		mode = app.Windowed
	}
	u.win.Option(mode.Option())
}
