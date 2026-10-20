package main

import (
	"slices"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/lichdotdev/lich/native/ui"
)

// blockState is one block's widget state, kept per project and block.
type blockState struct {
	toggle   widget.Clickable
	fold     ui.Collapse
	cards    ui.Sortable
	presence ui.Presence
	closing  closingCards
	launch   ui.MenuState
	options  ui.MenuState
	plus     widget.Clickable
	more     widget.Clickable
	rename   ui.InlineEdit
	drawn    bool // laid out this frame, else pruned
}

func (s *sidebar) blockState(b blockView) *blockState {
	key := foldKey(b.sessions[0].projectID, b.key)
	st, ok := s.blocks[key]
	if !ok {
		st = new(blockState)
		s.blocks[key] = st
	}
	st.drawn = true
	return st
}

// blockFrame is what every block of one frame reads.
type blockFrame struct {
	view      sidebarView
	active    string
	filtering bool
	// headerDragEnded is a block drag that ended this frame: its release lands
	// on the header it carried, which is not a click on its fold.
	headerDragEnded bool
	act             *sidebarAction
}

// blockList is the sidebar's scrolling list: the no-match Notice, the pinned
// block, fixed first, then every other block in one sortable list dragged by
// their headers.
func (s *sidebar) blockList(gtx C, th *ui.Theme, view sidebarView, active string, act *sidebarAction) D {
	f := blockFrame{view: view, active: active, filtering: s.header.filter().narrows(), act: act}
	var pinned *blockView
	var movable []blockView
	for i := range view.blocks {
		if view.blocks[i].kind == blockPinned {
			pinned = &view.blocks[i]
			continue
		}
		movable = append(movable, view.blocks[i])
	}
	keys := make([]string, len(movable))
	for i, b := range movable {
		keys[i] = b.key
	}
	s.sort.Disabled = f.filtering || !view.showHeader
	before := s.sort.Dragging()
	if d, ok := s.sort.Update(gtx); ok && d.ID != d.Over {
		act.blockOrder = arrayMove(keys, d.ID, d.Over)
	}
	f.headerDragEnded = before != "" && s.sort.Dragging() == ""
	for _, st := range s.blocks {
		st.drawn = false
	}
	var children []layout.FlexChild
	add := func(w layout.Widget) {
		if len(children) > 0 {
			children = append(children, ui.VGap(1.5))
		}
		children = append(children, layout.Rigid(w))
	}
	if view.notice != "" {
		add(func(gtx C) D { return noticeText(gtx, th, view.notice) })
	}
	if pinned != nil {
		add(func(gtx C) D { return s.block(gtx, th, *pinned, f) })
	}
	add(func(gtx C) D {
		return s.sort.List(gtx, ui.SortList{Gap: ui.Space(1.5), Within: true}, keys, func(gtx C, i int) D {
			return s.block(gtx, th, movable[i], f)
		})
	})
	d := s.scrollColumn(gtx, th).Layout(gtx, 1, func(gtx C, _ int) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
	s.pruneBlocks()
	return d
}

// scrollColumn is the block list's scroll area: an 8px thumb on a transparent
// track that takes room only once the list overflows, as a classic scrollbar
// does.
// Overflow is read off the previous frame's list.
func (s *sidebar) scrollColumn(gtx C, th *ui.Theme) material.ListStyle {
	list := th.ScrollArea(&s.list)
	list.Indicator.MinorWidth = ui.Space(2)
	list.Indicator.CornerRadius = ui.Space(1)
	list.Track.MinorPadding = 0
	list.AnchorStrategy = material.Overlay
	if s.list.Position.Length > gtx.Constraints.Max.Y {
		list.AnchorStrategy = material.Occupy
	}
	return list
}

func (s *sidebar) pruneBlocks() {
	for key, st := range s.blocks {
		if !st.drawn {
			delete(s.blocks, key)
		}
	}
}

// arrayMove is dnd-kit's arrayMove: id taken out and put back at over's slot.
func arrayMove(ids []string, id, over string) []string {
	from, to := slices.Index(ids, id), slices.Index(ids, over)
	if from < 0 || to < 0 {
		return slices.Clone(ids)
	}
	moved := slices.Delete(slices.Clone(ids), from, from+1)
	return slices.Insert(moved, to, id)
}

// block draws one block: the header, then the cards in the fold. A block
// dragged by its header wears the sidebar's fill, so it covers the blocks it
// passes over.
func (s *sidebar) block(gtx C, th *ui.Theme, b blockView, f blockFrame) D {
	st := s.blockState(b)
	dragEnded := s.dropCards(gtx, st, b, f)
	body := func(gtx C) D {
		var children []layout.FlexChild
		if f.view.showHeader {
			children = append(children, layout.Rigid(func(gtx C) D { return s.groupHeader(gtx, th, b, st, f) }), ui.VGap(1.5))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return st.fold.Layout(gtx, !b.folded, func(gtx C) D { return s.blockCards(gtx, th, b, st, f, dragEnded) })
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	if s.sort.Dragging() != b.key {
		return body(gtx)
	}
	return layout.Background{}.Layout(gtx, func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusLG) }, body)
}

// dropCards reads the block's card drag: a reorder inside it, or a card
// carried onto another block's header and filed by it. A pinned card is
// never carried out (filesByDrag), and nothing is dragged under a filter,
// whose drawn ids are not the ones an order splices back. It reports a drag
// that ended this frame, whose release is not a click on the card.
func (s *sidebar) dropCards(gtx C, st *blockState, b blockView, f blockFrame) bool {
	st.cards.Disabled = f.filtering
	st.cards.Carry = &s.carry
	if b.kind == blockPinned {
		st.cards.Carry = nil
	}
	before := st.cards.Dragging()
	d, ok := st.cards.Update(gtx)
	ended := before != "" && st.cards.Dragging() == ""
	if !ok {
		return ended
	}
	if !d.Carried {
		if d.ID != d.Over {
			f.act.cardOrder = blockReorder{key: b.key, ids: arrayMove(blockIDs(b), d.ID, d.Over)}
		}
		return ended
	}
	if i := slices.IndexFunc(f.view.blocks, func(t blockView) bool { return t.key == d.Over }); i >= 0 {
		f.act.file = fileRequest{ids: []string{d.ID}, folder: f.view.blocks[i].folder}
	}
	return ended
}

// blockCards is the block's card list, each card in its CardTransition: the
// gap between cards (pb-1.5) lives inside the motion, so a closing card takes
// it along, and the list cancels the one the last card trails (-mb-1.5).
func (s *sidebar) blockCards(gtx C, th *ui.Theme, b blockView, st *blockState, f blockFrame, dragEnded bool) D {
	shown, _ := st.closing.shown(b.sessions, gtx.Now)
	views := make(map[string]sessionView, len(shown))
	for _, sv := range shown {
		views[sv.ID] = sv
	}
	keys := st.presence.Update(gtx.Now, blockIDs(b))
	keys = slices.DeleteFunc(keys, func(id string) bool { _, ok := views[id]; return !ok })
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	// A card carried out of the sidebar goes back to its slot; the window
	// draws what follows the pointer over the panes (drawCarried).
	list := ui.SortList{Leaves: st.cards.Carry != nil}
	d := st.cards.List(gtx, list, keys, func(gtx C, i int) D {
		return s.blockCard(gtx, th, st, views[keys[i]], f, dragEnded)
	})
	if len(keys) > 0 {
		d.Size.Y = max(d.Size.Y-gtx.Dp(ui.Space(1.5)), 0)
	}
	return d
}

func (s *sidebar) blockCard(gtx C, th *ui.Theme, st *blockState, sv sessionView, f blockFrame, dragEnded bool) D {
	folders := slices.DeleteFunc(slices.Clone(f.view.folders), func(name string) bool { return name == sv.Folder })
	face := func(gtx C) D {
		d, a := s.card(gtx, th, &sv, sv.ID == f.active, folders)
		if dragEnded {
			a.activate = ""
		}
		f.act.takeCard(a)
		return d
	}
	return st.presence.Layout(gtx, sv.ID, func(gtx C) D {
		return layout.Inset{Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			if st.presence.Leaving(sv.ID) {
				return face(gtx)
			}
			return st.cards.Item(gtx, sv.ID, face, nil)
		})
	})
}
