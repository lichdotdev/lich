package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// Sidebar widths: the bounds of the panel's drag, in dp.
const (
	sideMinWidth  unit.Dp = 200
	sideMaxWidth  unit.Dp = 380
	sideStartWide unit.Dp = 280
	sideHeight    unit.Dp = 420
)

// The tree's measures and tints: a row's height, a node's ring and the
// indent a session hangs at; the header title's text-muted-foreground/70, a
// refusing header's opacity-45, the links' and a neutral ring's muted
// foreground, and a folder's tray (DESIGN.md › the sidebar: a tree of rows).
const (
	sideRow    unit.Dp = 34
	sideRingR  unit.Dp = 11
	sideIndent unit.Dp = 16
	sideInset  unit.Dp = 6
	sideElbow  unit.Dp = 5
	sideStroke unit.Dp = 1.25

	sideTitleOpacity  = 0.7
	sideRefuseOpacity = 0.45
	sideLinkAlpha     = 0.45
	sideRingAlpha     = 0.35
	sideTrayAlpha     = 0.45
	sideHoverFill     = 0.5
	sideDropBorder    = unit.Dp(1.5)
)

var sidePhases = []string{"Waiting", "Running", "Unread", "Idle"}

// sideBlock is one SessionGroup: a checkout's cards, or a folder's.
type sideBlock struct {
	id, name string
	folder   bool
	cards    []string
	open     bool
	toggle   widget.Clickable
	add      widget.Clickable
	fold     ui.Collapse
	cardSort ui.Sortable
	presence ui.Presence
}

// sidebar is the gallery's session sidebar: its state and its groups'.
type sidebar struct {
	blocks  []*sideBlock
	sort    ui.Sortable
	carry   ui.SortCarry
	width   unit.Dp
	resize  ui.ResizeHandle
	chips   [5]widget.Clickable
	picked  map[int]bool
	closers map[string]*widget.Clickable
	made    int
	icon    func(string) *icons.Icon
}

func newSidebar(icon func(string) *icons.Icon) *sidebar {
	s := &sidebar{width: sideStartWide, picked: map[int]bool{1: true}, closers: map[string]*widget.Clickable{}, icon: icon}
	s.blocks = []*sideBlock{
		{id: "root", name: "lich", cards: []string{"Claude · native ui", "Codex · tests"}, open: true},
		{id: "wt", name: "feature/toolbar", cards: []string{"Claude · footer", "Shell"}, open: true},
		{id: "reviews", name: "Reviews", folder: true, cards: []string{"opencode · PR #123"}, open: false},
	}
	for _, b := range s.blocks {
		b.cardSort.Carry = &s.carry
	}
	return s
}

func (g *gallery) sidebarSection(gtx C, th *ui.Theme) D {
	s := g.side
	return stack(3,
		caption(th, "SessionPhaseChips: Chip with a count; empty and off dims"),
		s.phaseChips(th),
		caption(th, "SessionSidebar: a tree of rows; drag a card (onto another block files it), a header, or the right edge; … adds a session"),
		func(gtx C) D { return s.panel(gtx, th) },
	)(gtx)
}

// phaseChips is SessionPhaseChips: All is on while no phase is.
func (s *sidebar) phaseChips(th *ui.Theme) layout.Widget {
	counts := []int{1, 3, 0, 1}
	return func(gtx C) D {
		if s.chips[0].Clicked(gtx) {
			clear(s.picked)
		}
		for i := range sidePhases {
			if !s.chips[i+1].Clicked(gtx) {
				continue
			}
			if s.picked[i] {
				delete(s.picked, i)
			} else {
				s.picked[i] = true
			}
		}
		kids := []ui.WrapChild{ui.WrapItem(th.Chip(&s.chips[0], "All", 5, len(s.picked) == 0).Layout)}
		for i, label := range sidePhases {
			kids = append(kids, ui.WrapItem(th.Chip(&s.chips[i+1], label, counts[i], s.picked[i]).Layout))
		}
		return ui.Wrap{GapX: ui.Space(0.5), GapY: ui.Space(0.5)}.Layout(gtx, kids...)
	}
}

// panel is the aside: bg-sidebar, the blocks in one sortable list, the
// resize handle on its right edge.
func (s *sidebar) panel(gtx C, th *ui.Theme) D {
	if w, _ := s.resize.Update(gtx, s.width, sideMinWidth, sideMaxWidth); w != s.width {
		s.width = w
	}
	if d, ok := s.sort.Update(gtx); ok && d.ID != d.Over {
		s.moveBlock(d)
	}
	on := th.On(th.Sidebar)
	size := image.Pt(min(gtx.Dp(s.width), gtx.Constraints.Max.X), gtx.Dp(sideHeight))
	gtx.Constraints = layout.Exact(size)
	ui.Fill(gtx, th.Sidebar, ui.RadiusLG)
	layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = 0
		ids := make([]string, len(s.blocks))
		for i, b := range s.blocks {
			ids[i] = b.id
		}
		return s.sort.List(gtx, ui.SortList{Gap: ui.Space(1.5), Within: true}, ids, func(gtx C, i int) D {
			return s.block(gtx, on, s.blocks[i])
		})
	})
	gtx.Constraints = layout.Exact(size)
	s.resize.Layout(gtx, on)
	return D{Size: size}
}

