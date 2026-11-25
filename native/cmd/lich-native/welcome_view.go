package main

import (
	"gioui.org/io/semantic"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/grimoire"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The Welcome's geometry, from the approved mockup (welcome-sim.html).
var (
	welcomeCellPadX = ui.Space(4.5)
	welcomePadY     = ui.Space(9)
	welcomePadX     = ui.Space(10)
	welcomeTagTop   = ui.Space(3)
	welcomeFootTop  = ui.Space(8.5)
	// welcomeHintHeight is the row under the books that says how to choose
	// the default, .hint's 13px line in 14px of padding, centered in it.
	welcomeHintHeight unit.Dp = 46
	// welcomeCellMinWidth is the narrowest a book's cell reads in: the book,
	// its gap and a name; welcomeIntroMinWidth the narrowest the logo and the
	// title fit, under which the intro gives its room to the books.
	welcomeCellMinWidth  unit.Dp = 300
	welcomeIntroMinWidth unit.Dp = 360
	// welcomeBookPixel is a book pixel's side, three device pixels at 1x;
	// welcomeLogoPixel the most a logo pixel takes, less in a short window.
	welcomeBookPixel unit.Dp = 3
	welcomeLogoPixel unit.Dp = 5
	welcomeColumns           = 2
)

const (
	welcomeTitleSize    unit.Sp = 54
	welcomeTitleLeading unit.Sp = 55 // leading-[1.02]
	welcomeNameSize     unit.Sp = 19
	welcomeStatusSize   unit.Sp = 13
	// welcomeAccentMuted is how far symbol 5 goes from Accent toward
	// MutedForeground (the mockup's INK).
	welcomeAccentMuted = 0.45
)

// layoutWelcome is the Welcome, over the window's whole content under the top
// strip, which keeps the window's controls: an island on the canvas, as the
// screens under it are. Continue, Enter and Escape close it, the first launch
// included; a login running in its own dialog over it keeps it open. It is
// laid out before the dialogs a sign-in opens, so they draw over it.
func (u *window) layoutWelcome(gtx C, th *ui.Theme) {
	u.decideWelcome()
	w := &u.welcome
	if !w.open {
		return
	}
	if w.cont.Clicked(gtx) || welcomeKey(gtx, key.NameReturn) || welcomeKey(gtx, key.NameEscape) && !w.signing {
		u.closeWelcome()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	if !th.Overlays.Open() && !gtx.Focused(&w.tag) {
		gtx.Execute(key.FocusCmd{Tag: &w.tag})
	}
	area := image.Rect(0, gtx.Dp(topbarHeight), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	u.coverPages(area)
	macro := op.Record(gtx.Ops)
	w.signing = u.welcomeScreen(gtx, th, area)
	op.Defer(gtx.Ops, macro.Stop())
}

// welcomeKey reports a press of name, as a dialog reads its Escape: the
// Welcome takes the focus off whatever is under it, a sign-in dialog over it
// takes it first.
func welcomeKey(gtx C, name key.Name) bool {
	pressed := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: name})
		if !ok {
			return pressed
		}
		if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
			pressed = true
		}
	}
}

// welcomeScreen draws the Welcome in area: the canvas, taking every press so
// nothing under it answers, and the island with the intro and the books. The
// books take what they need, the intro the rest, and a window too narrow for
// both shows the books alone. It reports a login running for a provider.
func (u *window) welcomeScreen(gtx C, th *ui.Theme, area image.Rectangle) bool {
	w := &u.welcome
	defer op.Offset(area.Min).Push(gtx.Ops).Pop()
	size := area.Size()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &w.tag)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &w.tag, Kinds: pointer.Press | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1, Max: 1}}); !ok {
			break
		}
	}
	paint.FillShape(gtx.Ops, th.Canvas, clip.Rect{Max: size}.Op())
	g := gtx.Dp(gutter)
	island := image.Rect(g, 0, size.X-g, size.Y-g)
	defer op.Offset(island.Min).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(island.Size())
	list := welcomeProviders(u.settings.providers.snapshot())
	rows := (len(list) + welcomeColumns - 1) / welcomeColumns
	books := min(island.Dx(), max(island.Dx()/2, welcomeColumns*gtx.Dp(welcomeCellMinWidth)))
	if island.Dx()-books < gtx.Dp(welcomeIntroMinWidth) {
		books = island.Dx()
	}
	signing := false
	pullsIsland(gtx, th, th.Background, func(gtx C, on *ui.Theme) D {
		var children []layout.FlexChild
		if books < island.Dx() {
			children = append(children,
				layout.Flexed(1, func(gtx C) D { return u.welcomeIntro(gtx, on) }),
				layout.Rigid(func(gtx C) D { return seam(gtx, on, image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)) }))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(books-gtx.Dp(1), island.Dy()))
			var d D
			// Alone, the books carry the way out on their last line.
			var foot layout.Widget
			if books == island.Dx() {
				foot = func(gtx C) D { return u.welcomeFoot(gtx, on) }
			}
			d, signing = u.welcomeGrid(gtx, on, list, rows, foot)
			return d
		}))
		return layout.Flex{}.Layout(gtx, children...)
	})
	return signing
}

