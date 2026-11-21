package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A checkout's editor is a node of the tree, the last row under the
// checkout for as long as the editor holds a file: a click puts it on the
// stage, and its menu is the only way to close it. It is no session, so it
// files nowhere and a drag leaves it where it was.

var icEditorFile = icons.Lucide("file")

// editorDirtyDot is the unsaved mark beside the file's name, the size the
// editor's header draws it.
const editorDirtyDot = unit.Dp(6)

// editorMenu is the node's menu: Close, the one way to take the editor away.
func editorMenu(checkout string, act *sidebarAction) []ui.MenuEntry {
	return []ui.MenuEntry{{Label: i18n.T("sidebar.editorRow.close"), Variant: ui.VariantDestructive, OnClick: func() { act.editorClose = checkout }}}
}

// editorTooltip is what the row's line cuts: the file's path in the
// checkout, and that it holds changes not saved.
func editorTooltip(th *ui.Theme, n editorNode) layout.Widget {
	th = th.On(th.Popover)
	sections := []layout.Widget{para(th.Text(ui.TextXS, n.rel).Mono()).Layout}
	if n.dirty {
		sections = append(sections, th.Text(ui.TextXS, i18n.T("sidebar.editorRow.unsaved")).Muted().Layout)
	}
	return func(gtx C) D { return ui.Stack(1.5, sections...)(gtx) }
}

// editorRow is a checkout's editor in the tree: the file glyph in a neutral
// ring and the file's name in mono, lit while the editor is on the stage,
// with the unsaved dot after the name.
func (s *sidebar) editorRow(gtx C, th *ui.Theme, r sessionRow, b blockView, act *sidebarAction, tree, dragEnded bool) D {
	n := r.editor
	tip, ok := s.tips[r.key]
	if !ok {
		tip = new(ui.Tooltip)
		s.tips[r.key] = tip
	}
	tooltip := th.TooltipCard(s.root, tip, editorTooltip(th, n))
	tooltip.Side = ui.SideRight
	entries := func() []ui.MenuEntry { return editorMenu(r.path, act) }
	menu := s.menuState(s.menus, r.key)
	more := s.menuState(s.moreMenus, r.key)
	hover := clickable(s.checkoutRows, r.key)
	if hover.Clicked(gtx) && !dragEnded && !clickable(s.moreClicks, r.key).Hovered() {
		act.editor = r.path
	}
	hovered := hover.Hovered() || more.Opened()
	p := r.place(tree)
	p.ground = nodeGround(th, b)
	line := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		row := append(treeLead(p),
			layout.Rigid(func(gtx C) D { return checkoutNode(gtx, onGround(th, p.ground), icEditorFile, p.entry) }),
			ui.Gap(1.5),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						l := th.Text(ui.TextSM, n.name()).Mono()
						l.MaxLines = 1
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !n.dirty {
							return D{}
						}
						return layout.Inset{Left: ui.Space(1.5)}.Layout(gtx, func(gtx C) D { return ui.Dot(gtx, th.ToneWait, editorDirtyDot) })
					}),
				)
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				size := gtx.Dp(cardSlotSize)
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				if !hovered {
					return D{Size: gtx.Constraints.Min}
				}
				return layout.Center.Layout(gtx, func(gtx C) D {
					return th.DropdownMenu(s.root, more, entries()).Layout(gtx, func(gtx C) D {
						return th.CornerButton(clickable(s.moreClicks, r.key), icGroupMore, more.Opened()).Layout(gtx)
					})
				})
			}),
		)
		return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
	return th.ContextMenuLazy(s.root, menu, entries).Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D {
			return hover.Layout(gtx, func(gtx C) D {
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return treeRowFill(gtx, cardLayer(th, "", n.shown, hovered), p) },
					line,
				)
			})
		})
	})
}