func (s *sidebar) moveBlock(d ui.SortDrop) {
	from := slices.IndexFunc(s.blocks, func(b *sideBlock) bool { return b.id == d.ID })
	to := slices.IndexFunc(s.blocks, func(b *sideBlock) bool { return b.id == d.Over })
	b := s.blocks[from]
	s.blocks = slices.Insert(slices.Delete(s.blocks, from, from+1), to, b)
}

// block is SessionGroup: the header, then the cards in a Collapse.
func (s *sidebar) block(gtx C, th *ui.Theme, b *sideBlock) D {
	if b.toggle.Clicked(gtx) {
		b.open = !b.open
	}
	if b.add.Clicked(gtx) {
		s.made++
		b.cards = append(b.cards, fmt.Sprintf("Claude · new %d", s.made))
	}
	if d, ok := b.cardSort.Update(gtx); ok {
		s.dropCard(b, d)
	}
	shown := b.presence.Update(gtx.Now, b.cards)
	body := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return s.header(gtx, th, b) }),
			layout.Rigid(func(gtx C) D {
				return b.fold.Layout(gtx, b.open, func(gtx C) D {
					return b.cardSort.List(gtx, ui.SortList{Leaves: true}, shown, func(gtx C, i int) D {
						return s.card(gtx, th, b, shown[i], i == len(shown)-1)
					})
				})
			}),
		)
	}
	// A folder sits on a tray; the block a carried card would go to lights
	// its whole tray, a border round it.
	switch {
	case s.carry.Hovers(b.id):
		return sideTray(gtx, th.Over(th.Accent), th.MutedForeground, body)
	case b.folder:
		return sideTray(gtx, th.Over(ui.Alpha(th.Accent, sideTrayAlpha)), color.NRGBA{}, body)
	}
	return body(gtx)
}

// sideTray lays w on a tray of fill, a border of edge round it when edge is
// set.
func sideTray(gtx C, fill, edge color.NRGBA, w layout.Widget) D {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	rr := gtx.Dp(ui.RadiusLG)
	shape := clip.UniformRRect(image.Rectangle{Max: d.Size}, rr)
	paint.FillShape(gtx.Ops, fill, shape.Op(gtx.Ops))
	call.Add(gtx.Ops)
	if edge.A != 0 {
		width := float32(gtx.Dp(sideDropBorder))
		in := int(width / 2)
		box := image.Rectangle{Min: image.Pt(in, in), Max: d.Size.Sub(image.Pt(in, in))}
		paint.FillShape(gtx.Ops, edge, clip.Stroke{Path: clip.UniformRRect(box, rr-in).Path(gtx.Ops), Width: width}.Op())
	}
	return d
}

// dropCard reorders inside the block, or files the card under the folder it
// was carried to.
func (s *sidebar) dropCard(b *sideBlock, d ui.SortDrop) {
	if !d.Carried {
		from, to := slices.Index(b.cards, d.ID), slices.Index(b.cards, d.Over)
		if from >= 0 && to >= 0 && from != to {
			b.cards = slices.Insert(slices.Delete(b.cards, from, from+1), to, d.ID)
		}
		return
	}
	for _, into := range s.blocks {
		if into.id == d.Over {
			b.cards = slices.DeleteFunc(b.cards, func(c string) bool { return c == d.ID })
			into.cards = append(into.cards, d.ID)
		}
	}
}

// header is a block's first row, its drag handle: a checkout's node and
// branch, or a folder's glyph, title and count over a rule. A block takes a
// card carried from another one.
func (s *sidebar) header(gtx C, th *ui.Theme, b *sideBlock) D {
	carried := s.carry.Carrying()
	takes := carried != "" && !slices.Contains(b.cards, carried)
	row := func(gtx C) D { return s.headerRow(gtx, th, b, carried != "" && !takes) }
	handle := func(gtx C) D { return s.sort.Item(gtx, b.id, row, nil) }
	if takes {
		return s.carry.Target(gtx, b.id, handle)
	}
	return handle(gtx)
}