// seam is the room between the intro and the books, or between two cells,
// left empty: they stand apart by space, not a line.
func seam(gtx C, th *ui.Theme, size image.Point) D {
	return D{Size: size}
}

// welcomeIntro is the left half: the logo burning, the title, the line under
// it and the way out.
func (u *window) welcomeIntro(gtx C, th *ui.Theme) D {
	return layout.Inset{Top: welcomePadY, Bottom: welcomePadY, Left: welcomePadX, Right: welcomePadX}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return welcomeLogo(gtx, th) }),
			layout.Rigid(func(gtx C) D { return welcomeTitle(gtx, th) }),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: welcomeTagTop}.Layout(gtx, th.Text(ui.TextBase, i18n.T("shell.welcome.tagline")).Muted().Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: welcomeFootTop}.Layout(gtx, func(gtx C) D { return u.welcomeFoot(gtx, th) })
			}),
		)
	})
}

// welcomeFoot is the way out: Continue at the end of the line, wearing
// Enter's cap; Esc closes it too, as it closes every dialog.
func (u *window) welcomeFoot(gtx C, th *ui.Theme) D {
	cont := th.Button(&u.welcome.cont, i18n.T("shell.welcome.continue"))
	cont.Size, cont.Keys = ui.SizeSM, "↵"
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
		layout.Rigid(cont.Layout),
	)
}

