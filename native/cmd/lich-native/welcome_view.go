package main

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
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
	welcomeMaxWidth   unit.Dp = 1180 // .win, max-w 1180px
	welcomeCellHeight unit.Dp = 155  // .tome, h-155px
	welcomeCellPadX           = ui.Space(4.5)
	welcomePadY               = ui.Space(9)
	welcomePadX               = ui.Space(10)
	welcomeTagTop             = ui.Space(3)
	welcomeFootTop            = ui.Space(8.5)
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

// layoutWelcome is the Welcome, over the whole window. Escape, a press off it
// and Continue close it; a login running in its own dialog keeps it open
// under that dialog.
func (u *window) layoutWelcome(gtx C, th *ui.Theme) {
	u.decideWelcome()
	w := &u.welcome
	if !w.open && !w.dialog.Animating(gtx) {
		return
	}
	list := welcomeProviders(u.settings.providers.snapshot())
	if w.cont.Clicked(gtx) || w.open && enterPressed(gtx) {
		u.closeWelcome()
	}
	on := th.On(th.Popover)
	signing := false
	d := th.Dialog(&w.dialog, w.open, "", "")
	d.MaxWidth, d.Root = welcomeMaxWidth, &u.root
	d.Raw = func(gtx C) D {
		rows := (len(list) + welcomeColumns - 1) / welcomeColumns
		h := min(rows*gtx.Dp(welcomeCellHeight), gtx.Constraints.Max.Y-gtx.Dp(ui.Space(8)))
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return u.welcomeIntro(gtx, on) }),
			layout.Rigid(func(gtx C) D { return seam(gtx, on, image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)) }),
			layout.Flexed(1, func(gtx C) D {
				var g D
				g, signing = u.welcomeGrid(gtx, on, list, rows)
				return g
			}),
		)
	}
	if d.Layout(gtx) && !signing {
		u.closeWelcome()
	}
}

func enterPressed(gtx C) bool {
	pressed := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameReturn})
		if !ok {
			return pressed
		}
		if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
			pressed = true
		}
	}
}

func seam(gtx C, th *ui.Theme, size image.Point) D {
	paint.FillShape(gtx.Ops, th.Border, clip.Rect{Max: size}.Op())
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
				return layout.Inset{Top: welcomeFootTop}.Layout(gtx, func(gtx C) D {
					cont := th.Button(&u.welcome.cont, i18n.T("shell.welcome.continue"))
					cont.Size = ui.SizeSM
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(th.Text(ui.TextXS, i18n.T("shell.welcome.escCloses")).Muted().Layout),
						layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
						layout.Rigid(cont.Layout),
					)
				})
			}),
		)
	})
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

// welcomeGrid is the right half: a cell per provider, two to a row, with a
// seam between them. signing is a login running for one of them.
func (u *window) welcomeGrid(gtx C, th *ui.Theme, list []providerInfo, rows int) (D, bool) {
	signing := false
	cell := image.Pt((gtx.Constraints.Max.X-gtx.Dp(1))/welcomeColumns, gtx.Constraints.Max.Y/max(rows, 1))
	line := gtx.Dp(1)
	for i, p := range list {
		col, row := i%welcomeColumns, i/welcomeColumns
		at := image.Pt(col*(cell.X+line), row*cell.Y)
		st := op.Offset(at).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(cell)
		if u.welcomeTome(cgtx, th, p) == tomeSigningIn {
			signing = true
		}
		st.Pop()
	}
	for col := 1; col < welcomeColumns; col++ {
		st := op.Offset(image.Pt(col*cell.X+(col-1)*line, 0)).Push(gtx.Ops)
		seam(gtx, th, image.Pt(line, gtx.Constraints.Max.Y))
		st.Pop()
	}
	for row := 1; row < rows; row++ {
		st := op.Offset(image.Pt(0, row*cell.Y)).Push(gtx.Ops)
		seam(gtx, th, image.Pt(gtx.Constraints.Max.X, line))
		st.Pop()
	}
	return D{Size: gtx.Constraints.Max}, signing
}