func (s *sidebar) headerRow(gtx C, th *ui.Theme, b *sideBlock, refuses bool) D {
	fg := th.Over(ui.Alpha(th.MutedForeground, sideTitleOpacity))
	if refuses {
		fg = th.Over(ui.Alpha(fg, sideRefuseOpacity))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = gtx.Dp(sideRow)
	title := func(gtx C) D {
		if !b.folder {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return s.node(gtx, th, "git-branch", ui.Alpha(fg, 1)) }),
				ui.Gap(1.5),
				layout.Flexed(1, th.Text(ui.TextSM, b.name).In(fg).Weight(font.Medium).Layout),
			)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(2*sideRingR), gtx.Dp(ui.Space(3))))
				return layout.Center.Layout(gtx, func(gtx C) D { return s.icon("folder").Layout(gtx, ui.Space(3), fg) })
			}),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, strings.ToUpper(b.name)).In(fg).Weight(font.SemiBold).Layout),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(len(b.cards))).In(fg).Layout),
			ui.Gap(1.5),
			layout.Flexed(1, func(gtx C) D { return th.Separator(gtx) }),
		)
	}
	return b.toggle.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: sideInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, title),
				layout.Rigid(func(gtx C) D {
					btn := th.Button(&b.add, "")
					btn.Icon, btn.Variant, btn.Size = s.icon("ellipsis"), ui.VariantGhost, ui.SizeIconXS
					return btn.Layout(gtx)
				}),
			)
		})
	})
}

// node is a tree node: glyph in a neutral ring.
func (s *sidebar) node(gtx C, th *ui.Theme, glyph string, fg color.NRGBA) D {
	side := gtx.Dp(2 * sideRingR)
	stroke := float32(gtx.Dp(sideStroke))
	c := float32(side) / 2
	ui.StrokeArc(gtx, f32.Pt(c, c), c-stroke/2, stroke, 0, 2*math.Pi, th.Over(ui.Alpha(th.MutedForeground, sideRingAlpha)))
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	return layout.Center.Layout(gtx, func(gtx C) D { return s.icon(glyph).Layout(gtx, unit.Dp(13), fg) })
}

// card is a session row inside a Presence: in a checkout it hangs from the
// checkout's line by an elbow, the last one ending it; the row lights under
// the pointer and in the air, and a close sits in its corner.
func (s *sidebar) card(gtx C, th *ui.Theme, b *sideBlock, id string, last bool) D {
	closer := s.closers[id]
	if closer == nil {
		closer = new(widget.Clickable)
		s.closers[id] = closer
	}
	if closer.Clicked(gtx) {
		b.cards = slices.DeleteFunc(b.cards, func(c string) bool { return c == id })
	}
	lifted := b.cardSort.Dragging() == id
	box := func(gtx C) D { return s.cardRow(gtx, th, closer, id, !b.folder && !lifted, last, lifted) }
	return b.presence.Layout(gtx, id, func(gtx C) D {
		if b.presence.Leaving(id) {
			return box(gtx)
		}
		return b.cardSort.Item(gtx, id, box, nil)
	})
}

func (s *sidebar) cardRow(gtx C, th *ui.Theme, closer *widget.Clickable, id string, hangs, last, lifted bool) D {
	bg := color.NRGBA{}
	if lifted || closer.Hovered() {
		bg = th.Over(ui.Alpha(th.Accent, sideHoverFill))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = gtx.Dp(sideRow)
	indent := unit.Dp(0)
	if hangs {
		indent = sideIndent
	}
	d := layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: sideInset + indent, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return s.node(gtx, th, "terminal", th.MutedForeground) }),
					ui.Gap(1.5),
					layout.Flexed(1, th.Text(ui.TextSM, id).Weight(font.Medium).Layout),
					layout.Rigid(func(gtx C) D {
						btn := th.Button(closer, "")
						btn.Icon, btn.Variant, btn.Size = s.icon("x"), ui.VariantGhost, ui.SizeIconXS
						return btn.Layout(gtx)
					}),
				)
			})
		},
	)
	if hangs {
		sideElbowLink(gtx, th, d.Size, last)
	}
	return d
}

// sideElbowLink is a row's link to its checkout's ring: down the checkout's
// column, round the elbow to the row's ring, and on down past it unless the
// row is the last.
func sideElbowLink(gtx C, th *ui.Theme, size image.Point, last bool) {
	px := float32(gtx.Dp(sideInset) + gtx.Dp(sideRingR))
	ring := float32(gtx.Dp(sideInset) + gtx.Dp(sideIndent))
	mid, elbow := float32(size.Y)/2, float32(gtx.Dp(sideElbow))
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(px, 0))
	p.LineTo(f32.Pt(px, mid-elbow))
	p.QuadTo(f32.Pt(px, mid), f32.Pt(px+elbow, mid))
	p.LineTo(f32.Pt(ring, mid))
	if !last {
		p.MoveTo(f32.Pt(px, mid-elbow))
		p.LineTo(f32.Pt(px, float32(size.Y)))
	}
	c := th.Over(ui.Alpha(th.MutedForeground, sideLinkAlpha))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(sideStroke))}.Op())
}