// welcomeTitle is the title a line at a time, the line with lich's name in
// the muted color: where the name falls is the language's to say.
func welcomeTitle(gtx C, th *ui.Theme) D {
	var lines []layout.FlexChild
	for line := range strings.SplitSeq(i18n.T("shell.welcome.title", i18n.Args{"name": brandName}), "\n") {
		l := th.Text(welcomeTitleSize, line).Weight(font.SemiBold).Leading(welcomeTitleLeading)
		if strings.Contains(line, brandName) {
			l = l.Muted()
		}
		lines = append(lines, layout.Rigid(l.Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
}

// welcomeLogo is the logo's flames at the frame of gtx.Now, centered in what
// the column leaves it, at the largest whole pixel that fits.
func welcomeLogo(gtx C, th *ui.Theme) D {
	frame, next := grimoire.LogoFrame(gtx.Now)
	if th.Still {
		next = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: next})
	room := gtx.Constraints.Max
	px := max(1, min(gtx.Dp(welcomeLogoPixel), room.Y/grimoire.LogoRows, room.X/grimoire.LogoCols))
	size := image.Pt(grimoire.LogoCols*px, grimoire.LogoRows*px)
	defer op.Offset(room.Sub(size).Div(2)).Push(gtx.Ops).Pop()
	paintGrid(gtx, th, grimoire.Logo[frame], px)
	return D{Size: room}
}

// welcomeStatus is where the provider is, in one line; a failure is the
// install's own line, whole in its tooltip.
func welcomeStatus(gtx C, th *ui.Theme, root *ui.Root, t *tome, s tomeState, f tomeFacts) D {
	l := th.Text(welcomeStatusSize, welcomeStatusText(s)).Muted().Truncated()
	switch s {
	case tomeReady:
		// the check says it is ready; its word is for screen readers
		d := icCheck.Layout(gtx, ui.Space(3.5), th.TonePass)
		defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
		semantic.LabelOp(welcomeStatusText(s)).Add(gtx.Ops)
		return d
	case tomeFailed:
		l = th.Text(welcomeStatusSize, f.install.failure).In(th.Destructive).Truncated()
		return th.Tooltip(root, &t.tip, f.install.failure).Layout(gtx, l.Layout)
	}
	return l.Layout(gtx)
}

func welcomeStatusText(s tomeState) string {
	switch s {
	case tomeInstalling:
		return i18n.T("shell.welcome.installing")
	case tomeSignedOut:
		return i18n.T("shell.welcome.signedOut")
	case tomeSigningIn:
		return i18n.T("shell.welcome.signingIn")
	case tomeReady:
		return i18n.T("shell.welcome.ready")
	}
	return i18n.T("shell.welcome.notInstalled")
}

// welcomeVerb is the state's one action, "" for a state that waits.
func welcomeVerb(p providerInfo, s tomeState) string {
	switch {
	case s == tomeNotInstalled && !p.installable:
		return i18n.T("shell.welcome.installPage")
	case s == tomeNotInstalled:
		return i18n.T("shell.welcome.install")
	case s == tomeFailed:
		return i18n.T("shell.welcome.retry")
	case s == tomeSignedOut:
		return i18n.T("shell.welcome.signIn")
	}
	return ""
}

// paintGrid paints g at px device pixels a symbol, as one image. A rectangle a
// run was thousands of draws a frame, and the inks' OKLab mixes a fifth of the
// window's CPU, while the Welcome animated.
func paintGrid(gtx C, th *ui.Theme, g grimoire.Grid, px int) {
	k := gridKey{cells: gridCells(g), px: px}
	for i := range len(gridSymbols) {
		k.inks[i], _ = grimoireInk(th, gridSymbols[i])
	}
	img := gridImages.get(gtx.Now, k, g)
	img.Add(gtx.Ops)
	defer clip.Rect(image.Rectangle{Max: img.Size()}).Push(gtx.Ops).Pop()
	paint.PaintOp{}.Add(gtx.Ops)
}

// gridSymbols are the symbols a grid inks; any other stays clear.
const gridSymbols = "12345"

// gridKey is how a painted grid looks: its symbols row by row, a symbol's side
// in device pixels, and the theme's ink for each of gridSymbols.
type gridKey struct {
	cells string
	px    int
	inks  [len(gridSymbols)]color.NRGBA
}

func gridCells(g grimoire.Grid) string {
	var b strings.Builder
	for _, row := range g {
		b.Write(row)
		b.WriteByte('\n')
	}
	return b.String()
}

// gridImageCache keeps the images of the grids the last two frames painted:
// a grid that holds still is rasterized once, and one an animation left is
// let go a frame later.
type gridImageCache struct {
	now       time.Time
	prev, cur map[gridKey]paint.ImageOp
}

var gridImages gridImageCache

func (c *gridImageCache) get(now time.Time, k gridKey, g grimoire.Grid) paint.ImageOp {
	if c.cur == nil {
		c.prev, c.cur = map[gridKey]paint.ImageOp{}, map[gridKey]paint.ImageOp{}
	}
	if !now.Equal(c.now) {
		c.now, c.prev, c.cur = now, c.cur, c.prev
		clear(c.cur)
	}
	if img, ok := c.cur[k]; ok {
		return img
	}
	img, ok := c.prev[k]
	if !ok {
		img = paint.NewImageOp(rasterGrid(g, k.px, k.inks))
		img.Filter = paint.FilterNearest
	}
	c.cur[k] = img
	return img
}

// rasterGrid draws g at px pixels a symbol, each of gridSymbols in its ink.
func rasterGrid(g grimoire.Grid, px int, inks [len(gridSymbols)]color.NRGBA) *image.NRGBA {
	cols := 0
	for _, row := range g {
		cols = max(cols, len(row))
	}
	img := image.NewNRGBA(image.Rect(0, 0, cols*px, len(g)*px))
	for y, row := range g {
		for x, s := range row {
			if i := strings.IndexByte(gridSymbols, s); i >= 0 {
				draw.Draw(img, image.Rect(x*px, y*px, (x+1)*px, (y+1)*px), image.NewUniform(inks[i]), image.Point{}, draw.Src)
			}
		}
	}
	return img
}

// grimoireInk is a symbol's color: the chat flame's four, and 5 between
// Accent and MutedForeground.
func grimoireInk(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	if symbol == '5' {
		return ui.MixOKLab(th.Accent, th.MutedForeground, welcomeAccentMuted), true
	}
	return chatFlameColor(th, symbol)
}
