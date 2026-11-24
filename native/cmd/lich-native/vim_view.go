package main

// What the editor draws of Neovim beyond the text: its cursor in the shape
// the mode's 'guicursor' gives, and a strip under the code with the command
// line, the last message, the mode's message and the partial command.

import (
	"image"
	"image/color"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
)

// vimBlockAlpha is how much of the cursor's color a block cursor lays over
// its character, which stays readable under it.
const vimBlockAlpha = 0.6

// vimMatchAlpha and vimCurrentMatchAlpha are how much of the wait tone a
// search match lays over its text, the one under the cursor more, as Vim's
// Search and CurSearch groups tell them apart.
const vimMatchAlpha, vimCurrentMatchAlpha = 0.3, 0.6

// vimCaretWidth is the command line's caret.
const vimCaretWidth = unit.Dp(1.5)

// vimCursorRect is where Neovim's cursor draws in the editor, shaped as the
// mode's cursor: the whole cell, its bottom or its left side.
func vimCursorRect(ed *godemirror.Editor, s vimState, c vimCursor) (image.Rectangle, bool) {
	st := ed.State()
	pos := vimPos(st, s.row, s.col)
	r, ok := ed.CoordsAtPos(pos, 1)
	if !ok {
		return image.Rectangle{}, false
	}
	w := ed.Geometry().CellWidth
	if line := st.LineAt(pos); pos < line.To {
		_, size := utf8.DecodeRuneInString(st.Doc()[pos:])
		next := pos + size
		if n, ok := ed.CoordsAtPos(next, -1); ok && n.Min.Y == r.Min.Y && n.Min.X > r.Min.X {
			w = n.Min.X - r.Min.X
		}
	}
	r.Max.X = r.Min.X + w
	share := func(n int) int { return max(n*min(max(c.percent, 1), 100)/100, 1) }
	switch c.shape {
	case "horizontal":
		r.Min.Y = r.Max.Y - share(r.Dy())
	case "vertical":
		r.Max.X = r.Min.X + share(w)
	}
	return r, true
}

// paintVimCursor draws the search's matches and Neovim's cursor over the
// code while the editor's buffer is the one Neovim shows.
func (f *fileEditor) paintVimCursor(gtx C) {
	if !f.vimOn() || f.vim.state == nil {
		return
	}
	buf, _, _ := f.vim.held()
	u, current := f.env.vim.status(buf)
	if !current {
		return
	}
	g := f.ed.Geometry()
	defer clip.Rect(image.Rect(g.GutterWidth, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)).Push(gtx.Ops).Pop()
	f.paintVimMatches(gtx, *f.vim.state)
	if u.cmdline != nil {
		// The command line has the caret; the matches of a search being
		// typed still show.
		return
	}
	c := u.cursor()
	r, ok := vimCursorRect(f.ed, *f.vim.state, c)
	if !ok {
		return
	}
	paint.FillShape(gtx.Ops, f.vimCursorColor(c), clip.Rect(r).Op())
}

// paintVimMatches lays the highlighted search's matches over their text.
func (f *fileEditor) paintVimMatches(gtx C, s vimState) {
	st := f.ed.State()
	cursor := vimPos(st, s.row, s.col)
	for _, m := range s.matches {
		r, ok := vimMatchRect(f.ed, vimPos(st, m[0], m[1]), m[2])
		if !ok {
			continue
		}
		alpha := float32(vimMatchAlpha)
		if from := vimPos(st, m[0], m[1]); cursor >= from && cursor < from+m[2] {
			alpha = vimCurrentMatchAlpha
		}
		paint.FillShape(gtx.Ops, ui.Alpha(f.th.ToneWait, alpha), clip.Rect(r).Op())
	}
}

// vimMatchRect is where a match of n bytes at from shows, false for one off
// screen or broken over a wrapped row.
func vimMatchRect(ed *godemirror.Editor, from, n int) (image.Rectangle, bool) {
	a, ok := ed.CoordsAtPos(from, 1)
	if !ok {
		return image.Rectangle{}, false
	}
	b, ok := ed.CoordsAtPos(from+n, -1)
	if !ok || b.Min.Y != a.Min.Y || b.Min.X <= a.Min.X {
		return image.Rectangle{}, false
	}
	return image.Rect(a.Min.X, a.Min.Y, b.Min.X, a.Max.Y), true
}

// vimCursorColor is the theme's caret color, a block's laid translucent over
// its character: the theme's, not the editor's, whose caret Vim mode hides.
func (f *fileEditor) vimCursorColor(c vimCursor) color.NRGBA {
	col := f.env.code.Cursor
	if c.shape == "block" || c.shape == "" {
		col = ui.Alpha(col, vimBlockAlpha)
	}
	return col
}

// vimStatus is the strip under the code while Vim runs the editor: the mode
// by name, whatever 'showmode' says, then the command line being typed, else
// the last message, else what 'showmode' adds to the mode (a recording), and
// the partial command at the right.
func (f *fileEditor) vimStatus(gtx C, th *ui.Theme) D {
	if !f.vimOn() {
		return D{}
	}
	buf, _, _ := f.vim.held()
	u, current := f.env.vim.status(buf)
	if !current {
		u = vimUI{}
	}
	left := func(gtx C) D {
		switch {
		case u.cmdline != nil:
			return vimCmdlineLayout(gtx, th, *u.cmdline)
		case u.msg != "":
			l := th.Text(ui.TextSM, u.msg).Mono().Truncated()
			if u.msgError {
				l = l.In(th.Destructive)
			}
			return l.Layout(gtx)
		}
		return th.Text(ui.TextSM, vimShowmodeRest(u.showmode)).Mono().Muted().Truncated().Layout(gtx)
	}
	mode, lane := "", 0
	if current && f.vim.state != nil {
		mode, lane = vimModeLabel(f.vim.state.mode)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return vimModeBadge(gtx, th, mode, th.GraphLanes[lane]) }),
					ui.Gap(3),
					layout.Flexed(1, left),
					layout.Rigid(th.Text(ui.TextSM, u.showcmd).Mono().Muted().Layout),
				)
			})
		}),
	)
}

// vimModeBadge is the mode's name in bold on its lane's color.
func vimModeBadge(gtx C, th *ui.Theme, mode string, bg color.NRGBA) D {
	if mode == "" {
		return D{}
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusSM) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx,
				th.On(bg).Text(ui.TextSM, mode).Mono().Weight(font.Bold).In(th.Background).Layout)
		})
}

// vimCmdlineLayout is the command line with its caret at pos.
func vimCmdlineLayout(gtx C, th *ui.Theme, c vimCmdline) D {
	pos := min(max(c.pos, 0), len(c.text))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, c.text[:pos]).Mono().Layout),
		layout.Rigid(func(gtx C) D {
			size := image.Pt(gtx.Dp(vimCaretWidth), gtx.Sp(ui.TextSM)+gtx.Dp(ui.Space(1)))
			paint.FillShape(gtx.Ops, th.Foreground, clip.Rect(image.Rectangle{Max: size}).Op())
			return D{Size: size}
		}),
		layout.Flexed(1, th.Text(ui.TextSM, c.text[pos:]).Mono().Truncated().Layout),
	)
}

// vimHideCaret is the editor's theme with its own caret out of sight, the
// cursor being Neovim's.
func vimHideCaret(t godemirror.Theme) godemirror.Theme {
	t.Cursor = color.NRGBA{}
	return t
}
