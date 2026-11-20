package main

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/grimoire"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// welcomeCell is one provider as the frame read it off the stores.
type welcomeCell struct {
	p     providerInfo
	facts tomeFacts
	state tomeState
}

// welcomeCells reads every provider's state, the shot's pins over it, and
// which ready book is the default.
func (u *window) welcomeCells(list []providerInfo) (cells []welcomeCell, main string) {
	var ready []string
	for _, p := range list {
		c := welcomeCell{p: p, facts: u.welcomeFacts(p)}
		c.state = tomeStateOf(c.facts)
		if shot, ok := u.welcome.shot[p.id]; ok {
			c.state = shot.state
			c.facts.install = installStep{running: shot.state == tomeInstalling, progress: shot.progress, failure: shot.failure}
			if shot.main {
				main = p.id
			}
		}
		if c.state == tomeReady {
			ready = append(ready, p.id)
		}
		cells = append(cells, c)
	}
	if main == "" {
		main = welcomeMain(u.settings.providers.snapshot().globals[providerDefaultKey], ready)
	}
	return cells, main
}

// welcomeGrid is the right half: a cell per provider, two to a row, with a
// seam between them, and under them how to choose the default once there is
// a choice, and foot at the line's end when it is given. signing is a login
// running for one of them.
func (u *window) welcomeGrid(gtx C, th *ui.Theme, list []providerInfo, rows int, foot layout.Widget) (D, bool) {
	cells, main := u.welcomeCells(list)
	for _, c := range cells {
		u.welcome.tome(c.p.id).observe(c.p.id, c.state, c.facts.known || !c.facts.needsLogin, gtx.Now)
	}
	u.welcome.observeMain(main, gtx.Now)
	hint := gtx.Dp(welcomeHintHeight)
	line := gtx.Dp(1)
	cell := image.Pt((gtx.Constraints.Max.X-line)/welcomeColumns, (gtx.Constraints.Max.Y-hint)/max(rows, 1))
	signing, ready := false, 0
	for i, c := range cells {
		at := image.Pt(i%welcomeColumns*(cell.X+line), i/welcomeColumns*cell.Y)
		st := op.Offset(at).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(cell)
		u.welcomeTome(cgtx, th, c, c.p.id == main)
		st.Pop()
		signing = signing || c.state == tomeSigningIn
		if c.state == tomeReady {
			ready++
		}
	}
	for col := 1; col < welcomeColumns; col++ {
		st := op.Offset(image.Pt(col*cell.X+(col-1)*line, 0)).Push(gtx.Ops)
		seam(gtx, th, image.Pt(line, rows*cell.Y))
		st.Pop()
	}
	for row := 1; row <= rows; row++ {
		st := op.Offset(image.Pt(0, row*cell.Y)).Push(gtx.Ops)
		seam(gtx, th, image.Pt(gtx.Constraints.Max.X, line))
		st.Pop()
	}
	st := op.Offset(image.Pt(0, rows*cell.Y+line)).Push(gtx.Ops)
	hgtx := gtx
	hgtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, hint-line))
	layout.Inset{Left: welcomeCellPadX, Right: welcomeCellPadX}.Layout(hgtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				if ready < 2 {
					return D{}
				}
				gtx.Constraints.Min.Y = 0
				return th.Text(welcomeStatusSize, i18n.T("shell.welcome.chooseHint")).Muted().Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if foot == nil {
					return D{}
				}
				gtx.Constraints.Min = image.Point{}
				return foot(gtx)
			}),
		)
	})
	st.Pop()
	return D{Size: gtx.Constraints.Max}, signing
}

// welcomeTome is one provider's cell: its book, its name, where it is and the
// one action that moves it on. A ready book that is not the default is the
// whole cell's click, which makes it the default.
func (u *window) welcomeTome(gtx C, th *ui.Theme, c welcomeCell, main bool) {
	t := u.welcome.tome(c.p.id)
	g, next, playing := t.frame(gtx.Now)
	shot, pinned := u.welcome.shot[c.p.id]
	if pinned && shot.journey != nil {
		g, playing = shot.grid(c.p.id), true
	}
	choosable := c.state == tomeReady && !main && !playing
	if choosable && t.row.Clicked(gtx) {
		u.chooseMain(c.p.id)
		// The default was read for this frame before the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	if t.action.Clicked(gtx) && !playing {
		u.welcomeAction(c.p, c.state)
	}
	switch {
	case playing && !pinned:
		gtx.Execute(op.InvalidateCmd{At: next})
	case playing:
	case c.state == tomeReady && main:
		g = welcomeFlame(gtx, th, c.p.id, pinned)
	case choosable && (t.row.Hovered() || pinned && shot.hover):
		g = grimoire.Hover(c.p.id)
	default:
		g = restSprite(c.p.id, c.state, c.facts.install.progress)
	}
	body := func(gtx C) D {
		if choosable && (t.row.Hovered() || pinned && shot.hover) {
			ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), 0)
		}
		return layout.Inset{Left: welcomeCellPadX, Right: welcomeCellPadX}.Layout(gtx, func(gtx C) D {
			// The cell is as tall as the window allows: the row centers in it.
			return layout.W.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = image.Point{X: gtx.Constraints.Max.X}
				return welcomeRow(gtx, th, g, func(gtx C) D { return u.welcomeLabels(gtx, th, c, t, main, playing) })
			})
		})
	}
	if choosable {
		t.row.Layout(gtx, body)
		return
	}
	body(gtx)
}

// welcomeRow is the book and its labels beside it.
func welcomeRow(gtx C, th *ui.Theme, g grimoire.Grid, labels layout.Widget) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return welcomeBook(gtx, th, g) }),
		ui.Gap(welcomeGapUnits),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			return labels(gtx)
		}),
	)
}

// welcomeFlame is the default's book at the flame's frame of now, asking for
// the next; a shot holds the first frame.
func welcomeFlame(gtx C, th *ui.Theme, id string, still bool) grimoire.Grid {
	if still {
		return grimoire.Flame(id, 0)
	}
	tick, next := flameTick(gtx.Now)
	if th.Still {
		next = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: next})
	return grimoire.Flame(id, tick)
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

// welcomeLabels are the name, the state, and under them the state's one
// action, or Default on the default's book.
func (u *window) welcomeLabels(gtx C, th *ui.Theme, c welcomeCell, t *tome, main, playing bool) D {
	children := []layout.FlexChild{
		layout.Rigid(th.Text(welcomeNameSize, c.p.name).Weight(font.SemiBold).Truncated().Layout),
		layout.Rigid(func(gtx C) D { return welcomeStatus(gtx, th, &u.root, t, c.state, c.facts) }),
	}
	verb := welcomeVerb(c.p, c.state)
	switch {
	case c.state == tomeReady && main:
		children = append(children, ui.VGap(welcomeActionGap),
			layout.Rigid(th.Text(ui.TextXS, i18n.T("shell.welcome.default")).Weight(font.SemiBold).Layout))
	case verb != "" && !playing:
		children = append(children, ui.VGap(welcomeActionGap), layout.Rigid(func(gtx C) D {
			b := th.Button(&t.action, verb)
			b.Variant, b.Size = ui.VariantSecondary, ui.SizeXS
			if c.state == tomeSignedOut && u.signInBlocked(c.p) {
				b.Disabled = true
				return th.Tooltip(&u.root, &t.tip, i18n.T("shell.welcome.signInNeedsProject")).Layout(gtx, b.Layout)
			}
			return b.Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
