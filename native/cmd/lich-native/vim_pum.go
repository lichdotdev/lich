package main

// Neovim's completion menu, drawn by lich as an IDE draws one: under the
// word being completed, its text in line with the word, a badge for each
// item's kind, its detail at the right, and the chosen item's documentation
// beside the menu. Neovim walks it (Ctrl-N, Ctrl-P, the arrows, Ctrl-Y,
// Ctrl-E); a click picks an item there.

import (
	"image"
	"strings"
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

// The menu's sizes: a row, its badge and the room around it, the menu's
// width and the documentation's.
const (
	vimPumRowH    = unit.Dp(22)
	vimPumBadge   = unit.Dp(16)
	vimPumRowPad  = unit.Dp(6)
	vimPumGap     = unit.Dp(8)
	vimPumMinW    = unit.Dp(240)
	vimPumMaxW    = unit.Dp(520)
	vimPumDocW    = unit.Dp(320)
	vimPumDocGap  = unit.Dp(6)
	vimPumDetailW = unit.Dp(24)
)

// vimPumDocLines is the most of an item's documentation the panel shows.
const vimPumDocLines = 14

// vimPumView is the menu's widgets: its list, a click per row, and the
// choice it last scrolled to.
type vimPumView struct {
	list     widget.List
	rows     []ui.Clickable
	scrolled int
	// motion grows the menu out of the word it completes.
	motion panelMotion
}

// vimKind is how a completion's kind is drawn: its badge's letter and the
// graph lane its color comes from.
type vimKind struct {
	letter string
	lane   int
}

// vimKinds are LSP's CompletionItemKind names, as Neovim's LSP completion
// spells them, and Vim's own one-letter kinds.
var vimKinds = map[string]vimKind{
	"Function": {"f", 3}, "Method": {"m", 3}, "Constructor": {"f", 3}, "f": {"f", 3},
	"Variable": {"v", 0}, "Field": {"v", 0}, "Property": {"p", 0}, "v": {"v", 0}, "m": {"v", 0},
	"Constant": {"c", 2}, "EnumMember": {"c", 2}, "Value": {"c", 2}, "d": {"c", 2},
	"Struct": {"S", 4}, "Class": {"C", 4}, "Interface": {"I", 4}, "Enum": {"E", 4},
	"TypeParameter": {"T", 4}, "t": {"T", 4},
	"Module": {"M", 1}, "File": {"F", 1}, "Folder": {"F", 1},
	"Keyword": {"k", 5}, "Snippet": {"s", 5}, "Operator": {"o", 5},
}

// vimKindOf is kind's badge, a muted dash for one it does not know.
func vimKindOf(kind string) (vimKind, bool) {
	k, ok := vimKinds[kind]
	return k, ok
}

// paintVimPum draws the completion menu under the word the cursor ends,
// above it when there is no room under, with the chosen item's
// documentation beside it; clicks pick in Neovim.
func (f *fileEditor) paintVimPum(gtx C, u vimUI) {
	p := u.pum
	shown := p != nil && len(p.items) > 0 && f.vim.state != nil
	f.vim.pum.motion.track(gtx.Now, shown, "")
	if !shown {
		return
	}
	st := f.ed.State()
	cursor := vimPos(st, f.vim.state.row, f.vim.state.col)
	start := vimWordStart(st, cursor)
	at, ok := f.ed.CoordsAtPos(start, 1)
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
	typed := st.SliceDoc(start, max(cursor, start))
	menu, size := f.vimPumMenu(gtx, p, typed)
	// The words stand in line with the word typed: the panel's padding, the
	// row's, the badge and the gap stand before them.
	lead := gtx.Dp(ui.Space(1)) + gtx.Dp(vimPumRowPad) + gtx.Dp(vimPumBadge) + gtx.Dp(vimPumGap)
	pos := image.Pt(max(min(at.Min.X-lead, gtx.Constraints.Max.X-size.X), 0), at.Max.Y)
	if pos.Y+size.Y > gtx.Constraints.Max.Y && at.Min.Y-size.Y >= 0 {
		pos.Y = at.Min.Y - size.Y
	}
	stack := op.Offset(pos).Push(gtx.Ops)
	from := image.Pt(min(lead, size.X), 0)
	if pos.Y < at.Max.Y {
		from.Y = size.Y
	}
	grow := f.vim.pum.motion.pushFrom(gtx, size, from)
	menu.Add(gtx.Ops)
	grow()
	stack.Pop()
	if p.selected >= 0 && p.selected < len(p.items) {
		f.paintVimPumDoc(gtx, p.items[p.selected], pos, size)
	}
}

// vimPumMenu records the menu, its rows at the code's size.
func (f *fileEditor) vimPumMenu(gtx C, p *vimPum, typed string) (op.CallOp, image.Point) {
	th, v := f.th, &f.vim.pum
	g := gtx
	pad := gtx.Dp(ui.Space(1))
	g.Constraints = layout.Constraints{Max: image.Pt(min(gtx.Dp(vimPumMaxW), gtx.Constraints.Max.X), vimPumRows*gtx.Dp(vimPumRowH)+2*pad)}
	g.Constraints.Min.X = min(max(gtx.Dp(vimPumMinW), f.vimPumWidth(gtx, p)), g.Constraints.Max.X)
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		return th.ThinScrollArea(&v.list).Layout(gtx, len(p.items), func(gtx C, i int) D {
			return f.vimPumRow(gtx, th, &v.rows[i], p.items[i], typed, i == p.selected)
		})
	})
	return macro.Stop(), d.Size
}