// welcomeTome is one provider's cell: its book, its name, where it is and the
// one action that moves it on. It returns the state it drew.
func (u *window) welcomeTome(gtx C, th *ui.Theme, p providerInfo) tomeState {
	t := u.welcome.tome(p.id)
	facts := u.welcomeFacts(p)
	state := tomeStateOf(facts)
	t.observe(p.id, state, facts.known || !facts.needsLogin, gtx.Now)
	g, next, playing := t.frame(gtx.Now)
	shot, pinned := u.welcome.shot[p.id]
	if pinned {
		state, facts.install, g, playing = shot.apply(p.id)
	}
	if playing && !pinned {
		gtx.Execute(op.InvalidateCmd{At: next})
	} else if !playing {
		g = restSprite(p.id, state, facts.install.progress)
	}
	if t.action.Clicked(gtx) && !playing {
		u.welcomeAction(p, state)
	}
	layout.Inset{Left: welcomeCellPadX, Right: welcomeCellPadX}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return welcomeBook(gtx, th, g) }),
			ui.Gap(welcomeGapUnits),
			layout.Flexed(1, func(gtx C) D { return u.welcomeLabels(gtx, th, p, t, state, facts, playing) }),
		)
	})
	return state
}

// welcomeGapUnits is the gap between a book and its labels, and
// welcomeActionGap the one over the action, on the spacing scale.
const (
	welcomeGapUnits  = 4
	welcomeActionGap = 2
)

func welcomeBook(gtx C, th *ui.Theme, g grimoire.Grid) D {
	px := max(1, gtx.Dp(welcomeBookPixel))
	paintGrid(gtx, th, g, px)
	return D{Size: image.Pt(grimoire.Cols*px, grimoire.Rows*px)}
}

func (u *window) welcomeLabels(gtx C, th *ui.Theme, p providerInfo, t *tome, s tomeState, f tomeFacts, playing bool) D {
	children := []layout.FlexChild{
		layout.Rigid(th.Text(welcomeNameSize, p.name).Weight(font.SemiBold).Truncated().Layout),
		layout.Rigid(func(gtx C) D { return welcomeStatus(gtx, th, &u.root, t, s, f) }),
	}
	verb := welcomeVerb(p, s)
	if verb != "" && !playing {
		children = append(children, ui.VGap(welcomeActionGap), layout.Rigid(func(gtx C) D {
			b := th.Button(&t.action, verb)
			b.Variant, b.Size = ui.VariantSecondary, ui.SizeXS
			if s == tomeSignedOut && u.signInBlocked(p) {
				b.Disabled = true
				return th.Tooltip(&u.root, &t.tip, i18n.T("shell.welcome.signInNeedsProject")).Layout(gtx, b.Layout)
			}
			return b.Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// welcomeStatus is where the provider is, in one line; a failure is the
// install's own line, whole in its tooltip.
func welcomeStatus(gtx C, th *ui.Theme, root *ui.Root, t *tome, s tomeState, f tomeFacts) D {
	l := th.Text(welcomeStatusSize, welcomeStatusText(s)).Muted().Truncated()
	switch s {
	case tomeReady:
		l = l.In(th.TonePass)
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

// paintGrid paints g at px device pixels a symbol, each run of one symbol on a
// row as one rectangle.
func paintGrid(gtx C, th *ui.Theme, g grimoire.Grid, px int) {
	for y, row := range g {
		for x := 0; x < len(row); {
			run := 1
			for x+run < len(row) && row[x+run] == row[x] {
				run++
			}
			if c, ok := grimoireInk(th, row[x]); ok {
				paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(x*px, y*px, (x+run)*px, (y+1)*px)).Op())
			}
			x += run
		}
	}
}

// grimoireInk is a symbol's color: the chat flame's four, and 5 between
// Accent and MutedForeground.
func grimoireInk(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	if symbol == '5' {
		return ui.MixOKLab(th.Accent, th.MutedForeground, welcomeAccentMuted), true
	}
	return chatFlameColor(th, symbol)
}
