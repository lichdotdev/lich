package main

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
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

// Header tints of SessionGroupHeader: the title's text-muted-foreground/70,
// a refusing header's opacity-45 and the drop target's bg-tone-pass/15.
const (
	sideTitleOpacity  = 0.7
	sideRefuseOpacity = 0.45
	sideDropFill      = 0.15
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
		caption(th, "SessionSidebar: drag a card (onto Reviews files it), a header, or the right edge"),
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return s.header(gtx, th, b) }),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			return b.fold.Layout(gtx, b.open, func(gtx C) D {
				return b.cardSort.List(gtx, ui.SortList{}, shown, func(gtx C, i int) D { return s.card(gtx, th, b, shown[i]) })
			})
		}),
	)
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

// header is SessionGroupHeader: the title button is the block's drag handle,
// and a folder takes a carried card from any other block.
func (s *sidebar) header(gtx C, th *ui.Theme, b *sideBlock) D {
	carried := s.carry.Carrying()
	takes := b.folder && carried != "" && !slices.Contains(b.cards, carried)
	refuses := carried != "" && !takes
	over := takes && s.carry.Over() == b.id
	row := func(gtx C) D {
		bg := color.NRGBA{}
		if over {
			bg = th.Over(ui.Alpha(th.TonePass, sideDropFill))
		}
		m := op.Record(gtx.Ops)
		d := s.headerRow(gtx, th, b, refuses)
		call := m.Stop()
		if bg.A != 0 {
			gtx.Constraints.Min = d.Size
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		call.Add(gtx.Ops)
		return d
	}
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
	chevron := "chevron-right"
	if b.open {
		chevron = "chevron-down"
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return b.toggle.Layout(gtx, func(gtx C) D {
				return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1), Left: ui.Space(1)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return s.icon(chevron).Layout(gtx, ui.Space(3), fg) }),
						ui.Gap(1.5),
						layout.Rigid(th.Text(ui.TextXS, strings.ToUpper(b.name)).In(fg).Weight(font.SemiBold).Layout),
						ui.Gap(1.5),
						layout.Flexed(1, func(gtx C) D { return th.Separator(gtx) }),
					)
				})
			})
		}),
		layout.Rigid(func(gtx C) D {
			btn := th.Button(&b.add, "")
			btn.Icon, btn.Variant, btn.Size = s.icon("plus"), ui.VariantGhost, ui.SizeIconXS
			return btn.Layout(gtx)
		}),
	)
}

// card is a session card stand-in inside a Presence: the gap between
// cards (pb-1.5) lives inside the motion, so a closing card takes it along.
func (s *sidebar) card(gtx C, th *ui.Theme, b *sideBlock, id string) D {
	closer := s.closers[id]
	if closer == nil {
		closer = new(widget.Clickable)
		s.closers[id] = closer
	}
	if closer.Clicked(gtx) {
		b.cards = slices.DeleteFunc(b.cards, func(c string) bool { return c == id })
	}
	box := func(gtx C) D { return s.cardBox(gtx, th, closer, id, b.cardSort.Dragging() == id) }
	return b.presence.Layout(gtx, id, func(gtx C) D {
		return layout.Inset{Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			if b.presence.Leaving(id) {
				return box(gtx)
			}
			return b.cardSort.Item(gtx, id, box, nil)
		})
	})
}

func (s *sidebar) cardBox(gtx C, th *ui.Theme, closer *widget.Clickable, id string, lifted bool) D {
	bg := th.Over(ui.Alpha(th.Accent, sideCardFill))
	if lifted {
		bg = th.Accent
	}
	on := th.On(bg)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusLG) },
		func(gtx C) D {
			return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(2.5), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, on.Text(ui.TextSM, id).Layout),
					layout.Rigid(func(gtx C) D {
						btn := on.Button(closer, "")
						btn.Icon, btn.Variant, btn.Size = s.icon("x"), ui.VariantGhost, ui.SizeIconXS
						return btn.Layout(gtx)
					}),
				)
			})
		},
	)
}

// sideCardFill is the resting card's bg-accent/40 stand-in.
const sideCardFill = 0.4
