package main

import (
	"image/color"
	"slices"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/lichdotdev/lich/native/ui"
)

// blockState is one block's widget state, kept per project and block.
type blockState struct {
	toggle   widget.Clickable
	header   ui.Collapse // shown while the list splits into blocks
	fold     ui.Collapse
	cards    ui.Sortable
	presence ui.Presence
	tree     treeMotion
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
// their headers. Each block opens and closes in its own motion, which holds
// the gap below it, so a block closing takes its gap along.
func (s *sidebar) blockList(gtx C, th *ui.Theme, view sidebarView, active string, act *sidebarAction) D {
	f := blockFrame{view: view, active: active, filtering: s.header.filter().narrows(), act: act}
	var pinned *blockView
	var movable []blockView
	shown := s.motion.shown(gtx.Now, view.project, view.blocks)
	for i := range shown {
		if shown[i].kind == blockPinned {
			pinned = &shown[i]
			continue
		}
		movable = append(movable, shown[i])
	}
	keys := make([]string, len(movable))
	for i, b := range movable {
		keys[i] = b.key
	}
	f.headerDragEnded = s.dropBlocks(gtx, f, keys)
	for _, st := range s.blocks {
		st.drawn = false
	}
	item := func(gtx C, b blockView) D {
		return s.motion.motion.Layout(gtx, b.key, func(gtx C) D {
			return layout.Inset{Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D { return s.block(gtx, th, b, f) })
		})
	}
	var children []layout.FlexChild
	if view.notice != "" {
		children = append(children, layout.Rigid(func(gtx C) D { return noticeText(gtx, th, view.notice) }), ui.VGap(1.5))
	}
	if pinned != nil {
		children = append(children, layout.Rigid(func(gtx C) D { return item(gtx, *pinned) }))
	}
	children = append(children, layout.Rigid(func(gtx C) D {
		return s.sort.List(gtx, ui.SortList{Within: true}, keys, func(gtx C, i int) D { return item(gtx, movable[i]) })
	}))
	d := s.scrollColumn(gtx, th).Layout(gtx, 1, func(gtx C, _ int) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			d := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			if len(shown) > 0 {
				d.Size.Y = max(d.Size.Y-gtx.Dp(ui.Space(1.5)), 0)
			}
			return d
		})
	})
	s.pruneBlocks()
	return d
}

// dropBlocks reads the block drag, a reorder by their headers, which only
// the live blocks take part in. It reports a drag that ended this frame.
func (s *sidebar) dropBlocks(gtx C, f blockFrame, keys []string) bool {
	s.sort.Disabled = f.filtering || !f.view.showHeader
	before := s.sort.Dragging()
	if d, ok := s.sort.Update(gtx); ok && d.ID != d.Over && !s.motion.motion.Leaving(d.Over) {
		f.act.blockOrder = arrayMove(liveKeys(keys, &s.motion.motion), d.ID, d.Over)
	}
	return before != "" && s.sort.Dragging() == ""
}

