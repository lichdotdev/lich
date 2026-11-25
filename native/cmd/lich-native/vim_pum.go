package main

// Neovim's completion menu, drawn by lich: its items on the popover surface
// under the word being completed, the chosen one lit. Neovim walks it
// (Ctrl-N, Ctrl-P, the arrows, Ctrl-Y, Ctrl-E); a click picks an item there.

import (
	"image"
	"unicode"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
)

// vimPumRows is how many rows the menu shows before it scrolls.
const vimPumRows = 10

// vimPumWidth bounds the menu's width.
const vimPumMinW, vimPumMaxW = unit.Dp(220), unit.Dp(480)

// vimPumView is the menu's widgets: its list, a click per row, and the
// choice it last scrolled to.
type vimPumView struct {
	list     widget.List
	rows     []ui.Clickable
	scrolled int
}

// paintVimPum draws the completion menu under the word the cursor ends,
// above it when there is no room under; clicks pick in Neovim.
func (f *fileEditor) paintVimPum(gtx C, u vimUI) {
	p := u.pum
	if p == nil || len(p.items) == 0 || f.vim.state == nil {
		return
	}
	at, ok := f.ed.CoordsAtPos(vimWordStart(f.ed.State(), vimPos(f.ed.State(), f.vim.state.row, f.vim.state.col)), 1)
	if !ok {
		return
	}
	v := &f.vim.pum
	if len(v.rows) < len(p.items) {
		v.rows = make([]ui.Clickable, len(p.items))
	}
	for i := range p.items {
		if v.rows[i].Clicked(gtx) {
			f.vimFeed("lich_editor.pick(...)", i)
		}
	}
	v.list.Axis = layout.Vertical
	if p.selected >= 0 && p.selected != v.scrolled {
		v.scrolled = p.selected
		v.list.ScrollTo(p.selected)
	}
	th := f.th
	g := gtx
	// The rows and the panel's padding, ui.Space(1) on each side.
	g.Constraints = layout.Constraints{Max: image.Pt(min(gtx.Dp(vimPumMaxW), gtx.Constraints.Max.X), vimPumRows*gtx.Dp(pickerRowH)+2*gtx.Dp(ui.Space(1)))}
	g.Constraints.Min.X = min(gtx.Dp(vimPumMinW), g.Constraints.Max.X)
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		return th.ThinScrollArea(&v.list).Layout(gtx, len(p.items), func(gtx C, i int) D {
			return pickerRow(gtx, th, &v.rows[i], i == p.selected, func(gtx C, th *ui.Theme) D {
				return vimPumRow(gtx, th, p.items[i])
			})
		})
	})
	call := macro.Stop()
	pos := image.Pt(min(at.Min.X, max(gtx.Constraints.Max.X-d.Size.X, 0)), at.Max.Y)
	if pos.Y+d.Size.Y > gtx.Constraints.Max.Y && at.Min.Y-d.Size.Y >= 0 {
		pos.Y = at.Min.Y - d.Size.Y
	}
	defer op.Offset(pos).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// vimPumRow is one completion: its word, and its kind muted at the right.
func vimPumRow(gtx C, th *ui.Theme, it vimPumItem) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, th.Text(ui.TextSM, it.word).Mono().Weight(font.Medium).Truncated().Layout),
		ui.Gap(3),
		layout.Rigid(th.Text(ui.TextXS, it.kind).Mono().Muted().Layout),
	)
}

// vimWordStart is where the word that ends at pos starts, the place the
// completion stands under.
func vimWordStart(st godemirror.EditorState, pos int) int {
	doc, from := st.Doc(), st.LineAt(pos).From
	for pos > from {
		r, size := utf8.DecodeLastRuneInString(doc[:pos])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		pos -= size
	}
	return pos
}