// vimPumWidth is how wide the rows want to be: the longest word and detail,
// in the code's cells, with the badge and the room around them.
func (f *fileEditor) vimPumWidth(gtx C, p *vimPum) int {
	words, details := 0, 0
	for _, it := range p.items {
		words = max(words, utf8.RuneCountInString(it.word))
		details = max(details, utf8.RuneCountInString(it.detail))
	}
	cell := f.ed.Geometry().CellWidth
	fixed := 2*gtx.Dp(ui.Space(1)) + 2*gtx.Dp(vimPumRowPad) + gtx.Dp(vimPumBadge) + gtx.Dp(vimPumGap)
	if details > 0 {
		fixed += gtx.Dp(vimPumDetailW)
	}
	return fixed + (words+details)*cell
}

// vimPumRow is one completion: its kind's badge, its word with what was
// typed of it lit, and its detail muted at the right.
func (f *fileEditor) vimPumRow(gtx C, th *ui.Theme, click *ui.Clickable, it vimPumItem, typed string, chosen bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if chosen || click.Hovered() {
			bg = th.Accent
		}
		th := th.On(bg)
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(vimPumRowH)))
		ui.Fill(gtx, bg, ui.RadiusSM)
		return layout.Inset{Left: vimPumRowPad, Right: vimPumRowPad}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return vimKindBadge(gtx, th, it.kind) }),
				layout.Rigid(layout.Spacer{Width: vimPumGap}.Layout),
				layout.Rigid(func(gtx C) D { return f.vimPumWord(gtx, th, it.word, typed) }),
				layout.Flexed(1, func(gtx C) D {
					if it.detail == "" {
						return D{Size: image.Pt(gtx.Constraints.Min.X, 0)}
					}
					return layout.Inset{Left: vimPumDetailW}.Layout(gtx, func(gtx C) D {
						return layout.E.Layout(gtx, f.vimCode(th, firstLine(it.detail), 0.9).Muted().Truncated().Layout)
					})
				}),
			)
		})
	})
}

// vimPumWord is the word, the part typed of it in the lane-0 color.
func (f *fileEditor) vimPumWord(gtx C, th *ui.Theme, word, typed string) D {
	n := 0
	if typed != "" && len(typed) <= len(word) && strings.EqualFold(word[:len(typed)], typed) {
		n = len(typed)
	}
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(f.vimCode(th, word[:n], 1).Weight(font.Bold).In(th.GraphLanes[0]).Layout),
		layout.Rigid(f.vimCode(th, word[n:], 1).Layout),
	)
}

// vimCode is text in the theme's mono face at scale of the code's size.
func (f *fileEditor) vimCode(th *ui.Theme, text string, scale float32) ui.LabelStyle {
	return th.Text(unit.Sp(float32(f.size)*scale), text).Mono()
}

// vimKindBadge is kind's letter on its lane's color, or a muted dot.
func vimKindBadge(gtx C, th *ui.Theme, kind string) D {
	side := gtx.Dp(vimPumBadge)
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	k, ok := vimKindOf(kind)
	if !ok {
		return layout.Center.Layout(gtx, th.Text(ui.TextXS, "·").Muted().Layout)
	}
	bg := th.GraphLanes[k.lane]
	ui.Fill(gtx, bg, ui.RadiusSM)
	return layout.Center.Layout(gtx, th.On(bg).Text(ui.TextXS, k.letter).Weight(font.Bold).In(th.Background).Layout)
}

// paintVimPumDoc draws it's documentation beside the menu at pos, size: at
// its right, at its left when there is no room, nowhere when neither has.
func (f *fileEditor) paintVimPumDoc(gtx C, it vimPumItem, pos, size image.Point) {
	doc := vimPumDocText(it)
	if doc == "" {
		return
	}
	w, gap := gtx.Dp(vimPumDocW), gtx.Dp(vimPumDocGap)
	x := pos.X + size.X + gap
	if x+w > gtx.Constraints.Max.X {
		x = pos.X - gap - w
	}
	if x < 0 {
		return
	}
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(w, max(gtx.Constraints.Max.Y-pos.Y, size.Y))}
	defer op.Offset(image.Pt(x, pos.Y)).Push(gtx.Ops).Pop()
	f.th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		return layout.UniformInset(ui.Space(1.5)).Layout(gtx, th.Text(ui.TextXS, doc).Wrapped().Layout)
	})
}

// vimPumDocText is an item's documentation as plain lines: its info, the
// Markdown fences an LSP wraps code in left out, cut to vimPumDocLines.
func vimPumDocText(it vimPumItem) string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(it.info), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			continue
		}
		lines = append(lines, l)
	}
	if len(lines) > vimPumDocLines {
		lines = append(lines[:vimPumDocLines], "…")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// firstLine is s up to its first line break.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
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