// liveKeys is keys without the blocks drawn only for their exit, what a drag
// commits.
func liveKeys(keys []string, motion *ui.Presence) []string {
	return slices.DeleteFunc(slices.Clone(keys), motion.Leaving)
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

// block draws one block: the header, which folds in and out as the list
// starts and stops splitting into blocks, then the cards in the fold. A block
// dragged by its header wears the sidebar's fill, so it covers the blocks it
// passes over.
func (s *sidebar) block(gtx C, th *ui.Theme, b blockView, f blockFrame) D {
	st := s.blockState(b)
	dragEnded := s.dropCards(gtx, st, b, f)
	body := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return st.header.Layout(gtx, f.view.showHeader, func(gtx C) D {
					// A checkout's header is its tree's root row, spaced like a card.
					gap := ui.Space(1.5)
					if b.kind == blockCheckout {
						gap = cardGap
					}
					return layout.Inset{Bottom: gap}.Layout(gtx, func(gtx C) D { return s.groupHeader(gtx, th, b, st, f) })
				})
			}),
			layout.Rigid(func(gtx C) D {
				return st.fold.Layout(gtx, !b.folded, func(gtx C) D { return s.blockCards(gtx, th, b, st, f, dragEnded) })
			}),
		)
	}
	// A closed folder is its header alone, with nothing to set apart.
	if b.kind == blockFolder && !b.collapsed {
		inner := body
		body = func(gtx C) D { return folderTray(gtx, th, b.color, inner) }
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

// cardGap is the gap under every card of a block: none, so the tree's links
// run from row to row unbroken, each row clipped to its own box.
var cardGap = unit.Dp(0)

// blockCards is the block's rows, each in its CardTransition: the gap between
// rows lives inside the motion, so a closing card takes it along, and the list
// cancels the one the last row trails. The rows are the block's sessions as
// trees (sessionRows), drawn from the shown sessions so a closing card keeps
// its place while it leaves.
func (s *sidebar) blockCards(gtx C, th *ui.Theme, b blockView, st *blockState, f blockFrame, dragEnded bool) D {
	shown, _ := st.closing.shown(b.sessions, gtx.Now)
	live := blockRows(b, b.sessions)
	open := make(map[string]bool, len(b.sessions))
	for _, sv := range b.sessions {
		open[sv.ID] = true
	}
	if st.tree.update(gtx.Now, live, open) {
		gtx.Execute(op.InvalidateCmd{})
	}
	rows := st.tree.place(gtx.Now, blockRows(b, shown))
	byKey := make(map[string]sessionRow, len(rows))
	tree := b.under != "" || b.byCheckout
	for _, r := range rows {
		byKey[r.key] = r
		tree = tree || r.depth > 0 || r.kids > 0
	}
	order := make([]string, len(live))
	for i, r := range live {
		order[i] = r.key
	}
	drawn := map[string]bool{}
	for _, key := range st.presence.Update(gtx.Now, order) {
		drawn[key] = true
	}
	var keys []string
	for _, r := range rows {
		if drawn[r.key] || r.motion != nil {
			keys = append(keys, r.key)
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	// A card carried out of the sidebar goes back to its slot; the window
	// draws what follows the pointer over the panes (drawCarried).
	list := ui.SortList{Leaves: st.cards.Carry != nil}
	d := st.cards.List(gtx, list, keys, func(gtx C, i int) D {
		return s.blockRow(gtx, th, st, b, byKey[keys[i]], tree, f, dragEnded)
	})
	if len(keys) > 0 {
		d.Size.Y = max(d.Size.Y-gtx.Dp(cardGap), 0)
	}
	return d
}

// blockRow is one row in its motion: a checkout a mother's children run in,
// or a card. Every card drags, but a reorder only moves it among the stored
// order: the tree is read off the origins, so a child stays under its mother.
func (s *sidebar) blockRow(gtx C, th *ui.Theme, st *blockState, b blockView, r sessionRow, tree bool, f blockFrame, dragEnded bool) D {
	if r.kind == rowNestedCheckout {
		row := func(gtx C) D {
			return layout.Inset{Bottom: cardGap}.Layout(gtx, func(gtx C) D { return s.nestedCheckout(gtx, th, r, b.facts[r.path], tree) })
		}
		if r.motion != nil {
			return moveRow(gtx, r, row)
		}
		return st.presence.Layout(gtx, r.key, row)
	}
	sv := r.sv
	sv.place = r.place(tree)
	sv.place.onTray = b.kind == blockFolder && b.color != "" && sv.Color == b.color
	folders := slices.DeleteFunc(slices.Clone(f.view.folders), func(name string) bool { return name == sv.Folder })
	face := func(gtx C) D {
		d, a := s.card(gtx, th, &sv, sv.ID == f.active, folders)
		if dragEnded {
			a.activate = ""
		}
		f.act.takeCard(a)
		return d
	}
	if r.motion != nil {
		return moveRow(gtx, r, func(gtx C) D { return layout.Inset{Bottom: cardGap}.Layout(gtx, face) })
	}
	return st.presence.Layout(gtx, sv.ID, func(gtx C) D {
		return layout.Inset{Bottom: cardGap}.Layout(gtx, func(gtx C) D {
			if st.presence.Leaving(sv.ID) {
				return face(gtx)
			}
			return st.cards.Item(gtx, sv.ID, face, nil)
		})
	})
}

// folderTrayAlpha is how strongly the accent tints a folder's tray.
const folderTrayAlpha = 0.45

// folderTray lays a folder's header and rows on a tray, so where the folder
// ends shows against the checkouts that follow it. A folder painted a color
// wears it on the whole tray, as its cards wore it at rest.
func folderTray(gtx C, th *ui.Theme, tint string, w layout.Widget) D {
	tray := th.Over(ui.Alpha(th.Accent, folderTrayAlpha))
	if c, ok := cardColors[tint]; ok {
		tray = th.Over(ui.MixOKLab(color.NRGBA{}, c, tintRest))
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, tray, ui.RadiusLG) },
		func(gtx C) D { return layout.Inset{Bottom: ui.Space(0.5)}.Layout(gtx, w) },
	)
}
