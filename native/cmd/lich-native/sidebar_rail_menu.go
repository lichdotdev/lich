package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// menu is a rail node's right click: the menu its row opens in the open
// sidebar, built by the same code, so the two shapes never drift apart. The
// rail draws no name, so nothing renames from it, and a node's fold, which
// the open sidebar puts on a click, is an entry here, where a click selects.
func (r *sidebarRail) menu(gtx C, th *ui.Theme, s *sidebar, it railItem, act *sidebarAction) ui.ContextMenuStyle {
	state, ok := r.menus[it.id]
	if !ok {
		state = new(ui.MenuState)
		r.menus[it.id] = state
	}
	return th.ContextMenuLazy(s.root, state, func() []ui.MenuEntry { return r.entries(th, s, it, act) })
}

func (r *sidebarRail) entries(th *ui.Theme, s *sidebar, it railItem, act *sidebarAction) []ui.MenuEntry {
	i := slices.IndexFunc(r.view.blocks, func(b blockView) bool { return b.key == it.block })
	if i < 0 {
		return nil
	}
	b := r.view.blocks[i]
	f := blockFrame{view: r.view, act: act}
	hooks := blockMenuHooks{newFolder: s.askFolderName}
	switch {
	case it.kind == railCard:
		sv := it.sv
		folders := slices.DeleteFunc(slices.Clone(r.view.folders), func(name string) bool { return name == sv.Folder })
		var extra []ui.MenuEntry
		if len(sv.continueIn) > 0 {
			extra = append(extra, continueInItem(th, sv, act))
		}
		if it.kids > 0 {
			extra = append(extra, foldEntry(motherKey(sv.ID), sv.Label, !it.folded, act))
		}
		return cardMenu(sv, folders, act, nil, func() { s.askFolderName([]string{sv.ID}) }, extra...)
	case it.kind == railCheckout && it.key != b.key:
		launch := launchEntries(s.launchProviders, i18n.T("sidebar.sessionLaunchMenuItems.newTerminal"), func(kind string) {
			act.launch = launchRequest{kind: kind, dir: it.path, folder: b.folder}
		})
		return []ui.MenuEntry{{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.checkoutRow.newSessionHere"), Icon: icPlus, Sub: launch}}
	}
	return append(headerEntries(b, f, s.launchProviders, hooks), foldEntry(b.key, b.title, !b.collapsed, act))
}

// foldEntry folds key away while open, or opens it.
func foldEntry(key, name string, open bool, act *sidebarAction) ui.MenuEntry {
	label, icon := i18n.T("sidebar.sessionGroupHeader.expand", i18n.Args{"name": name}), foldIcon(false)
	if open {
		label, icon = i18n.T("sidebar.sessionGroupHeader.collapse", i18n.Args{"name": name}), foldIcon(true)
	}
	return ui.MenuEntry{Label: label, Icon: icon, OnClick: func() { act.fold = key }}
}
